package services

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"strings"
	"time"

	"bss/internal/models"
	"bss/internal/pkg/code"

	"gorm.io/gorm"
)

// 薪资核算相关错误
var (
	ErrPayrollMissing      = errors.New("薪资记录不存在")
	ErrPayrollNotDraft     = errors.New("薪资记录已核算/已发放，金额不可修改")
	ErrPayrollCalcState    = errors.New("仅草稿状态可核算")
	ErrPayrollPayState     = errors.New("仅已核算状态可发放")
	ErrPayrollPaid         = errors.New("已发放薪资不可删除")
	ErrPayrollPeriodInvalid = errors.New("期间格式应为 YYYY-MM")
	ErrPayrollEmployeeMissing = errors.New("员工不存在")
)

// CurrentPeriod 返回当前月份期间 YYYY-MM
func CurrentPeriod() string {
	return time.Now().Format("2006-01")
}

// PayrollInput 薪资可写字段（金额均为分）
type PayrollInput struct {
	EmployeeID     uint   `json:"employee_id,string"`
	Period         string `json:"period"`
	BaseCent       int64  `json:"base_cent"`
	BonusCent      int64  `json:"bonus_cent"`
	DeductionCent  int64  `json:"deduction_cent"`
	SocialCent     int64  `json:"social_cent"`
	TaxCent        int64  `json:"tax_cent"`
	Remark         string `json:"remark,omitempty"`
}

// validatePeriod 校验 YYYY-MM 并归一化
func validatePeriod(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", ErrPayrollPeriodInvalid
	}
	if _, err := time.Parse("2006-01", p); err != nil {
		return "", ErrPayrollPeriodInvalid
	}
	return p, nil
}

// ---------------- 生成：按月批量建草稿 ----------------

// GeneratePayrolls 为指定期间所有在职员工生成草稿薪资：
//   - 已存在 (employee_id, period) 的跳过（幂等）
//   - 底薪优先取该员工生效中劳动合同的月薪，否则 0
//
// 返回本次新建条数。
func GeneratePayrolls(ctx context.Context, db *gorm.DB, gen *code.Generator, period string, ownerID uint) (int, error) {
	period, err := validatePeriod(period)
	if err != nil {
		return 0, err
	}
	var emps []models.Employee
	if err := db.WithContext(ctx).Where("status = ? AND deleted_at IS NULL", "active").Find(&emps).Error; err != nil {
		return 0, err
	}
	created := 0
	for _, e := range emps {
		var dup models.Payroll
		dErr := db.WithContext(ctx).Where("employee_id = ? AND period = ? AND deleted_at IS NULL", e.ID, period).First(&dup).Error
		if dErr == nil {
			continue // 已存在
		}
		if !errors.Is(dErr, gorm.ErrRecordNotFound) {
			return created, dErr
		}
		base := int64(0)
		if c, cErr := GetActiveContractForEmployee(ctx, db, e.ID); cErr == nil && c != nil {
			base = c.SalaryCent
		}
		c, cErr := gen.Next(ctx, code.PrefixPayroll)
		if cErr != nil {
			return created, cErr
		}
		p := models.Payroll{
			Code:       c,
			EmployeeID: e.ID,
			Period:     period,
			BaseCent:   base,
			Status:     models.PayrollDraft,
			OwnerID:    ownerID,
		}
		if err := db.WithContext(ctx).Create(&p).Error; err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

// ---------------- 手动单建 ----------------

// CreatePayroll 手动新建一条薪资（默认草稿）
func CreatePayroll(ctx context.Context, db *gorm.DB, gen *code.Generator, in PayrollInput, ownerID uint) (*models.Payroll, error) {
	period, err := validatePeriod(in.Period)
	if err != nil {
		return nil, err
	}
	if in.EmployeeID == 0 {
		return nil, ErrPayrollEmployeeMissing
	}
	if _, err := getEmployee(db, in.EmployeeID); err != nil {
		return nil, err
	}
	c, err := gen.Next(ctx, code.PrefixPayroll)
	if err != nil {
		return nil, err
	}
	p := models.Payroll{
		Code:           c,
		EmployeeID:     in.EmployeeID,
		Period:         period,
		BaseCent:       in.BaseCent,
		BonusCent:      in.BonusCent,
		DeductionCent:  in.DeductionCent,
		SocialCent:     in.SocialCent,
		TaxCent:        in.TaxCent,
		Status:         models.PayrollDraft,
		OwnerID:        ownerID,
		Remark:         in.Remark,
	}
	if err := db.WithContext(ctx).Create(&p).Error; err != nil {
		return nil, err
	}
	return GetPayroll(ctx, db, p.ID)
}

// GetPayroll 详情
func GetPayroll(ctx context.Context, db *gorm.DB, id uint) (*models.Payroll, error) {
	var p models.Payroll
	if err := db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).
		Preload("Employee").Preload("Owner").First(&p).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPayrollMissing
		}
		return nil, err
	}
	return &p, nil
}

