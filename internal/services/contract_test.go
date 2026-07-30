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

func setupContractDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "c.db")), &gorm.Config{
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Employee{}, &models.Customer{}, &models.Deal{},
		&models.Contract{}, &models.DealContract{}, &models.Attachment{},
		&models.PaymentPlan{}, &models.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	// code_counters 无 GORM 模型（纯 SQL 计数器），测试库需手动建表
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS code_counters (
		prefix TEXT NOT NULL, year INTEGER NOT NULL, seq INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (prefix, year))`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

type contractSeed struct {
	custID, otherCustID, ownerID, wonDealID, lostDealID, otherWonDealID uint
}

func seedContract(t *testing.T, db *gorm.DB) contractSeed {
	t.Helper()
	owner := models.Employee{Name: "owner", Email: "owner@x.com", Dept: "s", Role: "sales", Phone: "1"}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatal(err)
	}
	cust := models.Customer{Code: "KH-1", Name: "c1", OwnerID: owner.ID}
	other := models.Customer{Code: "KH-2", Name: "c2", OwnerID: owner.ID}
	db.Create(&cust)
	db.Create(&other)
	won := models.Deal{Code: "SD-1", CustomerID: cust.ID, Title: "d1", Status: models.DealWon, OwnerID: owner.ID, AmountCent: 10000, Probability: 100}
	lost := models.Deal{Code: "SD-2", CustomerID: cust.ID, Title: "d2", Status: models.DealLost, OwnerID: owner.ID}
	ow := models.Deal{Code: "SD-3", CustomerID: other.ID, Title: "d3", Status: models.DealWon, OwnerID: owner.ID, AmountCent: 20000, Probability: 100}
	db.Create(&won)
	db.Create(&lost)
	db.Create(&ow)
	return contractSeed{cust.ID, other.ID, owner.ID, won.ID, lost.ID, ow.ID}
}

// 创建 + 关联商单校验（仅 won + 同客户）
func TestContractCreateAndDealLink(t *testing.T) {
	db := setupContractDB(t)
	s := seedContract(t, db)
	svc := NewContractService(db)
	ctx := context.Background()

	ct, err := svc.Create(ctx, ContractInput{CustomerID: s.custID, Title: "合同A", AmountCent: 5000, DealIDs: []uint{s.wonDealID}}, s.ownerID)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if ct.Code == "" || len(ct.Code) < 3 {
		t.Fatalf("单号未生成: %q", ct.Code)
	}
	if ct.Status != models.ContractDraft {
		t.Fatalf("默认状态应为 draft, got %s", ct.Status)
	}
	var n int64
	db.Model(&models.DealContract{}).Where("contract_id = ?", ct.ID).Count(&n)
	if n != 1 {
		t.Fatalf("应关联 1 个 won 商单, got %d", n)
	}

	// 跨客户商单关联被拒
	if _, err := svc.Create(ctx, ContractInput{CustomerID: s.custID, Title: "跨客户", DealIDs: []uint{s.otherWonDealID}}, s.ownerID); err != ErrCrossCustomerLink {
		t.Fatalf("期望 ErrCrossCustomerLink, got %v", err)
	}
	// 非 won 商单关联被拒
	if _, err := svc.Create(ctx, ContractInput{CustomerID: s.custID, Title: "非won", DealIDs: []uint{s.lostDealID}}, s.ownerID); err != ErrDealNotWon {
		t.Fatalf("期望 ErrDealNotWon, got %v", err)
	}
}

// 状态机：草稿→待签→回退；取消旁路锁定
func TestContractStateMachine(t *testing.T) {
	db := setupContractDB(t)
	s := seedContract(t, db)
	svc := NewContractService(db)
	ctx := context.Background()
	ct, _ := svc.Create(ctx, ContractInput{CustomerID: s.custID, Title: "x", AmountCent: 1}, s.ownerID)

	if _, err := svc.ChangeStatus(ctx, ct.ID, models.ContractPending, ""); err != nil {
		t.Fatalf("draft->pending: %v", err)
	}
	if _, err := svc.ChangeStatus(ctx, ct.ID, models.ContractDraft, ""); err != nil {
		t.Fatalf("pending->draft 回退: %v", err)
	}
	if _, err := svc.ChangeStatus(ctx, ct.ID, models.ContractCancelled, ""); err != nil {
		t.Fatalf("draft->cancelled: %v", err)
	}
	if _, err := svc.ChangeStatus(ctx, ct.ID, models.ContractPending, ""); err != ErrContractInvalidTransition {
		t.Fatalf("cancelled 应锁定, got %v", err)
	}
}

// 签约主链路：signed->performing->completed 合法；跳级非法
func TestContractSignedFlow(t *testing.T) {
	db := setupContractDB(t)
	s := seedContract(t, db)
	svc := NewContractService(db)
	ctx := context.Background()
	ct, _ := svc.Create(ctx, ContractInput{CustomerID: s.custID, Title: "x", AmountCent: 1}, s.ownerID)
	svc.ChangeStatus(ctx, ct.ID, models.ContractPending, "")
	svc.ChangeStatus(ctx, ct.ID, models.ContractSigned, "")
	if _, err := svc.ChangeStatus(ctx, ct.ID, models.ContractCompleted, ""); err != ErrContractInvalidTransition {
		t.Fatalf("signed->completed 应非法, got %v", err)
	}
	if _, err := svc.ChangeStatus(ctx, ct.ID, models.ContractPerforming, ""); err != nil {
		t.Fatalf("signed->performing: %v", err)
	}
	if _, err := svc.ChangeStatus(ctx, ct.ID, models.ContractCompleted, ""); err != nil {
		t.Fatalf("performing->completed: %v", err)
	}
	if _, err := svc.ChangeStatus(ctx, ct.ID, models.ContractDraft, ""); err != ErrContractInvalidTransition {
		t.Fatalf("completed 应锁定, got %v", err)
	}
}

// terminated 必填原因
func TestContractTerminateReason(t *testing.T) {
	db := setupContractDB(t)
	s := seedContract(t, db)
	svc := NewContractService(db)
	ctx := context.Background()
	ct, _ := svc.Create(ctx, ContractInput{CustomerID: s.custID, Title: "x", AmountCent: 1}, s.ownerID)
	svc.ChangeStatus(ctx, ct.ID, models.ContractPending, "")
	svc.ChangeStatus(ctx, ct.ID, models.ContractSigned, "")
	if _, err := svc.ChangeStatus(ctx, ct.ID, models.ContractTerminated, ""); err != ErrTerminateReasonRequired {
		t.Fatalf("终止应必填原因, got %v", err)
	}
	ct2, err := svc.ChangeStatus(ctx, ct.ID, models.ContractTerminated, "客户违约")
	if err != nil {
		t.Fatalf("terminated: %v", err)
	}
	if ct2.TerminateReason != "客户违约" {
		t.Fatalf("terminate_reason 未保存: %q", ct2.TerminateReason)
	}
}

// 终态锁定：signed 后金额/关联商单只读
func TestContractFieldLocked(t *testing.T) {
	db := setupContractDB(t)
	s := seedContract(t, db)
	svc := NewContractService(db)
	ctx := context.Background()
	ct, _ := svc.Create(ctx, ContractInput{CustomerID: s.custID, Title: "x", AmountCent: 1000}, s.ownerID)
	svc.ChangeStatus(ctx, ct.ID, models.ContractPending, "")
	svc.ChangeStatus(ctx, ct.ID, models.ContractSigned, "")
	if err := svc.Update(ctx, ct.ID, ContractInput{Title: "x", AmountCent: 9999}); err != ErrContractFieldLocked {
		t.Fatalf("signed 后改金额应锁, got %v", err)
	}
	if err := svc.Update(ctx, ct.ID, ContractInput{Title: "新标题", AmountCent: 1000}); err != nil {
		t.Fatalf("signed 不改金额应可保存: %v", err)
	}
	if err := svc.ReplaceDeals(ctx, ct.ID, []uint{s.wonDealID}); err != ErrContractFieldLocked {
		t.Fatalf("signed 后改关联应锁, got %v", err)
	}
}

// 关联商单：非 won/跨客户拒绝；重关联成功并记审计
func TestContractReplaceDeals(t *testing.T) {
	db := setupContractDB(t)
	s := seedContract(t, db)
	svc := NewContractService(db)
	ctx := context.Background()
	ct, _ := svc.Create(ctx, ContractInput{CustomerID: s.custID, Title: "x", AmountCent: 1, DealIDs: []uint{s.wonDealID}}, s.ownerID)
	if err := svc.ReplaceDeals(ctx, ct.ID, []uint{s.lostDealID}); err != ErrDealNotWon {
		t.Fatalf("期望 ErrDealNotWon, got %v", err)
	}
	if err := svc.ReplaceDeals(ctx, ct.ID, []uint{s.otherWonDealID}); err != ErrCrossCustomerLink {
		t.Fatalf("期望 ErrCrossCustomerLink, got %v", err)
	}
	if err := svc.ReplaceDeals(ctx, ct.ID, []uint{s.wonDealID}); err != nil {
		t.Fatalf("重关联应成功: %v", err)
	}
	var n int64
	db.Model(&models.DealContract{}).Where("contract_id = ?", ct.ID).Count(&n)
	if n != 1 {
		t.Fatalf("重关联后应有 1 条, got %d", n)
	}
	var auditN int64
	db.Model(&models.AuditLog{}).Where("entity_type = 'contracts' AND entity_id = ? AND action = 'update'", ct.ID).Count(&auditN)
	if auditN == 0 {
		t.Fatalf("关联变更应写审计")
	}
}

// 删除：draft 可删；signed 锁定；有回款计划禁删
func TestContractDelete(t *testing.T) {
	db := setupContractDB(t)
	s := seedContract(t, db)
	svc := NewContractService(db)
	ctx := context.Background()
	ct, _ := svc.Create(ctx, ContractInput{CustomerID: s.custID, Title: "x", AmountCent: 1}, s.ownerID)
	if err := svc.Delete(ctx, ct.ID); err != nil {
		t.Fatalf("draft 应可删: %v", err)
	}
	ct2, _ := svc.Create(ctx, ContractInput{CustomerID: s.custID, Title: "y", AmountCent: 1}, s.ownerID)
	svc.ChangeStatus(ctx, ct2.ID, models.ContractPending, "")
	svc.ChangeStatus(ctx, ct2.ID, models.ContractSigned, "")
	if err := svc.Delete(ctx, ct2.ID); err != ErrContractLocked {
		t.Fatalf("signed 应不可删, got %v", err)
	}
	ct3, _ := svc.Create(ctx, ContractInput{CustomerID: s.custID, Title: "z", AmountCent: 1}, s.ownerID)
	db.Create(&models.PaymentPlan{ContractID: ct3.ID, PeriodNo: 1, DueDate: "2026-01-01", AmountCent: 100})
	if err := svc.Delete(ctx, ct3.ID); err != ErrContractHasChildren {
		t.Fatalf("有回款应不可删, got %v", err)
	}
}

// 附件：白名单/大小校验 + 记录 CRUD
func TestContractAttachment(t *testing.T) {
	db := setupContractDB(t)
	s := seedContract(t, db)
	svc := NewContractService(db)
	ctx := context.Background()
	if err := ValidateAttachment("a.pdf", 1024); err != nil {
		t.Fatalf("pdf 应合法: %v", err)
	}
	if err := ValidateAttachment("a.exe", 1024); err == nil {
		t.Fatalf("exe 应被拒")
	}
	if err := ValidateAttachment("a.pdf", 21*1024*1024); err == nil {
		t.Fatalf("超 20MB 应被拒")
	}
	if err := ValidateAttachment("a.pdf", 0); err == nil {
		t.Fatalf("空文件应被拒")
	}
	a, err := svc.CreateAttachment(ctx, models.Attachment{EntityType: "contract", EntityID: 1, FileName: "f.pdf", FilePath: "2026/07/x.pdf", FileSize: 10, Mime: "application/pdf", UploadedBy: s.ownerID})
	if err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	list, err := svc.ListAttachments(ctx, "contract", 1)
	if err != nil || len(list) != 1 {
		t.Fatalf("list attachment: %v len=%d", err, len(list))
	}
	got, err := svc.GetAttachment(ctx, a.ID)
	if err != nil || got.FileName != "f.pdf" {
		t.Fatalf("get attachment: %v", err)
	}
	if err := svc.DeleteAttachment(ctx, a.ID); err != nil {
		t.Fatalf("delete attachment: %v", err)
	}
	if _, err := svc.GetAttachment(ctx, a.ID); err != ErrAttachmentMissing {
		t.Fatalf("删除后应不存在, got %v", err)
	}
}
