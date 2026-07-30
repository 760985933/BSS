package services

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"bss/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupPoolDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "pool.db")), &gorm.Config{
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Employee{}, &models.Customer{}, &models.Deal{},
		&models.Contract{}, &models.CustomerPoolLog{}, &models.PoolSettings{},
		&models.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	return db
}

// newPoolCustomer 建一个公海客户（owner_id = 0）
func newPoolCustomer(t *testing.T, db *gorm.DB, code, name string) *models.Customer {
	t.Helper()
	c := models.Customer{Code: code, Name: name, OwnerID: 0}
	if err := db.Create(&c).Error; err != nil {
		t.Fatal(err)
	}
	return &c
}

// setClaimed 把客户的领取时间/跟进时间改到 daysAgo 天前，用于构造超期场景
func setClaimed(t *testing.T, db *gorm.DB, id uint, claimedDaysAgo, followedDaysAgo int) {
	t.Helper()
	now := time.Now().UTC()
	upd := map[string]any{"claimed_at": now.AddDate(0, 0, -claimedDaysAgo)}
	if followedDaysAgo < 0 {
		upd["last_followed_at"] = nil
	} else {
		upd["last_followed_at"] = now.AddDate(0, 0, -followedDaysAgo)
	}
	if err := db.Model(&models.Customer{}).Where("id = ?", id).Updates(upd).Error; err != nil {
		t.Fatal(err)
	}
}

