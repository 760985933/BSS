package services

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"bss/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupPaymentDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "pay.db")), &gorm.Config{
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Employee{}, &models.Customer{}, &models.Contract{},
		&models.PaymentPlan{}, &models.PaymentRecord{}, &models.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	return db
}

// seedPayment 直接建一个 signed 合同（金额 100000 分），tag 保证 email/code 唯一
func seedPayment(t *testing.T, db *gorm.DB, tag string) uint {
	t.Helper()
	owner := models.Employee{Name: "owner", Email: "po-" + tag + "@x.com", Dept: "s", Role: "sales", Phone: "1"}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatal(err)
	}
	cust := models.Customer{Code: "KH-" + tag, Name: "pc" + tag, OwnerID: owner.ID}
	if err := db.Create(&cust).Error; err != nil {
		t.Fatal(err)
	}
	ct := models.Contract{Code: "HT-" + tag, CustomerID: cust.ID, Title: "合同" + tag, AmountCent: 100000, Status: models.ContractSigned, OwnerID: owner.ID}
	if err := db.Create(&ct).Error; err != nil {
		t.Fatal(err)
	}
	return ct.ID
}

// 计划总额上限：超过合同额应被拒
func TestPaymentPlanTotalCap(t *testing.T) {
	db := setupPaymentDB(t)
	cid := seedPayment(t, db, "a")
	svc := NewPaymentService(db)
	ctx := context.Background()

	if _, err := svc.CreatePlan(ctx, PlanInput{ContractID: cid, PeriodNo: 1, DueDate: "2026-06-01", AmountCent: 60000}); err != nil {
		t.Fatalf("第一笔计划应成功: %v", err)
	}
	if _, err := svc.CreatePlan(ctx, PlanInput{ContractID: cid, PeriodNo: 2, DueDate: "2026-07-01", AmountCent: 50000}); err != ErrPlanAmountExceed {
		t.Fatalf("第二笔计划应超合同额(100000<110000)，得到 err=%v", err)
	}
}

// 已核销计划禁改禁删
func TestPaymentPlanLocked(t *testing.T) {
	db := setupPaymentDB(t)
	cid := seedPayment(t, db, "a")
	svc := NewPaymentService(db)
	ctx := context.Background()

	p, err := svc.CreatePlan(ctx, PlanInput{ContractID: cid, PeriodNo: 1, DueDate: "2026-06-01", AmountCent: 50000})
	if err != nil {
		t.Fatal(err)
	}
	// 全额核销 → 计划变为 paid
	pid := p.ID
	if err := svc.CreateRecords(ctx, cid, []RecordInput{{PlanID: strPtr(pid), AmountCent: 50000, PaidAt: "2026-06-02"}}, 1); err != nil {
		t.Fatal(err)
	}
	if err := svc.UpdatePlan(ctx, pid, cid, PlanInput{ContractID: cid, PeriodNo: 1, DueDate: "2026-06-01", AmountCent: 40000}); err != ErrPlanLocked {
		t.Fatalf("已核销计划编辑应被拒，得到 err=%v", err)
	}
	if err := svc.DeletePlan(ctx, pid, cid); err != ErrPlanLocked {
		t.Fatalf("已核销计划删除应被拒，得到 err=%v", err)
	}
}

// 记录核销自动推进计划状态 pending→partial→paid
func TestPaymentRecordAdvancesStatus(t *testing.T) {
	db := setupPaymentDB(t)
	cid := seedPayment(t, db, "a")
	svc := NewPaymentService(db)
	ctx := context.Background()

	p, _ := svc.CreatePlan(ctx, PlanInput{ContractID: cid, PeriodNo: 1, DueDate: "2026-06-01", AmountCent: 50000})
	pid := p.ID
	if err := svc.CreateRecords(ctx, cid, []RecordInput{{PlanID: strPtr(pid), AmountCent: 20000, PaidAt: "2026-06-02"}}, 1); err != nil {
		t.Fatal(err)
	}
	if st := planStatus(t, db, pid); st != models.PlanPartial {
		t.Fatalf("部分核销后应为 partial, got %s", st)
	}
	if err := svc.CreateRecords(ctx, cid, []RecordInput{{PlanID: strPtr(pid), AmountCent: 30000, PaidAt: "2026-06-03"}}, 1); err != nil {
		t.Fatal(err)
	}
	if st := planStatus(t, db, pid); st != models.PlanPaid {
		t.Fatalf("全额核销后应为 paid, got %s", st)
	}
}

