package services

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bss/internal/middleware"
	"bss/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupReportDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "r.db")), &gorm.Config{
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Employee{}, &models.Customer{}, &models.Deal{},
		&models.Contract{}, &models.DealContract{}, &models.PaymentRecord{}, &models.PaymentPlan{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func reportAdminCtx() context.Context {
	return middleware.WithClaims(context.Background(), &middleware.Claims{UserID: 1, Role: models.RoleAdmin, Dept: "HQ"})
}

func salesCtx(uid uint) context.Context {
	return middleware.WithClaims(context.Background(), &middleware.Claims{UserID: uid, Role: models.RoleSales, Dept: "S"})
}

// 构造：两个销售各签一份当月合同 + 一份 13 个月前的旧合同；赢单与回款若干
func seedReportData(t *testing.T, db *gorm.DB) (empA, empB uint) {
	t.Helper()
	a := models.Employee{Name: "甲", Email: "a@x.com", Dept: "S", Role: "sales", Phone: "1"}
	b := models.Employee{Name: "乙", Email: "b@x.com", Dept: "S", Role: "sales", Phone: "2"}
	if err := db.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&b).Error; err != nil {
		t.Fatal(err)
	}
	empA, empB = a.ID, b.ID

	ca := models.Customer{Code: "KH-RA", Name: "客户A", OwnerID: empA}
	cb := models.Customer{Code: "KH-RB", Name: "客户B", OwnerID: empB}
	db.Create(&ca)
	db.Create(&cb)

	thisMonth := time.Now().Format("2006-01-02")
	oldMonth := time.Now().AddDate(0, -13, 0).Format("2006-01-02")

	// 当月签约：甲 10000，乙 20000
	db.Create(&models.Contract{Code: "HT-1", CustomerID: ca.ID, Title: "合同A", AmountCent: 10000, SignDate: thisMonth, Status: models.ContractSigned, OwnerID: empA})
	db.Create(&models.Contract{Code: "HT-2", CustomerID: cb.ID, Title: "合同B", AmountCent: 20000, SignDate: thisMonth, Status: models.ContractSigned, OwnerID: empB})
	// 旧合同（不应出现在近 12 个月趋势里）
	db.Create(&models.Contract{Code: "HT-3", CustomerID: ca.ID, Title: "旧合同", AmountCent: 99999, SignDate: oldMonth, Status: models.ContractSigned, OwnerID: empA})

	// 赢单：甲 1 单 5000，乙 2 单各 8000
	db.Create(&models.Deal{Code: "SD-1", CustomerID: ca.ID, Title: "单A", AmountCent: 5000, Status: models.DealWon, OwnerID: empA})
	db.Create(&models.Deal{Code: "SD-2", CustomerID: cb.ID, Title: "单B1", AmountCent: 8000, Status: models.DealWon, OwnerID: empB})
	db.Create(&models.Deal{Code: "SD-3", CustomerID: cb.ID, Title: "单B2", AmountCent: 8000, Status: models.DealWon, OwnerID: empB})
	// 漏斗分布
	db.Create(&models.Deal{Code: "SD-4", CustomerID: ca.ID, Title: "线索", AmountCent: 1000, Status: models.DealProspecting, OwnerID: empA})
	db.Create(&models.Deal{Code: "SD-5", CustomerID: ca.ID, Title: "确认", AmountCent: 1000, Status: models.DealQualifying, OwnerID: empA})
	db.Create(&models.Deal{Code: "SD-6", CustomerID: ca.ID, Title: "方案", AmountCent: 1000, Status: models.DealProposal, OwnerID: empA})
	db.Create(&models.Deal{Code: "SD-7", CustomerID: ca.ID, Title: "谈判", AmountCent: 1000, Status: models.DealNegotiating, OwnerID: empA})

	// 回款：甲 3000，乙 5000（paid_at 当月）
	var ctA, ctB models.Contract
	db.Where("code = ?", "HT-1").First(&ctA)
	db.Where("code = ?", "HT-2").First(&ctB)
	db.Create(&models.PaymentRecord{ContractID: ctA.ID, AmountCent: 3000, PaidAt: thisMonth, Method: "bank", CreatedBy: empA})
	db.Create(&models.PaymentRecord{ContractID: ctB.ID, AmountCent: 5000, PaidAt: thisMonth, Method: "bank", CreatedBy: empB})
	return empA, empB
}

