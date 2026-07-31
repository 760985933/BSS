package services

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"bss/internal/middleware"
	"bss/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupNotifDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "n.db")), &gorm.Config{
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Employee{}, &models.Customer{}, &models.Deal{},
		&models.Contract{}, &models.DealContract{}, &models.Attachment{},
		&models.PaymentPlan{}, &models.PaymentRecord{}, &models.Notification{},
		&models.LaborContract{}, &models.Onboarding{},
		&models.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS code_counters (
		prefix TEXT NOT NULL, year INTEGER NOT NULL, seq INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (prefix, year))`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func adminCtx() context.Context {
	return middleware.WithClaims(context.Background(), &middleware.Claims{
		UserID: 1, Role: models.RoleAdmin, Name: "t", Dept: "s",
	})
}

func mustEmployee(t *testing.T, db *gorm.DB, tag string) uint {
	t.Helper()
	e := models.Employee{Name: "e" + tag, Email: "e" + tag + "@x.com", Dept: "s", Role: "sales", Phone: "1"}
	if err := db.Create(&e).Error; err != nil {
		t.Fatal(err)
	}
	return e.ID
}

func mustCustomer(t *testing.T, db *gorm.DB, tag string, owner uint) uint {
	t.Helper()
	c := models.Customer{Code: "KH-" + tag, Name: "c" + tag, OwnerID: owner}
	if err := db.Create(&c).Error; err != nil {
		t.Fatal(err)
	}
	return c.ID
}

func TestScanContractExpiring(t *testing.T) {
	db := setupNotifDB(t)
	now := time.Now()
	owner := mustEmployee(t, db, "a")
	mustCustomer(t, db, "a", owner)

	near := models.Contract{Code: "HT-N", CustomerID: 1, Title: "近", AmountCent: 1000,
		ExpireDate: now.AddDate(0, 0, 10).Format("2006-01-02"), Status: models.ContractSigned, OwnerID: owner}
	if err := db.Create(&near).Error; err != nil {
		t.Fatal(err)
	}
	far := models.Contract{Code: "HT-F", CustomerID: 1, Title: "远", AmountCent: 1000,
		ExpireDate: now.AddDate(0, 0, 60).Format("2006-01-02"), Status: models.ContractSigned, OwnerID: owner}
	if err := db.Create(&far).Error; err != nil {
		t.Fatal(err)
	}
	draft := models.Contract{Code: "HT-D", CustomerID: 1, Title: "草", AmountCent: 1000,
		ExpireDate: now.AddDate(0, 0, 5).Format("2006-01-02"), Status: models.ContractDraft, OwnerID: owner}
	if err := db.Create(&draft).Error; err != nil {
		t.Fatal(err)
	}

	n, err := ScanReminders(context.Background(), db, now)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("期望生成 1 条到期提醒，实际 %d", n)
	}
	// 重复扫描应去重
	n2, err := ScanReminders(context.Background(), db, now)
	if err != nil {
		t.Fatal(err)
	}
	if n2 != 0 {
		t.Fatalf("重复扫描应 0 新增，实际 %d", n2)
	}
	var cnt int64
	db.Model(&models.Notification{}).Where("type = ?", NotifContractExpiring).Count(&cnt)
	if cnt != 1 {
		t.Fatalf("到期通知应为 1 条，实际 %d", cnt)
	}
}

func TestScanPaymentOverdue(t *testing.T) {
	db := setupNotifDB(t)
	now := time.Now()
	owner := mustEmployee(t, db, "b")
	cid := mustCustomer(t, db, "b", owner)
	ct := models.Contract{Code: "HT-O", CustomerID: cid, Title: "o", AmountCent: 50000,
		SignDate: now.Format("2006-01-02"), Status: models.ContractSigned, OwnerID: owner}
	if err := db.Create(&ct).Error; err != nil {
		t.Fatal(err)
	}
	overdue := models.PaymentPlan{ContractID: ct.ID, PeriodNo: 1,
		DueDate: now.AddDate(0, 0, -5).Format("2006-01-02"), AmountCent: 50000, Status: models.PlanPending}
	if err := db.Create(&overdue).Error; err != nil {
		t.Fatal(err)
	}
	paid := models.PaymentPlan{ContractID: ct.ID, PeriodNo: 2,
		DueDate: now.AddDate(0, 0, -3).Format("2006-01-02"), AmountCent: 20000, Status: models.PlanPaid}
	if err := db.Create(&paid).Error; err != nil {
		t.Fatal(err)
	}

	n, err := ScanReminders(context.Background(), db, now)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("期望生成 1 条逾期提醒（已付计划不计），实际 %d", n)
	}
	var cnt int64
	db.Model(&models.Notification{}).Where("type = ?", NotifPaymentOverdue).Count(&cnt)
	if cnt != 1 {
		t.Fatalf("逾期通知应为 1 条，实际 %d", cnt)
	}
}

func TestDashboardAggregation(t *testing.T) {
	db := setupNotifDB(t)
	now := time.Now()
	month := now.Format("2006-01")
	owner := mustEmployee(t, db, "c")
	cid := mustCustomer(t, db, "c", owner)

	// 赢单商单（本月内 updated_at）
	won := models.Deal{Code: "D-W", CustomerID: cid, Title: "w", AmountCent: 80000,
		Status: models.DealWon, OwnerID: owner, Probability: 100}
	if err := db.Create(&won).Error; err != nil {
		t.Fatal(err)
	}
	// 进行中商单
	open := models.Deal{Code: "D-O", CustomerID: cid, Title: "o", AmountCent: 40000,
		Status: models.DealProspecting, OwnerID: owner, Probability: 10}
	if err := db.Create(&open).Error; err != nil {
		t.Fatal(err)
	}

	// 本月签约合同
	ct := models.Contract{Code: "HT-S", CustomerID: cid, Title: "s", AmountCent: 100000,
		SignDate: now.Format("2006-01-02"), ExpireDate: now.AddDate(0, 0, 10).Format("2006-01-02"),
		Status: models.ContractSigned, OwnerID: owner}
	if err := db.Create(&ct).Error; err != nil {
		t.Fatal(err)
	}
	// 本月回款
	rec := models.PaymentRecord{ContractID: ct.ID, AmountCent: 30000, PaidAt: now.Format("2006-01-02"), CreatedBy: owner}
	if err := db.Create(&rec).Error; err != nil {
		t.Fatal(err)
	}
	// 逾期计划：部分核销，未收 30000
	plan := models.PaymentPlan{ContractID: ct.ID, PeriodNo: 1,
		DueDate: now.AddDate(0, 0, -2).Format("2006-01-02"), AmountCent: 50000, Status: models.PlanPartial}
	if err := db.Create(&plan).Error; err != nil {
		t.Fatal(err)
	}
	rec2 := models.PaymentRecord{ContractID: ct.ID, PlanID: &plan.ID, AmountCent: 20000, PaidAt: now.Format("2006-01-02"), CreatedBy: owner}
	if err := db.Create(&rec2).Error; err != nil {
		t.Fatal(err)
	}

	d, err := GetDashboard(adminCtx(), db, now)
	if err != nil {
		t.Fatal(err)
	}
	if d.Cards.SignedThisMonth != 100000 {
		t.Errorf("本月签约金额应为 100000，实际 %d", d.Cards.SignedThisMonth)
	}
	if d.Cards.PaidThisMonth != 50000 { // 30000 + 20000 均本月
		t.Errorf("本月回款金额应为 50000，实际 %d", d.Cards.PaidThisMonth)
	}
	if d.Cards.OpenDeals < 1 {
		t.Errorf("进行中商单应 >=1，实际 %d", d.Cards.OpenDeals)
	}
	if d.Cards.OverdueAmount != 30000 {
		t.Errorf("逾期金额应为 30000（50000-20000），实际 %d", d.Cards.OverdueAmount)
	}
	// 列表应含即将到期合同与逾期计划与近期赢单
	if len(d.ExpiringContracts) < 1 {
		t.Error("即将到期合同列表不应为空")
	}
	if len(d.OverduePlans) < 1 {
		t.Error("逾期回款列表不应为空")
	}
	if len(d.RecentWonDeals) < 1 {
		t.Error("近期赢单列表不应为空")
	}
	_ = month
}

func TestMarkReadOwnership(t *testing.T) {
	db := setupNotifDB(t)
	n := models.Notification{UserID: 5, Type: NotifContractExpiring, Title: "t", EntityType: "contract", EntityID: 1, DedupKey: "k1"}
	if err := db.Create(&n).Error; err != nil {
		t.Fatal(err)
	}
	// 他人标记应被拒
	if err := MarkRead(context.Background(), db, 9, n.ID); err != ErrNotificationForbidden {
		t.Fatalf("非归属人标记应返回 Forbidden，实际 %v", err)
	}
	// 归属人标记成功
	if err := MarkRead(context.Background(), db, 5, n.ID); err != nil {
		t.Fatalf("归属人标记应成功，实际 %v", err)
	}
	var after models.Notification
	db.First(&after, n.ID)
	if !after.IsRead {
		t.Error("标记后应已读")
	}
}
