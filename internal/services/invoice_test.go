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

func setupInvoiceDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "inv.db")), &gorm.Config{
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Employee{}, &models.Customer{}, &models.Contract{}, &models.Invoice{}, &models.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS code_counters (
		prefix TEXT NOT NULL, year INTEGER NOT NULL, seq INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (prefix, year))`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func seedInvoiceContract(t *testing.T, db *gorm.DB) uint {
	t.Helper()
	emp := models.Employee{Name: "o", Email: "inv@x.com", Role: "sales", Phone: "1"}
	db.Create(&emp)
	cust := models.Customer{Code: "KH-INV", Name: "ic", OwnerID: emp.ID}
	db.Create(&cust)
	ct := models.Contract{Code: "HT-INV", CustomerID: cust.ID, Title: "开票合同", AmountCent: 100000, Status: models.ContractSigned, OwnerID: emp.ID}
	db.Create(&ct)
	return ct.ID
}

func TestInvoiceCreateIssueVoid(t *testing.T) {
	db := setupInvoiceDB(t)
	svc := NewInvoiceService(db)
	ctx := context.Background()
	ctID := seedInvoiceContract(t, db)

	inv, err := svc.Create(ctx, InvoiceInput{ContractID: ctID, AmountCent: 30000}, 1)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if inv.Code == "" || inv.Status != models.InvoiceDraft {
		t.Fatalf("新建应为 draft, got %s", inv.Status)
	}
	if err := svc.Issue(ctx, inv.ID); err != nil {
		t.Fatalf("issue: %v", err)
	}
	var i1 models.Invoice
	db.Take(&i1, inv.ID)
	if i1.Status != models.InvoiceIssued || i1.IssuedAt == "" {
		t.Fatalf("开票后状态/开票日异常: %s %q", i1.Status, i1.IssuedAt)
	}
	// 已开票不可编辑
	if err := svc.Update(ctx, inv.ID, InvoiceInput{AmountCent: 40000}); err != ErrInvoiceNotDraft {
		t.Fatalf("已开票编辑应拒, got %v", err)
	}
	// 作废
	if err := svc.Void(ctx, inv.ID); err != nil {
		t.Fatalf("void: %v", err)
	}
	var i2 models.Invoice
	db.Take(&i2, inv.ID)
	if i2.Status != models.InvoiceVoided {
		t.Fatalf("作废后应为 voided, got %s", i2.Status)
	}
	// 作废后不可重复作废
	if err := svc.Void(ctx, inv.ID); err != ErrInvoiceInvalidState {
		t.Fatalf("重复作废应拒, got %v", err)
	}
}

func TestInvoiceAmountExceed(t *testing.T) {
	db := setupInvoiceDB(t)
	svc := NewInvoiceService(db)
	ctx := context.Background()
	ctID := seedInvoiceContract(t, db) // 合同 100000

	if _, err := svc.Create(ctx, InvoiceInput{ContractID: ctID, AmountCent: 60000}, 1); err != nil {
		t.Fatalf("首张应成功: %v", err)
	}
	// 累计 60000 + 50000 = 110000 > 100000 → 超额
	if _, err := svc.Create(ctx, InvoiceInput{ContractID: ctID, AmountCent: 50000}, 1); err != ErrInvoiceAmountExceed {
		t.Fatalf("超额应 ErrInvoiceAmountExceed, got %v", err)
	}
	// 恰好 100000 应允许
	if _, err := svc.Create(ctx, InvoiceInput{ContractID: ctID, AmountCent: 40000}, 1); err != nil {
		t.Fatalf("恰好等于合同额应允许: %v", err)
	}
}

func TestInvoiceDeleteDraftOnly(t *testing.T) {
	db := setupInvoiceDB(t)
	svc := NewInvoiceService(db)
	ctx := context.Background()
	ctID := seedInvoiceContract(t, db)
	inv, _ := svc.Create(ctx, InvoiceInput{ContractID: ctID, AmountCent: 10000}, 1)
	if err := svc.Delete(ctx, inv.ID); err != nil {
		t.Fatalf("删除 draft 应成功: %v", err)
	}
	// 已删除再取应不存在
	if _, err := svc.Get(ctx, inv.ID); err != ErrInvoiceMissing {
		t.Fatalf("删除后应为 missing, got %v", err)
	}
}