func TestReportSignTrend(t *testing.T) {
	db := setupReportDB(t)
	empA, empB := seedReportData(t, db)
	svc := NewReportService(db)
	ctx := reportAdminCtx()

	res, err := svc.GetSignTrend(ctx, 12)
	if err != nil {
		t.Fatalf("sign trend: %v", err)
	}
	thisMonth := time.Now().Format("2006-01")
	var cur int64
	for _, r := range res.Rows {
		if r.Month == thisMonth {
			cur = r.AmountCent
		}
	}
	// 当月应为 10000+20000=30000；旧合同 99999 不应计入近 12 个月
	if cur != 30000 {
		t.Fatalf("当月签约额应为 30000, got %d", cur)
	}

	// 数据范围：甲(sales) 仅见自己 10000
	resA, err := svc.GetSignTrend(salesCtx(empA), 12)
	if err != nil {
		t.Fatalf("sign trend A: %v", err)
	}
	var curA int64
	for _, r := range resA.Rows {
		if r.Month == thisMonth {
			curA = r.AmountCent
		}
	}
	if curA != 10000 {
		t.Fatalf("销售甲当月签约额应为 10000, got %d", curA)
	}
	_ = empB
}

func TestReportPaymentTrend(t *testing.T) {
	db := setupReportDB(t)
	seedReportData(t, db)
	svc := NewReportService(db)
	res, err := svc.GetPaymentTrend(reportAdminCtx(), 12)
	if err != nil {
		t.Fatalf("payment trend: %v", err)
	}
	thisMonth := time.Now().Format("2006-01")
	var cur int64
	for _, r := range res.Rows {
		if r.Month == thisMonth {
			cur = r.AmountCent
		}
	}
	if cur != 8000 { // 3000 + 5000
		t.Fatalf("当月回款额应为 8000, got %d", cur)
	}
}

func TestReportSalesRank(t *testing.T) {
	db := setupReportDB(t)
	seedReportData(t, db)
	svc := NewReportService(db)
	res, err := svc.GetSalesRank(reportAdminCtx())
	if err != nil {
		t.Fatalf("sales rank: %v", err)
	}
	m := map[uint]SalesRankRow{}
	for _, r := range res.Rows {
		m[r.OwnerID] = r
	}
	if len(res.Rows) != 2 {
		t.Fatalf("应有 2 名销售, got %d", len(res.Rows))
	}
	// 乙签约 20000、赢单 2、回款 5000
	if r, ok := m[findOwnerID(t, db, "乙")]; ok {
		if r.SignedCent != 20000 {
			t.Fatalf("乙签约额应为 20000, got %d", r.SignedCent)
		}
		if r.WonDeals != 2 {
			t.Fatalf("乙赢单应为 2, got %d", r.WonDeals)
		}
		if r.PaidCent != 5000 {
			t.Fatalf("乙回款额应为 5000, got %d", r.PaidCent)
		}
	} else {
		t.Fatalf("未找到乙")
	}
}

func TestReportFunnel(t *testing.T) {
	db := setupReportDB(t)
	seedReportData(t, db)
	svc := NewReportService(db)
	res, err := svc.GetFunnel(reportAdminCtx())
	if err != nil {
		t.Fatalf("funnel: %v", err)
	}
	if len(res.Rows) != 5 {
		t.Fatalf("漏斗应有 5 阶段, got %d", len(res.Rows))
	}
	m := map[string]FunnelRow{}
	for _, r := range res.Rows {
		m[r.Stage] = r
	}
	if m[models.DealProspecting].Count != 1 {
		t.Fatalf("线索数应为 1, got %d", m[models.DealProspecting].Count)
	}
	if m[models.DealWon].Count != 3 { // 甲1+乙2
		t.Fatalf("赢单数应为 3, got %d", m[models.DealWon].Count)
	}
}

func TestReportExportCSV(t *testing.T) {
	db := setupReportDB(t)
	seedReportData(t, db)
	svc := NewReportService(db)
	csv, filename, err := svc.ExportCSV(reportAdminCtx(), string(ReportSalesRank))
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if filename != "bss_sales_rank.csv" {
		t.Fatalf("文件名应为 bss_sales_rank.csv, got %s", filename)
	}
	if !strings.HasPrefix(csv, "\uFEFF") {
		t.Fatalf("CSV 应带 UTF-8 BOM")
	}
	if !strings.Contains(csv, "乙") || !strings.Contains(csv, "200.00") {
		t.Fatalf("CSV 应包含乙及其签约额(元)")
	}
	if _, _, err := svc.ExportCSV(reportAdminCtx(), "bad_type"); err == nil {
		t.Fatalf("未知类型应报错")
	}
}

func findOwnerID(t *testing.T, db *gorm.DB, name string) uint {
	t.Helper()
	var e models.Employee
	if err := db.Where("name = ?", name).First(&e).Error; err != nil {
		t.Fatal(err)
	}
	return e.ID
}
