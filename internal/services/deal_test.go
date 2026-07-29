package services

import (
	"context"
	"testing"

	"bss/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupDealDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if sqlDB, _ := db.DB(); sqlDB != nil {
		sqlDB.SetMaxOpenConns(1)
	}
	for _, ddl := range []string{
		`CREATE TABLE customers (id INTEGER PRIMARY KEY AUTOINCREMENT, code TEXT UNIQUE, name TEXT UNIQUE, industry TEXT DEFAULT '', source TEXT DEFAULT '', level TEXT DEFAULT '', owner_id INTEGER, remark TEXT DEFAULT '', created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE deals (id INTEGER PRIMARY KEY AUTOINCREMENT, code TEXT UNIQUE, customer_id INTEGER, title TEXT, amount_cent INTEGER DEFAULT 0, probability INTEGER DEFAULT 10, expected_sign_date TEXT, status TEXT DEFAULT 'prospecting', lost_reason TEXT DEFAULT '', owner_id INTEGER, remark TEXT DEFAULT '', created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE deal_contracts (id INTEGER PRIMARY KEY AUTOINCREMENT, deal_id INTEGER, contract_id INTEGER, created_at DATETIME, UNIQUE(deal_id, contract_id))`,
		`CREATE TABLE code_counters (prefix TEXT NOT NULL, year INTEGER NOT NULL, seq INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (prefix, year))`,
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&models.Customer{Code: "KH-1", Name: "测试客户", OwnerID: 1}).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func mustCreateDeal(t *testing.T, svc *DealService, amount int64) *models.Deal {
	t.Helper()
	d, err := svc.Create(context.Background(), DealInput{CustomerID: 1, Title: "测试商单", AmountCent: amount}, 1)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestDealFlowLegalAndRollback(t *testing.T) {
	svc := NewDealService(setupDealDB(t))
	ctx := context.Background()
	d := mustCreateDeal(t, svc, 100000)

	// 正向推进
	if _, err := svc.ChangeStatus(ctx, d.ID, models.DealQualifying, "", false); err != nil {
		t.Fatalf("合法推进失败: %v", err)
	}
	// 跳级（qualifying → won）非法
	if _, err := svc.ChangeStatus(ctx, d.ID, models.DealWon, "", false); err != ErrInvalidTransition {
		t.Errorf("跳级应被拒绝，得到 %v", err)
	}
	// 回退（qualifying → prospecting）合法
	d2, err := svc.ChangeStatus(ctx, d.ID, models.DealProspecting, "", false)
	if err != nil {
		t.Fatalf("回退应合法: %v", err)
	}
	if d2.Probability != 10 {
		t.Errorf("回退后概率应重置为 10，得到 %d", d2.Probability)
	}
}

func TestDealLostReasonRequired(t *testing.T) {
	svc := NewDealService(setupDealDB(t))
	ctx := context.Background()
	d := mustCreateDeal(t, svc, 50000)

	if _, err := svc.ChangeStatus(ctx, d.ID, models.DealLost, "", false); err != ErrLostReasonRequired {
		t.Errorf("缺输单原因应拒绝，得到 %v", err)
	}
	if _, err := svc.ChangeStatus(ctx, d.ID, models.DealLost, "not_a_reason", false); err != ErrLostReasonRequired {
		t.Errorf("非法枚举应拒绝，得到 %v", err)
	}
	d2, err := svc.ChangeStatus(ctx, d.ID, models.DealLost, "competitor", false)
	if err != nil {
		t.Fatalf("合法输单失败: %v", err)
	}
	if d2.Probability != 0 {
		t.Errorf("输单后概率应为 0，得到 %d", d2.Probability)
	}
	// 终态不可逆
	if _, err := svc.ChangeStatus(ctx, d.ID, models.DealProspecting, "", false); err != ErrInvalidTransition {
		t.Errorf("lost 后复活应拒绝，得到 %v", err)
	}
}

func TestDealTerminalLock(t *testing.T) {
	svc := NewDealService(setupDealDB(t))
	ctx := context.Background()
	d := mustCreateDeal(t, svc, 100000)
	svc.ChangeStatus(ctx, d.ID, models.DealQualifying, "", false)
	svc.ChangeStatus(ctx, d.ID, models.DealProposal, "", false)
	svc.ChangeStatus(ctx, d.ID, models.DealNegotiating, "", false)
	if _, err := svc.ChangeStatus(ctx, d.ID, models.DealWon, "", false); err != nil {
		t.Fatal(err)
	}
	// won 后改金额被拒
	err := svc.Update(ctx, d.ID, DealInput{CustomerID: 1, Title: "测试商单", AmountCent: 999999})
	if err != ErrDealLocked {
		t.Errorf("终态改金额应锁定，得到 %v", err)
	}
	// won 后仅 remark 可改
	if err := svc.Update(ctx, d.ID, DealInput{CustomerID: 1, Title: "测试商单", AmountCent: 100000, Remark: "补充说明"}); err != nil {
		t.Errorf("终态改 remark 应允许: %v", err)
	}
	// won 后删除被拒
	if err := svc.Delete(ctx, d.ID); err != ErrDealClosed {
		t.Errorf("终态删除应拒绝，得到 %v", err)
	}
}

func TestDealExitWarningAndForce(t *testing.T) {
	svc := NewDealService(setupDealDB(t))
	ctx := context.Background()
	d := mustCreateDeal(t, svc, 0) // 金额 0
	svc.ChangeStatus(ctx, d.ID, models.DealQualifying, "", false)

	// 金额 0 推进 proposal → 软校验 warning
	if _, err := svc.ChangeStatus(ctx, d.ID, models.DealProposal, "", false); err != ErrExitWarning {
		t.Errorf("应返回退出标准 warning，得到 %v", err)
	}
	// force 通过
	if _, err := svc.ChangeStatus(ctx, d.ID, models.DealProposal, "", true); err != nil {
		t.Errorf("force 应通过: %v", err)
	}
}

func TestDealForecast(t *testing.T) {
	db := setupDealDB(t)
	svc := NewDealService(db)
	ctx := context.Background()

	// 进行中：10 万 * 10% + 20 万 * 60%
	mustCreateDeal(t, svc, 100000) // prospecting 10%
	d2 := mustCreateDeal(t, svc, 200000)
	svc.ChangeStatus(ctx, d2.ID, models.DealQualifying, "", false)
	svc.ChangeStatus(ctx, d2.ID, models.DealProposal, "", false) // 60%
	// 已关闭的不计入
	d3 := mustCreateDeal(t, svc, 500000)
	svc.ChangeStatus(ctx, d3.ID, models.DealLost, "budget", false)

	r, err := svc.Forecast(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if r.OpenCount != 2 {
		t.Errorf("进行中单数 = %d，期望 2", r.OpenCount)
	}
	if r.TotalCent != 300000 {
		t.Errorf("进行中金额合计 = %d，期望 300000", r.TotalCent)
	}
	// 100000*10/100 + 200000*60/100 = 10000 + 120000 = 130000
	if r.WeightedCent != 130000 {
		t.Errorf("加权预测 = %d，期望 130000", r.WeightedCent)
	}
}
