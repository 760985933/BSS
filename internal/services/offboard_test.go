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

func setupOffboardDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "ob.db")), &gorm.Config{
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Employee{}, &models.Customer{}, &models.Deal{},
		&models.Contract{}, &models.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestOffboardTransferAndDisable(t *testing.T) {
	db := setupOffboardDB(t)
	svc := NewEmployeeService(db)
	ctx := context.Background()

	a := models.Employee{Name: "甲", Email: "ob-a@x.com", Dept: "S", Role: "sales", Status: "active"}
	b := models.Employee{Name: "乙", Email: "ob-b@x.com", Dept: "S", Role: "sales", Status: "active"}
	if err := db.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&b).Error; err != nil {
		t.Fatal(err)
	}
	// 甲名下数据
	db.Create(&models.Customer{Code: "KH-O", Name: "c", OwnerID: a.ID})
	db.Create(&models.Deal{Code: "SD-O", CustomerID: 1, Title: "d", AmountCent: 100, Status: models.DealWon, OwnerID: a.ID})
	db.Create(&models.Contract{Code: "HT-O", CustomerID: 1, Title: "ct", AmountCent: 100, Status: models.ContractSigned, OwnerID: a.ID})

	// 预览
	prev, err := svc.OffboardPreview(ctx, a.ID)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if !prev.HasData || prev.Customers != 1 || prev.Deals != 1 || prev.Contracts != 1 {
		t.Fatalf("预览数据不符: %+v", prev)
	}

	// 交接给乙（operator 为其他管理员，非本人）
	res, err := svc.Offboard(ctx, a.ID, b.ID, 100)
	if err != nil {
		t.Fatalf("offboard: %v", err)
	}
	if res.Customers != 1 || res.Deals != 1 || res.Contracts != 1 {
		t.Fatalf("交接结果不符: %+v", res)
	}

	// 甲已停用
	var ea models.Employee
	db.Take(&ea, a.ID)
	if ea.Status != "disabled" {
		t.Fatalf("甲应已停用, got %s", ea.Status)
	}
	// 数据已转移给乙
	var c models.Customer
	db.Take(&c, 1)
	if c.OwnerID != b.ID {
		t.Fatalf("客户应归属乙, got %d", c.OwnerID)
	}
	var d models.Deal
	db.Take(&d, 1)
	if d.OwnerID != b.ID {
		t.Fatalf("商单应归属乙, got %d", d.OwnerID)
	}
	var ct models.Contract
	db.Take(&ct, 1)
	if ct.OwnerID != b.ID {
		t.Fatalf("合同应归属乙, got %d", ct.OwnerID)
	}

	// 审计写入
	var cnt int64
	db.Model(&models.AuditLog{}).Where("entity_type = 'employee' AND entity_id = ? AND action = 'offboard'", a.ID).Count(&cnt)
	if cnt != 1 {
		t.Fatalf("应写入 1 条 offboard 审计, got %d", cnt)
	}
}

func TestOffboardGuards(t *testing.T) {
	db := setupOffboardDB(t)
	svc := NewEmployeeService(db)
	ctx := context.Background()
	a := models.Employee{Name: "甲", Email: "g-a@x.com", Dept: "S", Role: "sales", Status: "active"}
	b := models.Employee{Name: "乙", Email: "g-b@x.com", Dept: "S", Role: "sales", Status: "active"}
	db.Create(&a)
	db.Create(&b)

	// 交接人不能是本人
	if _, err := svc.Offboard(ctx, a.ID, a.ID, 1); err != ErrOffboardSameEmployee {
		t.Fatalf("交接人=本人应 ErrOffboardSameEmployee, got %v", err)
	}
	// 交接人不存在
	if _, err := svc.Offboard(ctx, a.ID, 9999, 1); err != ErrSuccessorMissing {
		t.Fatalf("交接人不存在应 ErrSuccessorMissing, got %v", err)
	}
	// 交接人停用 → 不可作为交接人
	db.Model(&b).Update("status", "disabled")
	if _, err := svc.Offboard(ctx, a.ID, b.ID, 1); err != ErrSuccessorNotActive {
		t.Fatalf("停用员工作为交接人应 ErrSuccessorNotActive, got %v", err)
	}
}

func TestAuditQueryFilters(t *testing.T) {
	db := setupOffboardDB(t)
	audSvc := NewAuditQueryService(db)
	ctx := context.Background()

	// 注入若干审计日志
	db.Create(&models.AuditLog{EntityType: "contract", EntityID: 10, Action: "create", OperatorID: 1, CreatedAt: time.Now().Add(-48 * time.Hour)})
	db.Create(&models.AuditLog{EntityType: "contract", EntityID: 10, Action: "update", OperatorID: 2, CreatedAt: time.Now()})
	db.Create(&models.AuditLog{EntityType: "deal", EntityID: 20, Action: "offboard", OperatorID: 1, CreatedAt: time.Now()})

	// 按 entity 过滤
	list, total, err := audSvc.List(ctx, AuditQuery{EntityType: "contract", Page: 1, Size: 20})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 {
		t.Fatalf("contract 审计应有 2 条, got %d", total)
	}
	if len(list) != 2 {
		t.Fatalf("返回条数不符: %d", len(list))
	}

	// 按 action 过滤
	list, total, err = audSvc.List(ctx, AuditQuery{Action: "offboard", Page: 1, Size: 20})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 {
		t.Fatalf("offboard 审计应有 1 条, got %d", total)
	}

	// 按 operator 过滤
	list, total, err = audSvc.List(ctx, AuditQuery{OperatorID: 2, Page: 1, Size: 20})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 {
		t.Fatalf("operator=2 审计应有 1 条, got %d", total)
	}
}
