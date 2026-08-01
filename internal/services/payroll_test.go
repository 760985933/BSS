package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"bss/internal/models"
	"bss/internal/pkg/code"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func sqliteOpen() gorm.Dialector {
	return sqlite.Open("file::memory:?cache=shared")
}

func setupPayrollDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqliteOpen(), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Employee{}, &models.LaborContract{}, &models.Payroll{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	// 单号生成器依赖的计数器表
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS code_counters (
		prefix TEXT NOT NULL, year INTEGER NOT NULL, seq INTEGER NOT NULL,
		PRIMARY KEY (prefix, year))`).Error; err != nil {
		t.Fatalf("create code_counters: %v", err)
	}
	return db
}

func payrollMustEmployee(t *testing.T, db *gorm.DB, name string) uint {
	t.Helper()
	e := models.Employee{Name: name, Email: name + "@x.com", Role: "hr", Status: "active"}
	if err := db.Create(&e).Error; err != nil {
		t.Fatalf("create employee: %v", err)
	}
	return e.ID
}

func payrollMustContract(t *testing.T, db *gorm.DB, empID uint, salary int64) uint {
	t.Helper()
	sd := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := models.LaborContract{
		EmployeeID: empID, Type: models.LCTypeFixed,
		StartDate: &sd, Status: models.LCStatusActive, SalaryCent: salary,
	}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("create contract: %v", err)
	}
	return c.ID
}

func TestGeneratePayrollsPullsContractSalary(t *testing.T) {
	db := setupPayrollDB(t)
	gen := code.NewGenerator(db)
	ctx := context.Background()
	empID := payrollMustEmployee(t, db, "薪员工")
	payrollMustContract(t, db, empID, 200000) // 2000 元/月

	n, err := GeneratePayrolls(ctx, db, gen, "2026-08", 1)
	if err != nil {
		t.Fatalf("GeneratePayrolls: %v", err)
	}
	if n != 1 {
		t.Fatalf("应生成 1 条，got %d", n)
	}
	// 幂等：重复生成不应新增
	n2, err := GeneratePayrolls(ctx, db, gen, "2026-08", 1)
	if err != nil {
		t.Fatalf("GeneratePayrolls 2: %v", err)
	}
	if n2 != 0 {
		t.Fatalf("重复生成应 0 条，got %d", n2)
	}
	list, err := ListPayrolls(ctx, db, "2026-08", "", "")
	if err != nil {
		t.Fatalf("ListPayrolls: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("应有 1 条，got %d", len(list))
	}
	if list[0].BaseCent != 200000 {
		t.Fatalf("底薪应取自合同 200000，got %d", list[0].BaseCent)
	}
	if list[0].Status != models.PayrollDraft {
		t.Fatalf("初始应为 draft，got %s", list[0].Status)
	}
	if !strings.HasPrefix(list[0].Code, "PA-") {
		t.Fatalf("单号应以 PA- 开头，got %s", list[0].Code)
	}
}

func TestPayrollCalcAndPayFlow(t *testing.T) {
	db := setupPayrollDB(t)
	gen := code.NewGenerator(db)
	ctx := context.Background()
	empID := payrollMustEmployee(t, db, "核员工")

	in := PayrollInput{EmployeeID: empID, Period: "2026-09", BaseCent: 100000, BonusCent: 5000, DeductionCent: 2000, SocialCent: 3000, TaxCent: 1000}
	p, err := CreatePayroll(ctx, db, gen, in, 1)
	if err != nil {
		t.Fatalf("CreatePayroll: %v", err)
	}
	// 实发 = 100000+5000-2000-3000-1000 = 99000，但草稿未核算，net=0
	if p.NetCent != 0 {
		t.Fatalf("草稿 net 应为 0，got %d", p.NetCent)
	}
	// 编辑（草稿可改）
	p, err = UpdatePayroll(ctx, db, p.ID, PayrollInput{EmployeeID: empID, Period: "2026-09", BaseCent: 120000, BonusCent: 5000})
	if err != nil {
		t.Fatalf("UpdatePayroll: %v", err)
	}
	if p.BaseCent != 120000 {
		t.Fatalf("编辑后底薪应 120000，got %d", p.BaseCent)
	}
	// 核算：net = 120000+5000-0-0-0 = 125000
	p, err = CalcPayroll(ctx, db, p.ID)
	if err != nil {
		t.Fatalf("CalcPayroll: %v", err)
	}
	if p.Status != models.PayrollCalced {
		t.Fatalf("核算后状态应 calced，got %s", p.Status)
	}
	if p.NetCent != 125000 {
		t.Fatalf("实发应 125000，got %d", p.NetCent)
	}
	// 核算后编辑应被拒
	_, err = UpdatePayroll(ctx, db, p.ID, PayrollInput{EmployeeID: empID, Period: "2026-09", BaseCent: 1})
	if err == nil || !errors.Is(err, ErrPayrollNotDraft) {
		t.Fatalf("核算后编辑应被拒(ErrPayrollNotDraft)，got %v", err)
	}
	// 发放
	p, err = MarkPayrollPaid(ctx, db, p.ID)
	if err != nil {
		t.Fatalf("MarkPayrollPaid: %v", err)
	}
	if p.Status != models.PayrollPaid {
		t.Fatalf("发放后状态应 paid，got %s", p.Status)
	}
	if p.PaidAt == nil {
		t.Fatalf("发放时间应已记录")
	}
	// 已发放不可删
	if err := DeletePayroll(ctx, db, p.ID); err == nil || !errors.Is(err, ErrPayrollPaid) {
		t.Fatalf("已发放删除应被拒(ErrPayrollPaid)，got %v", err)
	}
}

func TestPayrollInvalidTransitions(t *testing.T) {
	db := setupPayrollDB(t)
	gen := code.NewGenerator(db)
	ctx := context.Background()
	empID := payrollMustEmployee(t, db, "状员工")
	p, err := CreatePayroll(ctx, db, gen, PayrollInput{EmployeeID: empID, Period: "2026-10", BaseCent: 1000}, 1)
	if err != nil {
		t.Fatalf("CreatePayroll: %v", err)
	}
	// 未核算直接发放 → 拒
	if _, err := MarkPayrollPaid(ctx, db, p.ID); err == nil || !errors.Is(err, ErrPayrollPayState) {
		t.Fatalf("未核算发放应被拒(ErrPayrollPayState)，got %v", err)
	}
}

func TestExportPayrollsCSV(t *testing.T) {
	db := setupPayrollDB(t)
	gen := code.NewGenerator(db)
	ctx := context.Background()
	empID := payrollMustEmployee(t, db, "导员工")
	p, err := CreatePayroll(ctx, db, gen, PayrollInput{EmployeeID: empID, Period: "2026-11", BaseCent: 123450}, 1)
	if err != nil {
		t.Fatalf("CreatePayroll: %v", err)
	}
	if _, err := CalcPayroll(ctx, db, p.ID); err != nil {
		t.Fatalf("CalcPayroll: %v", err)
	}
	csvStr, err := ExportPayrollsCSV(ctx, db, "2026-11")
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if !strings.Contains(csvStr, "导员工") {
		t.Fatalf("CSV 应含员工名: %s", csvStr)
	}
	if !strings.Contains(csvStr, "1234.50") {
		t.Fatalf("CSV 应含实发 1234.50: %s", csvStr)
	}
}