// ListPayrolls 列表（期间/员工/状态过滤）
func ListPayrolls(ctx context.Context, db *gorm.DB, period, employeeID, status string) ([]models.Payroll, error) {
	q := db.WithContext(ctx).Where("deleted_at IS NULL").Preload("Employee").Preload("Owner")
	if period != "" {
		q = q.Where("period = ?", period)
	}
	if employeeID != "" {
		q = q.Where("employee_id = ?", employeeID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var list []models.Payroll
	if err := q.Order("period DESC, employee_id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// UpdatePayroll 编辑（仅草稿；金额可手动调整，实发留待核算）
func UpdatePayroll(ctx context.Context, db *gorm.DB, id uint, in PayrollInput) (*models.Payroll, error) {
	var p models.Payroll
	if err := db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&p).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPayrollMissing
		}
		return nil, err
	}
	if p.Status != models.PayrollDraft {
		return nil, ErrPayrollNotDraft
	}
	if in.EmployeeID != 0 && in.EmployeeID != p.EmployeeID {
		if _, err := getEmployee(db, in.EmployeeID); err != nil {
			return nil, err
		}
	}
	updates := map[string]any{}
	if in.EmployeeID != 0 {
		updates["employee_id"] = in.EmployeeID
	}
	if in.Period != "" {
		period, err := validatePeriod(in.Period)
		if err != nil {
			return nil, err
		}
		updates["period"] = period
	}
	updates["base_cent"] = in.BaseCent
	updates["bonus_cent"] = in.BonusCent
	updates["deduction_cent"] = in.DeductionCent
	updates["social_cent"] = in.SocialCent
	updates["tax_cent"] = in.TaxCent
	updates["remark"] = in.Remark
	if err := db.WithContext(ctx).Model(&p).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetPayroll(ctx, db, id)
}

// CalcPayroll 核算：实发 = 底薪 + 奖金 - 扣款 - 社保 - 个税，状态转 calced
func CalcPayroll(ctx context.Context, db *gorm.DB, id uint) (*models.Payroll, error) {
	var p models.Payroll
	if err := db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&p).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPayrollMissing
		}
		return nil, err
	}
	if p.Status != models.PayrollDraft {
		return nil, ErrPayrollCalcState
	}
	net := p.BaseCent + p.BonusCent - p.DeductionCent - p.SocialCent - p.TaxCent
	updates := map[string]any{
		"net_cent": net,
		"status":   models.PayrollCalced,
	}
	if err := db.WithContext(ctx).Model(&p).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetPayroll(ctx, db, id)
}

// MarkPayrollPaid 发放：calced → paid，记录发放时间
func MarkPayrollPaid(ctx context.Context, db *gorm.DB, id uint) (*models.Payroll, error) {
	var p models.Payroll
	if err := db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&p).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPayrollMissing
		}
		return nil, err
	}
	if p.Status != models.PayrollCalced {
		return nil, ErrPayrollPayState
	}
	now := time.Now().UTC()
	updates := map[string]any{
		"status":  models.PayrollPaid,
		"paid_at": &now,
	}
	if err := db.WithContext(ctx).Model(&p).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetPayroll(ctx, db, id)
}

// DeletePayroll 软删（已发放不可删）
func DeletePayroll(ctx context.Context, db *gorm.DB, id uint) error {
	var p models.Payroll
	if err := db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&p).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPayrollMissing
		}
		return err
	}
	if p.Status == models.PayrollPaid {
		return ErrPayrollPaid
	}
	res := db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).Delete(&models.Payroll{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrPayrollMissing
	}
	return nil
}

// ExportPayrollsCSV 导出指定期间工资条 CSV（敏感字段仅对授权角色可见，路由已限 admin/finance/hr）
func ExportPayrollsCSV(ctx context.Context, db *gorm.DB, period string) (string, error) {
	if _, err := validatePeriod(period); err != nil {
		return "", err
	}
	list, err := ListPayrolls(ctx, db, period, "", "")
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	w := csv.NewWriter(&sb)
	header := []string{"员工姓名", "工号/邮箱", "期间", "基本工资(元)", "奖金(元)", "扣款(元)", "社保(元)", "个税(元)", "实发(元)"}
	if err := w.Write(header); err != nil {
		return "", err
	}
	for _, p := range list {
		name := ""
		email := ""
		if p.Employee != nil {
			name = p.Employee.Name
			email = p.Employee.Email
		}
		row := []string{
			name,
			email,
			p.Period,
			fmt.Sprintf("%.2f", float64(p.BaseCent)/100),
			fmt.Sprintf("%.2f", float64(p.BonusCent)/100),
			fmt.Sprintf("%.2f", float64(p.DeductionCent)/100),
			fmt.Sprintf("%.2f", float64(p.SocialCent)/100),
			fmt.Sprintf("%.2f", float64(p.TaxCent)/100),
			fmt.Sprintf("%.2f", float64(p.NetCent)/100),
		}
		if err := w.Write(row); err != nil {
			return "", err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	return sb.String(), nil
}