// 跨合同核销应被拒
func TestPaymentCrossPlanMismatch(t *testing.T) {
	db := setupPaymentDB(t)
	cid := seedPayment(t, db, "a")
	other := seedPayment(t, db, "b") // 另一合同
	svc := NewPaymentService(db)
	ctx := context.Background()

	p, _ := svc.CreatePlan(ctx, PlanInput{ContractID: other, PeriodNo: 1, DueDate: "2026-06-01", AmountCent: 50000})
	if err := svc.CreateRecords(ctx, cid, []RecordInput{{PlanID: strPtr(p.ID), AmountCent: 10000, PaidAt: "2026-06-02"}}, 1); err != ErrRecordPlanMismatch {
		t.Fatalf("跨合同核销应被拒，得到 err=%v", err)
	}
}

// 汇总：应收/已收/余额/逾期额准确到分
func TestPaymentSummary(t *testing.T) {
	db := setupPaymentDB(t)
	cid := seedPayment(t, db, "a")
	svc := NewPaymentService(db)
	ctx := context.Background()

	// p1 逾期 60000，p2 未来 40000（合同 100000）
	p1, _ := svc.CreatePlan(ctx, PlanInput{ContractID: cid, PeriodNo: 1, DueDate: "2020-01-01", AmountCent: 60000})
	p2, _ := svc.CreatePlan(ctx, PlanInput{ContractID: cid, PeriodNo: 2, DueDate: "2099-01-01", AmountCent: 40000})
	// 对 p1 核销 40000（partial + 逾期）
	if err := svc.CreateRecords(ctx, cid, []RecordInput{{PlanID: strPtr(p1.ID), AmountCent: 40000, PaidAt: "2020-02-01"}}, 1); err != nil {
		t.Fatal(err)
	}

	sum, err := svc.Summary(ctx, cid)
	if err != nil {
		t.Fatal(err)
	}
	if sum.ReceivableCent != 100000 {
		t.Errorf("应收应为 100000, got %d", sum.ReceivableCent)
	}
	if sum.ReceivedCent != 40000 {
		t.Errorf("已收应为 40000, got %d", sum.ReceivedCent)
	}
	if sum.BalanceCent != 60000 {
		t.Errorf("余额应为 60000, got %d", sum.BalanceCent)
	}
	if sum.OverdueCent != 20000 { // p1 剩余 20000 且逾期；p2 未逾期不计
		t.Errorf("逾期额应为 20000, got %d", sum.OverdueCent)
	}

	// is_overdue 派生：p1 逾期未收满，p2 未逾期
	plans, _ := svc.ListPlans(ctx, cid)
	if len(plans) != 2 {
		t.Fatalf("应有 2 个计划, got %d", len(plans))
	}
	if !plans[0].IsOverdue {
		t.Errorf("p1 应标记逾期")
	}
	if plans[1].IsOverdue {
		t.Errorf("p2 不应标记逾期")
	}
	_ = p2
}

// 删除回款记录后计划状态与汇总回退
func TestPaymentDeleteRecord(t *testing.T) {
	db := setupPaymentDB(t)
	cid := seedPayment(t, db, "a")
	svc := NewPaymentService(db)
	ctx := context.Background()

	p, _ := svc.CreatePlan(ctx, PlanInput{ContractID: cid, PeriodNo: 1, DueDate: "2026-06-01", AmountCent: 50000})
	pid := p.ID
	svc.CreateRecords(ctx, cid, []RecordInput{{PlanID: strPtr(pid), AmountCent: 50000, PaidAt: "2026-06-02"}}, 1)

	var rec models.PaymentRecord
	db.Where("plan_id = ?", pid).First(&rec)
	if err := svc.DeleteRecord(ctx, rec.ID, cid, 1); err != nil {
		t.Fatal(err)
	}
	if st := planStatus(t, db, pid); st != models.PlanPending {
		t.Fatalf("删除记录后计划应回退 pending, got %s", st)
	}
	sum, _ := svc.Summary(ctx, cid)
	if sum.ReceivedCent != 0 || sum.BalanceCent != 100000 {
		t.Fatalf("删除记录后已收应为 0/余额 100000, got recv=%d bal=%d", sum.ReceivedCent, sum.BalanceCent)
	}
}

func planStatus(t *testing.T, db *gorm.DB, id uint) string {
	t.Helper()
	var p models.PaymentPlan
	if err := db.Take(&p, id).Error; err != nil {
		t.Fatal(err)
	}
	return p.Status
}

func strPtr(s any) *string {
	switch v := s.(type) {
	case uint:
		x := fmt.Sprintf("%d", v)
		return &x
	case string:
		return &v
	}
	return nil
}
