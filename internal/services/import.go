// Package services Excel 批量导入（M3-2）：存量客户/联系人批量录入。
// 复用 customers / contacts 表，解析 .xlsx 后按行创建，并返回导入摘要。
package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"bss/internal/models"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// 导入模板/解析使用的列头（第一行表头，顺序无关，按列名匹配）
var importCustomerHeaders = []string{
	"客户名称", "行业", "来源", "等级", "负责人邮箱", "备注",
	"联系人姓名", "联系人手机", "联系人邮箱", "联系人职位", "是否首要联系人",
}

// ImportError 单行导入错误
type ImportError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

// ImportResult 导入摘要
type ImportResult struct {
	Total            int          `json:"total"`             // 有效数据行数
	CreatedCustomers int          `json:"created_customers"` // 新建客户数
	CreatedContacts  int          `json:"created_contacts"`  // 新建联系人数
	Skipped          int          `json:"skipped"`           // 因重名跳过
	Errors           []ImportError `json:"errors"`           // 解析/落库失败行
}

// ImportService Excel 导入
type ImportService struct {
	db *gorm.DB
}

func NewImportService(db *gorm.DB) *ImportService { return &ImportService{db: db} }

// ImportCustomers 解析 xlsx 并批量创建客户（及可选联系人）。
// operatorID 为导入人；operatorRole 用于决定「负责人邮箱」列是否可跨人分配。
func (s *ImportService) ImportCustomers(ctx context.Context, r io.Reader, operatorID uint, operatorRole string) (*ImportResult, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, errors.New("无法解析 Excel 文件，请确认文件为 .xlsx 格式")
	}
	defer f.Close()

	// 选定工作表：优先「客户导入」，否则取第一个
	sheet := ""
	for _, n := range f.GetSheetList() {
		if n == "客户导入" {
			sheet = n
			break
		}
	}
	if sheet == "" {
		if names := f.GetSheetList(); len(names) > 0 {
			sheet = names[0]
		} else {
			return nil, errors.New("工作簿为空")
		}
	}

	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, errors.New("读取工作表失败")
	}
	if len(rows) < 2 {
		return &ImportResult{}, nil // 仅表头，无数据
	}

	colIdx := mapHeader(rows[0])
	res := &ImportResult{}

	for i := 1; i < len(rows); i++ {
		row := rows[i]
		if isBlankRow(row) {
			continue
		}
		res.Total++

		in := CustomerInput{
			Name:     strings.TrimSpace(cellAt(row, colIdx, "客户名称")),
			Industry: strings.TrimSpace(cellAt(row, colIdx, "行业")),
			Source:   strings.TrimSpace(cellAt(row, colIdx, "来源")),
			Level:    strings.TrimSpace(cellAt(row, colIdx, "等级")),
			Remark:   strings.TrimSpace(cellAt(row, colIdx, "备注")),
		}
		if in.Name == "" {
			res.Errors = append(res.Errors, ImportError{Row: i + 1, Message: "客户名称不能为空"})
			continue
		}

		// 负责人解析
		ownerID := operatorID
		email := strings.TrimSpace(cellAt(row, colIdx, "负责人邮箱"))
		if email != "" {
			var emp models.Employee
			if err := s.db.WithContext(ctx).
				Where("email = ? AND status = 'active' AND deleted_at IS NULL", email).
				First(&emp).Error; err != nil {
				res.Errors = append(res.Errors, ImportError{Row: i + 1, Message: fmt.Sprintf("负责人邮箱 %s 未匹配到在职员工", email)})
				continue
			}
			// 仅管理员/主管可跨人分配；销售导入一律归自己名下
			if operatorRole == models.RoleAdmin || operatorRole == models.RoleSalesLead {
				ownerID = emp.ID
			}
		}

		// 重名跳过（按名称唯一约束）
		var cnt int64
		if err := s.db.WithContext(ctx).Model(&models.Customer{}).Where("name = ?", in.Name).Count(&cnt).Error; err != nil {
			res.Errors = append(res.Errors, ImportError{Row: i + 1, Message: "校验重名失败：" + err.Error()})
			continue
		}
		if cnt > 0 {
			res.Skipped++
			continue
		}

		// 单客户 + 联系人 事务落库，保证一致性
		tx := s.db.WithContext(ctx).Begin()
		svc := NewCustomerService(tx)
		cust, err := svc.Create(ctx, in, ownerID)
		if err != nil {
			tx.Rollback()
			res.Errors = append(res.Errors, ImportError{Row: i + 1, Message: err.Error()})
			continue
		}
		if cname := strings.TrimSpace(cellAt(row, colIdx, "联系人姓名")); cname != "" {
			if _, cerr := svc.CreateContact(ctx, cust.ID, ContactInput{
				Name:      cname,
				Phone:     strings.TrimSpace(cellAt(row, colIdx, "联系人手机")),
				Email:     strings.TrimSpace(cellAt(row, colIdx, "联系人邮箱")),
				Position:  strings.TrimSpace(cellAt(row, colIdx, "联系人职位")),
				IsPrimary: parsePrimary(cellAt(row, colIdx, "是否首要联系人")),
			}); cerr != nil {
				tx.Rollback()
				res.Errors = append(res.Errors, ImportError{Row: i + 1, Message: "联系人创建失败：" + cerr.Error()})
				continue
			}
			res.CreatedContacts++
		}
		if err := tx.Commit().Error; err != nil {
			res.Errors = append(res.Errors, ImportError{Row: i + 1, Message: "提交失败：" + err.Error()})
			continue
		}
		res.CreatedCustomers++
	}
	return res, nil
}