func TestPoolClaimAndRelease(t *testing.T) {
	db := setupPoolDB(t)
	svc := NewPoolService(db)
	ctx := context.Background()

	sales := models.Employee{Name: "销售A", Email: "p-a@x.com", Dept: "S", Role: "sales", Status: "active"}
	if err := db.Create(&sales).Error; err != nil {
		t.Fatal(err)
	}
	cust := newPoolCustomer(t, db, "KH-P1", "公海客户1")

	// 公海列表应能查到
	list, total, err := svc.List(ctx, PoolFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("期望公海 1 个客户，实际 total=%d len=%d", total, len(list))
	}

	// 领取
	if err := svc.Claim(ctx, cust.ID, sales.ID); err != nil {
		t.Fatalf("领取失败: %v", err)
	}
	var got models.Customer
	db.Take(&got, cust.ID)
	if got.OwnerID != sales.ID {
		t.Fatalf("领取后 owner 应为 %d，实际 %d", sales.ID, got.OwnerID)
	}
	if got.ClaimedAt == nil || got.LastFollowedAt == nil {
		t.Fatal("领取后应写入 claimed_at 与 last_followed_at")
	}

	// 重复领取应失败
	if err := svc.Claim(ctx, cust.ID, sales.ID); !errors.Is(err, ErrNotInPool) {
		t.Fatalf("重复领取应返回 ErrNotInPool，实际 %v", err)
	}

	// 公海应为空
	if _, total, _ := svc.List(ctx, PoolFilter{}); total != 0 {
		t.Fatalf("领取后公海应为空，实际 %d", total)
	}

	// 释放
	if err := svc.Release(ctx, cust.ID, sales.ID, ""); err != nil {
		t.Fatalf("释放失败: %v", err)
	}
	db.Take(&got, cust.ID)
	if got.OwnerID != 0 {
		t.Fatalf("释放后应无主，实际 owner=%d", got.OwnerID)
	}
	if got.PoolReason != models.PoolReasonRelease {
		t.Fatalf("释放原因应为「%s」，实际「%s」", models.PoolReasonRelease, got.PoolReason)
	}

	// 重复释放应失败
	if err := svc.Release(ctx, cust.ID, sales.ID, ""); !errors.Is(err, ErrAlreadyInPool) {
		t.Fatalf("重复释放应返回 ErrAlreadyInPool，实际 %v", err)
	}

	// 流水应有领取 + 释放各一条
	logs, err := svc.Logs(ctx, cust.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 {
		t.Fatalf("期望 2 条流水，实际 %d", len(logs))
	}
	if logs[0].Action != models.PoolActionRelease || logs[1].Action != models.PoolActionClaim {
		t.Fatalf("流水顺序应为倒序[release, claim]，实际 [%s, %s]", logs[0].Action, logs[1].Action)
	}
}

func TestPoolClaimLimit(t *testing.T) {
	db := setupPoolDB(t)
	svc := NewPoolService(db)
	ctx := context.Background()

	sales := models.Employee{Name: "销售B", Email: "p-b@x.com", Dept: "S", Role: "sales", Status: "active"}
	db.Create(&sales)
	c1 := newPoolCustomer(t, db, "KH-P2", "公海客户2")
	c2 := newPoolCustomer(t, db, "KH-P3", "公海客户3")

	// 上限设为 1
	if _, err := svc.UpdateSettings(ctx, PoolSettingsInput{
		Enabled: true, MaxClaimPerSales: 1, IdleDaysNoFollow: 30, IdleDaysNoDeal: 60, ProtectDays: 7,
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Claim(ctx, c1.ID, sales.ID); err != nil {
		t.Fatalf("第一次领取应成功: %v", err)
	}
	if err := svc.Claim(ctx, c2.ID, sales.ID); !errors.Is(err, ErrClaimLimit) {
		t.Fatalf("超上限应返回 ErrClaimLimit，实际 %v", err)
	}

	// 上限 0 = 不限
	if _, err := svc.UpdateSettings(ctx, PoolSettingsInput{MaxClaimPerSales: 0}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Claim(ctx, c2.ID, sales.ID); err != nil {
		t.Fatalf("上限 0 表示不限，应领取成功: %v", err)
	}
}

func TestPoolReleaseGuards(t *testing.T) {
	db := setupPoolDB(t)
	svc := NewPoolService(db)
	ctx := context.Background()

	sales := models.Employee{Name: "销售C", Email: "p-c@x.com", Dept: "S", Role: "sales", Status: "active"}
	db.Create(&sales)

	// 有进行中商单：禁止释放
	withDeal := models.Customer{Code: "KH-P4", Name: "有商单", OwnerID: sales.ID}
	db.Create(&withDeal)
	db.Create(&models.Deal{Code: "SD-P1", CustomerID: withDeal.ID, Title: "在途",
		Status: models.DealNegotiating, OwnerID: sales.ID})
	if err := svc.Release(ctx, withDeal.ID, sales.ID, ""); !errors.Is(err, ErrReleaseHasDeal) {
		t.Fatalf("有进行中商单应返回 ErrReleaseHasDeal，实际 %v", err)
	}

	// 商单已丢单：可以释放
	db.Model(&models.Deal{}).Where("customer_id = ?", withDeal.ID).
		Update("status", models.DealLost)
	if err := svc.Release(ctx, withDeal.ID, sales.ID, ""); err != nil {
		t.Fatalf("商单已关闭应可释放: %v", err)
	}

	// 有有效合同：禁止释放
	withContract := models.Customer{Code: "KH-P5", Name: "有合同", OwnerID: sales.ID}
	db.Create(&withContract)
	db.Create(&models.Contract{Code: "HT-P1", CustomerID: withContract.ID, Title: "执行中",
		Status: models.ContractPerforming, OwnerID: sales.ID})
	if err := svc.Release(ctx, withContract.ID, sales.ID, ""); !errors.Is(err, ErrReleaseHasContra) {
		t.Fatalf("有有效合同应返回 ErrReleaseHasContra，实际 %v", err)
	}

	// 合同已终止：可以释放
	db.Model(&models.Contract{}).Where("customer_id = ?", withContract.ID).
		Update("status", models.ContractTerminated)
	if err := svc.Release(ctx, withContract.ID, sales.ID, ""); err != nil {
		t.Fatalf("合同已终止应可释放: %v", err)
	}
}

func TestPoolRecycleRules(t *testing.T) {
	db := setupPoolDB(t)
	svc := NewPoolService(db)
	ctx := context.Background()

	sales := models.Employee{Name: "销售D", Email: "p-d@x.com", Dept: "S", Role: "sales", Status: "active"}
	db.Create(&sales)

	if _, err := svc.UpdateSettings(ctx, PoolSettingsInput{
		Enabled: true, MaxClaimPerSales: 0, IdleDaysNoFollow: 30, IdleDaysNoDeal: 0, ProtectDays: 7,
	}); err != nil {
		t.Fatal(err)
	}

	mk := func(code, name string, claimedAgo, followedAgo int) *models.Customer {
		c := models.Customer{Code: code, Name: name, OwnerID: sales.ID}
		db.Create(&c)
		setClaimed(t, db, c.ID, claimedAgo, followedAgo)
		return &c
	}

	stale := mk("KH-R1", "超期未跟进", 60, 45)  // 应回收
	fresh := mk("KH-R2", "近期有跟进", 60, 3)   // 不回收（跟进新）
	protected := mk("KH-R3", "保护期内", 3, 3)  // 不回收（保护期）
	hasDeal := mk("KH-R4", "有在途商单", 60, 45) // 不回收（在途商单豁免）
	db.Create(&models.Deal{Code: "SD-R1", CustomerID: hasDeal.ID, Title: "在途",
		Status: models.DealProposal, OwnerID: sales.ID})
	hasContract := mk("KH-R5", "有有效合同", 60, 45) // 不回收（成交客户豁免）
	db.Create(&models.Contract{Code: "HT-R1", CustomerID: hasContract.ID, Title: "履约",
		Status: models.ContractPerforming, OwnerID: sales.ID})

	// dry run 不落库
	res, err := svc.Recycle(ctx, time.Now(), true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || res.Items[0].CustomerID != stale.ID {
		t.Fatalf("dry run 应命中 1 个（超期未跟进），实际 %d 个: %+v", res.Total, res.Items)
	}
	var check models.Customer
	db.Take(&check, stale.ID)
	if check.OwnerID != sales.ID {
		t.Fatal("dry run 不应真正回收")
	}

	// 实际回收
	res, err = svc.Recycle(ctx, time.Now(), false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 {
		t.Fatalf("期望回收 1 个，实际 %d", res.Total)
	}
	db.Take(&check, stale.ID)
	if check.OwnerID != 0 {
		t.Fatal("超期客户应被回收到公海")
	}
	if check.PoolReason != models.PoolReasonNoFollow {
		t.Fatalf("回收原因应为「%s」，实际「%s」", models.PoolReasonNoFollow, check.PoolReason)
	}

	// 其余四个都应保持原主
	for _, c := range []*models.Customer{fresh, protected, hasDeal, hasContract} {
		var got models.Customer
		db.Take(&got, c.ID)
		if got.OwnerID != sales.ID {
			t.Fatalf("客户「%s」不应被回收", c.Name)
		}
	}

	// 回收流水已落地
	logs, _ := svc.Logs(ctx, stale.ID)
	if len(logs) != 1 || logs[0].Action != models.PoolActionRecycle {
		t.Fatalf("应有 1 条回收流水，实际 %+v", logs)
	}
}

func TestPoolRecycleNoDealRule(t *testing.T) {
	db := setupPoolDB(t)
	svc := NewPoolService(db)
	ctx := context.Background()

	sales := models.Employee{Name: "销售E", Email: "p-e@x.com", Dept: "S", Role: "sales", Status: "active"}
	db.Create(&sales)

	// 只启用「领取后长期未建商单」规则
	if _, err := svc.UpdateSettings(ctx, PoolSettingsInput{
		Enabled: true, IdleDaysNoFollow: 0, IdleDaysNoDeal: 30, ProtectDays: 7,
	}); err != nil {
		t.Fatal(err)
	}

	noDeal := models.Customer{Code: "KH-N1", Name: "领取久未建单", OwnerID: sales.ID}
	db.Create(&noDeal)
	setClaimed(t, db, noDeal.ID, 60, 1) // 跟进很新，但一直没建商单

	res, err := svc.Recycle(ctx, time.Now(), false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || res.Items[0].Reason != models.PoolReasonNoDeal {
		t.Fatalf("应按「未建商单」回收 1 个，实际 %+v", res.Items)
	}
}

func TestPoolRecycleDisabledRules(t *testing.T) {
	db := setupPoolDB(t)
	svc := NewPoolService(db)
	ctx := context.Background()

	sales := models.Employee{Name: "销售F", Email: "p-f@x.com", Dept: "S", Role: "sales", Status: "active"}
	db.Create(&sales)
	c := models.Customer{Code: "KH-D1", Name: "长期未跟进", OwnerID: sales.ID}
	db.Create(&c)
	setClaimed(t, db, c.ID, 999, 999)

	// 两条规则都关闭 → 不回收任何客户
	if _, err := svc.UpdateSettings(ctx, PoolSettingsInput{
		Enabled: true, IdleDaysNoFollow: 0, IdleDaysNoDeal: 0, ProtectDays: 0,
	}); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Recycle(ctx, time.Now(), false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 0 {
		t.Fatalf("规则全关时不应回收，实际回收 %d", res.Total)
	}
}

func TestPoolSettingsPersist(t *testing.T) {
	db := setupPoolDB(t)
	svc := NewPoolService(db)
	ctx := context.Background()

	// 无数据时返回默认值
	st, err := svc.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.MaxClaimPerSales != 50 || st.IdleDaysNoFollow != 30 || st.ProtectDays != 7 {
		t.Fatalf("默认规则不符: %+v", st)
	}

	if _, err := svc.UpdateSettings(ctx, PoolSettingsInput{
		Enabled: true, MaxClaimPerSales: 20, IdleDaysNoFollow: 15, IdleDaysNoDeal: 45, ProtectDays: 3,
	}); err != nil {
		t.Fatal(err)
	}
	st, _ = svc.Settings(ctx)
	if !st.Enabled || st.MaxClaimPerSales != 20 || st.IdleDaysNoFollow != 15 || st.ProtectDays != 3 {
		t.Fatalf("规则未正确持久化: %+v", st)
	}

	// 负数应被拒绝
	if _, err := svc.UpdateSettings(ctx, PoolSettingsInput{ProtectDays: -1}); err == nil {
		t.Fatal("负数天数应被拒绝")
	}
}

func TestPoolAssign(t *testing.T) {
	db := setupPoolDB(t)
	svc := NewPoolService(db)
	ctx := context.Background()

	sales := models.Employee{Name: "销售G", Email: "p-g@x.com", Dept: "S", Role: "sales", Status: "active"}
	off := models.Employee{Name: "已停用", Email: "p-off@x.com", Dept: "S", Role: "sales", Status: "disabled"}
	db.Create(&sales)
	db.Create(&off)
	c := newPoolCustomer(t, db, "KH-A1", "待指派")

	if err := svc.Assign(ctx, c.ID, off.ID, 1); err == nil {
		t.Fatal("指派给停用员工应失败")
	}
	if err := svc.Assign(ctx, c.ID, sales.ID, 1); err != nil {
		t.Fatalf("指派失败: %v", err)
	}
	var got models.Customer
	db.Take(&got, c.ID)
	if got.OwnerID != sales.ID {
		t.Fatalf("指派后 owner 应为 %d，实际 %d", sales.ID, got.OwnerID)
	}
	logs, _ := svc.Logs(ctx, c.ID)
	if len(logs) != 1 || logs[0].Action != models.PoolActionAssign {
		t.Fatalf("应有 1 条指派流水，实际 %+v", logs)
	}
}

// TestOffboardToPool 离职不指定交接人：客户退公海；有商单/合同时必须指定交接人
func TestOffboardToPool(t *testing.T) {
	db := setupPoolDB(t)
	svc := NewEmployeeService(db)
	ctx := context.Background()

	leaver := models.Employee{Name: "离职者", Email: "p-leave@x.com", Dept: "S", Role: "sales", Status: "active"}
	db.Create(&leaver)
	cust := models.Customer{Code: "KH-OB1", Name: "离职者客户", OwnerID: leaver.ID}
	db.Create(&cust)

	// 仅有客户：可退公海
	res, err := svc.Offboard(ctx, leaver.ID, 0, 999)
	if err != nil {
		t.Fatalf("退公海离职应成功: %v", err)
	}
	if res.Customers != 1 {
		t.Fatalf("应转移 1 个客户，实际 %d", res.Customers)
	}
	var got models.Customer
	db.Take(&got, cust.ID)
	if got.OwnerID != 0 || got.PoolReason != models.PoolReasonOffboard {
		t.Fatalf("客户应退回公海并标记离职原因，实际 owner=%d reason=%s", got.OwnerID, got.PoolReason)
	}
	var emp models.Employee
	db.Take(&emp, leaver.ID)
	if emp.Status != "disabled" {
		t.Fatal("离职后员工应被停用")
	}

	// 有商单的员工：不允许仅退公海
	leaver2 := models.Employee{Name: "离职者2", Email: "p-leave2@x.com", Dept: "S", Role: "sales", Status: "active"}
	db.Create(&leaver2)
	c2 := models.Customer{Code: "KH-OB2", Name: "客户2", OwnerID: leaver2.ID}
	db.Create(&c2)
	db.Create(&models.Deal{Code: "SD-OB1", CustomerID: c2.ID, Title: "在途",
		Status: models.DealProposal, OwnerID: leaver2.ID})
	if _, err := svc.Offboard(ctx, leaver2.ID, 0, 999); !errors.Is(err, ErrSuccessorRequired) {
		t.Fatalf("有商单时应返回 ErrSuccessorRequired，实际 %v", err)
	}
}
