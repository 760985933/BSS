package services

import (
	"bytes"
	"context"
	"testing"

	"bss/internal/db"
	"bss/internal/models"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	// 用临时目录初始化真实 DB（含全部 migration），隔离测试
	dir := t.TempDir()
	gdb, err := db.Open(dir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	return gdb
}

// 生成一份导入模板格式的 xlsx（内存）
func buildImportXLSX(t *testing.T, rows [][]string) *bytes.Buffer {
	t.Helper()
	f := excelize.NewFile()
	const sheet = "客户导入"
	f.SetSheetName("Sheet1", sheet)
	headers := []string{"客户名称", "行业", "来源", "等级", "负责人邮箱", "备注",
		"联系人姓名", "联系人手机", "联系人邮箱", "联系人职位", "是否首要联系人"}
	for i, h := range headers {
		c, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, c, h); err != nil {
			t.Fatal(err)
		}
	}
	for r, row := range rows {
		for c, v := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			if err := f.SetCellValue(sheet, cell, v); err != nil {
				t.Fatal(err)
			}
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return &buf
}

func TestImportCustomers_Basic(t *testing.T) {
	gdb := newTestDB(t)
	svc := NewImportService(gdb)

	rows := [][]string{
		{"测试客户A", "科技", "转介绍", "A", "", "备注A", "张三", "13800000000", "z@x.com", "CTO", "是"},
		{"测试客户B", "金融", "广告", "B", "", "", "", "", "", "", ""},
	}
	buf := buildImportXLSX(t, rows)

	res, err := svc.ImportCustomers(context.Background(), buf, 1, models.RoleAdmin)
	if err != nil {
		t.Fatalf("ImportCustomers: %v", err)
	}
	if res.Total != 2 {
		t.Errorf("Total = %d, want 2", res.Total)
	}
	if res.CreatedCustomers != 2 {
		t.Errorf("CreatedCustomers = %d, want 2", res.CreatedCustomers)
	}
	if res.CreatedContacts != 1 {
		t.Errorf("CreatedContacts = %d, want 1（仅客户A 含联系人）", res.CreatedContacts)
	}
	if len(res.Errors) != 0 {
		t.Errorf("Errors = %v, want empty", res.Errors)
	}

	// 重复导入：重名应跳过
	buf2 := buildImportXLSX(t, rows)
	res2, err := svc.ImportCustomers(context.Background(), buf2, 1, models.RoleAdmin)
	if err != nil {
		t.Fatalf("ImportCustomers#2: %v", err)
	}
	if res2.CreatedCustomers != 0 || res2.Skipped != 2 {
		t.Errorf("重复导入：Created=%d Skipped=%d, want 0/2", res2.CreatedCustomers, res2.Skipped)
	}
}

func TestImportCustomers_InvalidOwnerEmail(t *testing.T) {
	gdb := newTestDB(t)
	svc := NewImportService(gdb)
	rows := [][]string{
		{"测试客户C", "科技", "", "", "nobody@x.com", ""},
	}
	buf := buildImportXLSX(t, rows)
	res, err := svc.ImportCustomers(context.Background(), buf, 1, models.RoleAdmin)
	if err != nil {
		t.Fatalf("ImportCustomers: %v", err)
	}
	if len(res.Errors) != 1 || res.CreatedCustomers != 0 {
		t.Errorf("应记录 1 条无效负责人邮箱错误且不创建客户；got errors=%v created=%d", res.Errors, res.CreatedCustomers)
	}
}