// GenerateTemplate 生成导入模板（xlsx 字节流）：含表头与填写说明。
func (s *ImportService) GenerateTemplate() ([]byte, error) {
	f := excelize.NewFile()
	const sheet = "客户导入"
	f.SetSheetName("Sheet1", sheet)

	hdrStyle, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}
	for i, h := range importCustomerHeaders {
		cellName, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cellName, h); err != nil {
			return nil, err
		}
		if err := f.SetCellStyle(sheet, cellName, cellName, hdrStyle); err != nil {
			return nil, err
		}
	}

	// 填写说明 sheet
	f.NewSheet("填写说明")
	notes := []string{
		"请按「客户导入」工作表的列填写，第一行是表头，不要修改列名。",
		"客户名称：必填；系统按名称去重，已存在的客户将跳过不覆盖。",
		"行业 / 来源 / 等级：可选，建议与系统数据字典（系统配置）中的值一致。",
		"负责人邮箱：可选；留空则导入人为负责人，填写则分配对应在职员工（仅管理员/主管可跨人分配）。",
		"联系人* 列：可选；仅当填写「联系人姓名」时才创建该客户的联系人。",
		"是否首要联系人：填 是/否（或 Y/N/1/0）。",
		"空行会被忽略；请勿保留示例数据。",
	}
	for i, n := range notes {
		c, _ := excelize.CoordinatesToCellName(1, i+1)
		if err := f.SetCellValue("填写说明", c, n); err != nil {
			return nil, err
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ---------- 辅助 ----------

// mapHeader 将表头行映射为 列名→索引
func mapHeader(header []string) map[string]int {
	m := make(map[string]int)
	for i, h := range header {
		key := strings.TrimSpace(h)
		if key == "" {
			continue
		}
		// 兼容别名
		switch key {
		case "客户名称", "客户名", "名称":
			m["客户名称"] = i
		case "行业":
			m["行业"] = i
		case "来源":
			m["来源"] = i
		case "等级":
			m["等级"] = i
		case "负责人邮箱", "邮箱", "负责人":
			m["负责人邮箱"] = i
		case "备注", "备注说明":
			m["备注"] = i
		case "联系人姓名", "联系人", "对接人":
			m["联系人姓名"] = i
		case "联系人手机", "手机":
			m["联系人手机"] = i
		case "联系人邮箱", "联系人邮件":
			m["联系人邮箱"] = i
		case "联系人职位", "职位", "联系人职务":
			m["联系人职位"] = i
		case "是否首要联系人", "首要联系人":
			m["是否首要联系人"] = i
		default:
			m[key] = i
		}
	}
	return m
}

func cellAt(row []string, colIdx map[string]int, name string) string {
	i, ok := colIdx[name]
	if !ok || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func isBlankRow(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

func parsePrimary(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "是", "y", "yes", "true", "1", "√", "✔":
		return true
	default:
		return false
	}
}
