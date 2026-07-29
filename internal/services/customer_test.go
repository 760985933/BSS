package services

import (
	"context"
	"testing"

	"bss/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// 回归测试：Count 查询错误必须向上传播（修复前 DB 故障时 cnt=0 会错误放行）
// 场景：关闭底层连接模拟 DB 故障，Create 不得继续、Delete 不得误删有下游数据的客户

func brokenDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil { // 强制后续所有查询失败
		t.Fatal(err)
	}
	return db
}

func TestCreatePropagatesDBError(t *testing.T) {
	svc := NewCustomerService(brokenDB(t))
	_, err := svc.Create(context.Background(), CustomerInput{Name: "故障注入客户"}, 1)
	if err == nil {
		t.Fatal("DB 故障时 Create 应返回错误，却放行了（Count 错误被吞）")
	}
}

func TestUpdatePropagatesDBError(t *testing.T) {
	svc := NewCustomerService(brokenDB(t))
	if err := svc.Update(context.Background(), 1, CustomerInput{Name: "故障注入"}); err == nil {
		t.Fatal("DB 故障时 Update 应返回错误，却放行了")
	}
}

func TestTransferPropagatesDBError(t *testing.T) {
	svc := NewCustomerService(brokenDB(t))
	if err := svc.Transfer(context.Background(), 1, 2); err == nil {
		t.Fatal("DB 故障时 Transfer 应返回错误，却放行了")
	}
}

func TestDeletePropagatesDBError(t *testing.T) {
	svc := NewCustomerService(brokenDB(t))
	if err := svc.Delete(context.Background(), 1); err == nil {
		t.Fatal("DB 故障时 Delete 应返回错误，却放行了（下游检查被吞会导致误删）")
	}
}

// 正常路径不受影响：健康库上 Count 逻辑依旧工作
func TestCreateHealthyPath(t *testing.T) {
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
		`CREATE TABLE code_counters (prefix TEXT NOT NULL, year INTEGER NOT NULL, seq INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (prefix, year))`,
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatal(err)
		}
	}
	svc := NewCustomerService(db)
	ctx := context.Background()

	c1, err := svc.Create(ctx, CustomerInput{Name: "健康客户"}, 1)
	if err != nil {
		t.Fatalf("正常创建失败: %v", err)
	}
	if c1.Code == "" {
		t.Error("未生成 KH 单号")
	}
	// 重名必须 409 语义
	if _, err := svc.Create(ctx, CustomerInput{Name: "健康客户"}, 1); err != ErrCustomerNameExists {
		t.Errorf("重名校验失效: %v", err)
	}
	var _ = models.Customer{}
}
