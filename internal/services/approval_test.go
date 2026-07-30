package services

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"bss/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupApprovalDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "a.db")), &gorm.Config{
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Employee{}, &models.Customer{}, &models.Deal{},
		&models.Contract{}, &models.DealContract{}, &models.Approval{}, &models.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS code_counters (
		prefix TEXT NOT NULL, year INTEGER NOT NULL, seq INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (prefix, year))`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func seedApprovalActors(t *testing.T, db *gorm.DB) (ownerID uint) {
	t.Helper()
	owner := models.Employee{Name: "owner", Email: "appr@x.com", Dept: "s", Role: "sales", Phone: "1"}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatal(err)
	}
	cust := models.Customer{Code: "KH-AP1", Name: "ac1", OwnerID: owner.ID}
	if err := db.Create(&cust).Error; err != nil {
		t.Fatal(err)
	}
	// 写入供合同/商单引用
	_ = cust
	return owner.ID
}

// 合同签约审批：提交后待审批 → 审批通过 → 签约；驳回 → 回到 pending
func TestApprovalContractSignFlow(t *testing.T) {
	db := setupApprovalDB(t)
	ownerID := seedApprovalActors(t, db)
	svc := NewApprovalService(db)
	ctx := context.Background()

	ct, _ := NewContractService(db).Create(ctx, ContractInput{CustomerID: 1, Title: "待审合同", AmountCent: 5000}, ownerID)
	NewContractService(db).ChangeStatus(ctx, ct.ID, models.ContractPending, "")

	ap, err := svc.Create(ctx, ApprovalInput{EntityType: "contract", EntityID: ct.ID, Kind: models.ApprovalContractSign}, ownerID)
	if err != nil {
		t.Fatalf("create approval: %v", err)
	}
	if ap.Code == "" || ap.Status != models.ApprovalPending {
		t.Fatalf("审批单状态应为 pending, got %s", ap.Status)
	}
	// 合同应进入待审批
	var c models.Contract
	db.Take(&c, ct.ID)
	if c.Status != models.ContractPendingApproval {
		t.Fatalf("合同应进入 pending_approval, got %s", c.Status)
	}
	// 审批通过 → 签约
	if err := svc.Approve(ctx, ap.ID, 999); err != nil {
		t.Fatalf("approve: %v", err)
	}
	db.Take(&c, ct.ID)
	if c.Status != models.ContractSigned {
		t.Fatalf("审批通过后合同应 signed, got %s", c.Status)
	}
	var ap2 models.Approval
	db.Take(&ap2, ap.ID)
	if ap2.Status != models.ApprovalApproved || ap2.ApproverID != 999 {
		t.Fatalf("审批单应为 approved, got %s approver=%d", ap2.Status, ap2.ApproverID)
	}

	// 驳回路径：另一个合同
	ct2, _ := NewContractService(db).Create(ctx, ContractInput{CustomerID: 1, Title: "待驳回", AmountCent: 5000}, ownerID)
	NewContractService(db).ChangeStatus(ctx, ct2.ID, models.ContractPending, "")
	apR, _ := svc.Create(ctx, ApprovalInput{EntityType: "contract", EntityID: ct2.ID, Kind: models.ApprovalContractSign}, ownerID)
	if err := svc.Reject(ctx, apR.ID, 999, "风险过高"); err != nil {
		t.Fatalf("reject: %v", err)
	}
	var c2 models.Contract
	db.Take(&c2, ct2.ID)
	if c2.Status != models.ContractPending {
		t.Fatalf("驳回后合同应回退 pending, got %s", c2.Status)
	}
	var apR2 models.Approval
	db.Take(&apR2, apR.ID)
	if apR2.Status != models.ApprovalRejected || apR2.RejectReason != "风险过高" {
		t.Fatalf("审批单应 rejected, got %s reason=%q", apR2.Status, apR2.RejectReason)
	}
}

// 商单折扣审批：谈判中提交 → 待审批 → 审批通过 → 赢单且折扣落地
func TestApprovalDealDiscountFlow(t *testing.T) {
	db := setupApprovalDB(t)
	ownerID := seedApprovalActors(t, db)
	svc := NewApprovalService(db)
	ctx := context.Background()

	d, _ := NewDealService(db).Create(ctx, DealInput{CustomerID: 1, Title: "折扣单", AmountCent: 100000}, ownerID)
	NewDealService(db).ChangeStatus(ctx, d.ID, models.DealQualifying, "", false)
	NewDealService(db).ChangeStatus(ctx, d.ID, models.DealProposal, "", false)
	NewDealService(db).ChangeStatus(ctx, d.ID, models.DealNegotiating, "", false)

	ap, err := svc.Create(ctx, ApprovalInput{EntityType: "deal", EntityID: d.ID, Kind: models.ApprovalDealDiscount, AmountCent: 10000}, ownerID)
	if err != nil {
		t.Fatalf("create deal approval: %v", err)
	}
	var d1 models.Deal
	db.Take(&d1, d.ID)
	if d1.Status != models.DealPendingApproval {
		t.Fatalf("商单应进入 pending_approval, got %s", d1.Status)
	}
	if err := svc.Approve(ctx, ap.ID, 1); err != nil {
		t.Fatalf("approve: %v", err)
	}
	var d2 models.Deal
	db.Take(&d2, d.ID)
	if d2.Status != models.DealWon {
		t.Fatalf("审批通过后商单应 won, got %s", d2.Status)
	}
	if d2.DiscountAmountCent != 10000 {
		t.Fatalf("折扣金额应落地 10000, got %d", d2.DiscountAmountCent)
	}
}

// 非法前置状态：draft 合同不可直接提交签约审批
func TestApprovalInvalidEntityState(t *testing.T) {
	db := setupApprovalDB(t)
	ownerID := seedApprovalActors(t, db)
	svc := NewApprovalService(db)
	ctx := context.Background()
	ct, _ := NewContractService(db).Create(ctx, ContractInput{CustomerID: 1, Title: "草稿合同", AmountCent: 5000}, ownerID)
	if _, err := svc.Create(ctx, ApprovalInput{EntityType: "contract", EntityID: ct.ID, Kind: models.ApprovalContractSign}, ownerID); err != ErrApprovalEntityState {
		t.Fatalf("期望 ErrApprovalEntityState, got %v", err)
	}
}

// 审批单非待审状态：重复审批/驳回被拒；驳回缺原因被拒
func TestApprovalNotPendingGuards(t *testing.T) {
	db := setupApprovalDB(t)
	ownerID := seedApprovalActors(t, db)
	svc := NewApprovalService(db)
	ctx := context.Background()
	ct, _ := NewContractService(db).Create(ctx, ContractInput{CustomerID: 1, Title: "x", AmountCent: 5000}, ownerID)
	NewContractService(db).ChangeStatus(ctx, ct.ID, models.ContractPending, "")
	ap, _ := svc.Create(ctx, ApprovalInput{EntityType: "contract", EntityID: ct.ID, Kind: models.ApprovalContractSign}, ownerID)
	if err := svc.Approve(ctx, ap.ID, 1); err != nil {
		t.Fatalf("approve: %v", err)
	}
	// 已审批通过，再次审批应拒
	if err := svc.Approve(ctx, ap.ID, 1); err != ErrApprovalNotPending {
		t.Fatalf("重复审批应 ErrApprovalNotPending, got %v", err)
	}
	// 驳回缺原因应拒
	ct2, _ := NewContractService(db).Create(ctx, ContractInput{CustomerID: 1, Title: "y", AmountCent: 5000}, ownerID)
	NewContractService(db).ChangeStatus(ctx, ct2.ID, models.ContractPending, "")
	ap2, _ := svc.Create(ctx, ApprovalInput{EntityType: "contract", EntityID: ct2.ID, Kind: models.ApprovalContractSign}, ownerID)
	if err := svc.Reject(ctx, ap2.ID, 1, ""); err != ErrApprovalRejectReasonRequired {
		t.Fatalf("驳回缺原因应 ErrApprovalRejectReasonRequired, got %v", err)
	}
}
