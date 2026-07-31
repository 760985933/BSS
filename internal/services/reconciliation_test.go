package services

import (
	"context"
	"errors"
	"testing"

	"bss/internal/models"
)

func TestReconcile_Basic(t *testing.T) {
	gdb := newTestDB(t)
	emp := mkEmployee(t, gdb, "r-owner@x.com")
	cust := mkCustomer(t, gdb, "对账客户", "KH-R1")
	contr := models.Contract{CustomerID: cust.ID, Code: "HT-R1", Title: "c", Status: "active", OwnerID: emp.ID}
	if err := gdb.Create(&contr).Error; err != nil {
		t.Fatal(err)
	}
	pr := models.PaymentRecord{ContractID: contr.ID, AmountCent: 1000, PaidAt: "2026-07-01", Method: "bank", CreatedBy: emp.ID}
	if err := gdb.Create(&pr).Error; err != nil {
		t.Fatal(err)
	}

	n, err := CreateBankStatements(context.Background(), gdb, []BankStatementInput{
		{TransDate: "2026-07-01", Counterparty: "甲方", AmountCent: 1000, Direction: "income"},
	})
	if err != nil || n != 1 {
		t.Fatalf("create: %v n=%d", err, n)
	}

	bs, err := ListBankStatements(context.Background(), gdb, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(bs) != 1 {
		t.Fatalf("list=%d want 1", len(bs))
	}

	if err := Reconcile(context.Background(), gdb, bs[0].ID, pr.ID); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// 第二条流水再勾同一回款 -> AlreadyReconciled
	if _, err := CreateBankStatements(context.Background(), gdb, []BankStatementInput{
		{TransDate: "2026-07-02", Counterparty: "甲方2", AmountCent: 1000, Direction: "income"},
	}); err != nil {
		t.Fatal(err)
	}
	bs2, _ := ListBankStatements(context.Background(), gdb, nil)
	var stmt2 models.BankStatement
	for _, b := range bs2 {
		if b.PaymentRecordID == nil {
			stmt2 = b
			break
		}
	}
	if err := Reconcile(context.Background(), gdb, stmt2.ID, pr.ID); !errors.Is(err, ErrAlreadyReconciled) {
		t.Errorf("want AlreadyReconciled, got %v", err)
	}

	sum, err := ReconciliationSummary(context.Background(), gdb)
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.CompanyOnly) != 0 {
		t.Errorf("company_only=%d want 0（回款已勾对）", len(sum.CompanyOnly))
	}
	if len(sum.BankOnly) != 1 {
		t.Errorf("bank_only=%d want 1（第二条流水未勾对）", len(sum.BankOnly))
	}

	// 取消勾对后，回款变为企业已收银行未收
	if err := Unreconcile(context.Background(), gdb, bs[0].ID); err != nil {
		t.Fatal(err)
	}
	sum2, _ := ReconciliationSummary(context.Background(), gdb)
	if len(sum2.CompanyOnly) != 1 {
		t.Errorf("after unreconcile company_only=%d want 1", len(sum2.CompanyOnly))
	}
}
