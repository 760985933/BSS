package services

import (
	"context"
	"testing"

	"bss/internal/middleware"
	"bss/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// 造数：3 部门 4 员工 + 3 个客户（owner 各异），验证 ScopeOwner 行级过滤
func setupScopeDB(t *testing.T) (*gorm.DB, map[string]uint) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	for _, ddl := range []string{
		`CREATE TABLE employees (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, email TEXT, phone TEXT DEFAULT '', dept TEXT DEFAULT '', position TEXT DEFAULT '', role TEXT, password_hash TEXT DEFAULT '', must_change_pwd INTEGER DEFAULT 0, status TEXT DEFAULT 'active', created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE customers (id INTEGER PRIMARY KEY AUTOINCREMENT, code TEXT, name TEXT, industry TEXT DEFAULT '', source TEXT DEFAULT '', level TEXT DEFAULT '', owner_id INTEGER, remark TEXT DEFAULT '', last_followed_at DATETIME, claimed_at DATETIME, pool_reason TEXT NOT NULL DEFAULT '', created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatal(err)
		}
	}

	ids := map[string]uint{}
	emps := []models.Employee{
		{Name: "管理员", Email: "a@t.com", Role: models.RoleAdmin, Dept: "总经办"},
		{Name: "销售甲", Email: "s1@t.com", Role: models.RoleSales, Dept: "销售一部"},
		{Name: "主管乙", Email: "l1@t.com", Role: models.RoleSalesLead, Dept: "销售一部"},
		{Name: "销售丙", Email: "s2@t.com", Role: models.RoleSales, Dept: "销售二部"},
	}
	for i := range emps {
		if err := db.Create(&emps[i]).Error; err != nil {
			t.Fatal(err)
		}
		ids[emps[i].Email] = emps[i].ID
	}
	customers := []models.Customer{
		{Code: "KH-1", Name: "甲的客户", OwnerID: ids["s1@t.com"]},
		{Code: "KH-2", Name: "乙的客户", OwnerID: ids["l1@t.com"]},
		{Code: "KH-3", Name: "丙的客户", OwnerID: ids["s2@t.com"]},
	}
	for i := range customers {
		if err := db.Create(&customers[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db, ids
}

func injectClaims(userID uint, role, dept string) context.Context {
	return middleware.WithClaims(context.Background(),
		&middleware.Claims{UserID: userID, Role: role, Dept: dept})
}

func countCustomers(t *testing.T, db *gorm.DB, ctx context.Context) int64 {
	t.Helper()
	var cnt int64
	if err := ScopeOwner(db.WithContext(ctx).Model(&models.Customer{}), ctx).Count(&cnt).Error; err != nil {
		t.Fatal(err)
	}
	return cnt
}

func TestScopeOwner(t *testing.T) {
	db, ids := setupScopeDB(t)

	cases := []struct {
		name    string
		userID  uint
		role    string
		dept    string
		wantCnt int64
	}{
		{"admin 看全部", ids["a@t.com"], models.RoleAdmin, "总经办", 3},
		{"销售只看本人", ids["s1@t.com"], models.RoleSales, "销售一部", 1},
		{"主管看本部门", ids["l1@t.com"], models.RoleSalesLead, "销售一部", 2},
		{"财务看全部", ids["s2@t.com"], models.RoleFinance, "销售二部", 3},
		{"未登录兜底为 0", 999, "", "", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := countCustomers(t, db, injectClaims(c.userID, c.role, c.dept)); got != c.wantCnt {
				t.Errorf("%s: 期望 %d 条，实际 %d 条", c.name, c.wantCnt, got)
			}
		})
	}
}

func TestCanAccessOwner(t *testing.T) {
	db, ids := setupScopeDB(t)

	cases := []struct {
		name    string
		userID  uint
		role    string
		dept    string
		ownerID uint
		want    bool
	}{
		{"销售访问本人", ids["s1@t.com"], models.RoleSales, "销售一部", ids["s1@t.com"], true},
		{"销售访问他人", ids["s1@t.com"], models.RoleSales, "销售一部", ids["s2@t.com"], false},
		{"主管访问本部门", ids["l1@t.com"], models.RoleSalesLead, "销售一部", ids["s1@t.com"], true},
		{"主管访问外部门", ids["l1@t.com"], models.RoleSalesLead, "销售一部", ids["s2@t.com"], false},
		{"admin 访问任意", ids["a@t.com"], models.RoleAdmin, "总经办", ids["s2@t.com"], true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := CanAccessOwner(db, injectClaims(c.userID, c.role, c.dept), c.ownerID)
			if err != nil {
				t.Fatalf("%s: 意外错误 %v", c.name, err)
			}
			if got != c.want {
				t.Errorf("%s: 期望 %v，实际 %v", c.name, c.want, got)
			}
		})
	}
}
