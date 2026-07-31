package services

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"

	"bss/internal/models"
)

func mkCustomer(t *testing.T, db *gorm.DB, name, code string) models.Customer {
	t.Helper()
	c := models.Customer{Name: name, Code: code}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("create customer %s: %v", name, err)
	}
	return c
}

func mkEmployee(t *testing.T, db *gorm.DB, email string) models.Employee {
	t.Helper()
	e := models.Employee{Name: "emp-" + email, Email: email, Role: models.RoleAdmin}
	if err := db.Create(&e).Error; err != nil {
		t.Fatalf("create employee %s: %v", email, err)
	}
	return e
}

func TestFindDuplicates_Basic(t *testing.T) {
	gdb := newTestDB(t)
	a := mkCustomer(t, gdb, "A公司", "KH-T1")
	b := mkCustomer(t, gdb, "B公司", "KH-T2")
	c := mkCustomer(t, gdb, "C公司", "KH-T3")
	d := mkCustomer(t, gdb, "D公司", "KH-T4")
	// A,B 共享手机
	gdb.Create(&models.Contact{CustomerID: a.ID, Name: "ca", Phone: "13800001111"})
	gdb.Create(&models.Contact{CustomerID: b.ID, Name: "cb", Phone: "13800001111"})
	// C,D 共享邮箱
	gdb.Create(&models.Contact{CustomerID: c.ID, Name: "cc", Email: "dup@x.com"})
	gdb.Create(&models.Contact{CustomerID: d.ID, Name: "cd", Email: "dup@x.com"})
	// 孤立联系人（不重复）
	gdb.Create(&models.Contact{CustomerID: a.ID, Name: "ca2", Phone: "13900000000"})

	groups, err := FindDuplicates(context.Background(), gdb)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 {
		t.Fatalf("groups=%d, want 2 (phone + email)", len(groups))
	}
	for _, g := range groups {
		if len(g.Customers) != 2 {
			t.Errorf("group %s/%s has %d customers, want 2", g.Field, g.Value, len(g.Customers))
		}
	}
}

func TestMergeCustomers_Basic(t *testing.T) {
	gdb := newTestDB(t)
	emp := mkEmployee(t, gdb, "owner@x.com")
	a := mkCustomer(t, gdb, "主客户", "KH-M1")
	b := mkCustomer(t, gdb, "从客户", "KH-M2")
	// B 的下游数据
	if err := gdb.Create(&models.Contact{CustomerID: b.ID, Name: "cb", Phone: "13800002222", IsPrimary: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&models.Contact{CustomerID: b.ID, Name: "cb2", Phone: "13800002223"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&models.Deal{CustomerID: b.ID, Code: "SD-M1", Title: "deal", Status: "won", OwnerID: emp.ID}).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&models.Contract{CustomerID: b.ID, Code: "HT-M1", Title: "contract", Status: "active", OwnerID: emp.ID}).Error; err != nil {
		t.Fatal(err)
	}

	if err := MergeCustomers(context.Background(), gdb, a.ID, b.ID); err != nil {
		t.Fatalf("MergeCustomers: %v", err)
	}
	// 联系人迁移到 A
	var contacts []models.Contact
	gdb.Where("customer_id = ?", a.ID).Find(&contacts)
	if len(contacts) != 2 {
		t.Errorf("A 的联系人数 = %d, want 2", len(contacts))
	}
	// 商单迁移
	var deals []models.Deal
	gdb.Where("customer_id = ?", a.ID).Find(&deals)
	if len(deals) != 1 {
		t.Errorf("A 的商单数 = %d, want 1", len(deals))
	}
	// 合同迁移
	var contracts []models.Contract
	gdb.Where("customer_id = ?", a.ID).Find(&contracts)
	if len(contracts) != 1 {
		t.Errorf("A 的合同数 = %d, want 1", len(contracts))
	}
	// B 软删
	var gone models.Customer
	err := gdb.Where("id = ?", b.ID).First(&gone).Error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("B 应已软删，got err=%v", err)
	}
	// A 仍在
	var pa models.Customer
	if err := gdb.Where("id = ?", a.ID).First(&pa).Error; err != nil {
		t.Errorf("A 应存在: %v", err)
	}
}

func TestMergeCustomers_Same(t *testing.T) {
	gdb := newTestDB(t)
	a := mkCustomer(t, gdb, "X", "KH-S1")
	err := MergeCustomers(context.Background(), gdb, a.ID, a.ID)
	if !errors.Is(err, ErrDupSameCustomer) {
		t.Errorf("want ErrDupSameCustomer, got %v", err)
	}
}

func TestMergeCustomers_Missing(t *testing.T) {
	gdb := newTestDB(t)
	a := mkCustomer(t, gdb, "X", "KH-MI1")
	err := MergeCustomers(context.Background(), gdb, a.ID, 99999)
	if !errors.Is(err, ErrDupCustomerMissing) {
		t.Errorf("want ErrDupCustomerMissing, got %v", err)
	}
}
