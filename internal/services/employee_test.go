package services

import (
	"context"
	"strings"
	"testing"

	"bss/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupEmpDB(t *testing.T) *gorm.DB {
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
	if err := db.AutoMigrate(&models.Employee{}, &models.Dict{}); err != nil {
		t.Fatal(err)
	}
	// 复刻生产迁移里的局部唯一索引 (type,value) WHERE deleted_at IS NULL，
	// AutoMigrate 不会自动创建，测试需手动补齐以校验 AddDict 去重逻辑
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uq_dicts_type_value ON dicts(type, value) WHERE deleted_at IS NULL").Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func TestEmployeeCreate(t *testing.T) {
	svc := NewEmployeeService(setupEmpDB(t))
	ctx := context.Background()

	// 成功：默认 active + 强制改密 + 初始密码哈希
	emp, err := svc.Create(ctx, EmployeeInput{Name: "张三", Role: models.RoleSales, Dept: "销售"}, "zhang@bss.local")
	if err != nil {
		t.Fatalf("创建员工失败: %v", err)
	}
	if emp.ID == 0 || !emp.MustChangePwd || emp.Status != "active" {
		t.Fatalf("创建结果不符合预期: %+v", emp)
	}

	// 邮箱不区分大小写重复 → ErrEmailExists
	if _, err := svc.Create(ctx, EmployeeInput{Name: "李四", Role: models.RoleSales}, "ZHANG@BSS.LOCAL"); err != ErrEmailExists {
		t.Fatalf("重复邮箱应返回 ErrEmailExists，得到 %v", err)
	}

	// 非法角色 → ErrInvalidRole
	if _, err := svc.Create(ctx, EmployeeInput{Name: "王五", Role: "superuser"}, "w@bss.local"); err != ErrInvalidRole {
		t.Fatalf("非法角色应返回 ErrInvalidRole，得到 %v", err)
	}

	// 空姓名 → 报错
	if _, err := svc.Create(ctx, EmployeeInput{Name: "", Role: models.RoleSales}, "x@bss.local"); err == nil {
		t.Fatalf("空姓名应报错")
	}
}

func TestEmployeeUpdateDowngradeLastAdmin(t *testing.T) {
	db := setupEmpDB(t)
	svc := NewEmployeeService(db)
	ctx := context.Background()

	admin, err := svc.Create(ctx, EmployeeInput{Name: "唯一管理员", Role: models.RoleAdmin}, "admin@bss.local")
	if err != nil {
		t.Fatal(err)
	}

	// 降级唯一管理员应被拒绝
	if err := svc.Update(ctx, admin.ID, EmployeeInput{Name: "唯一管理员", Role: models.RoleSales}); err != ErrLastAdmin {
		t.Fatalf("降级最后管理员应返回 ErrLastAdmin，得到 %v", err)
	}

	// 存在另一管理员时可降级
	if _, err := svc.Create(ctx, EmployeeInput{Name: "另一管理员", Role: models.RoleAdmin}, "admin2@bss.local"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Update(ctx, admin.ID, EmployeeInput{Name: "唯一管理员", Role: models.RoleSales}); err != nil {
		t.Fatalf("存在其他管理员时应可降级，得到 %v", err)
	}
}

func TestEmployeeSetStatusGuards(t *testing.T) {
	db := setupEmpDB(t)
	svc := NewEmployeeService(db)
	ctx := context.Background()

	a, _ := svc.Create(ctx, EmployeeInput{Name: "管理员A", Role: models.RoleAdmin}, "a@bss.local")
	b, _ := svc.Create(ctx, EmployeeInput{Name: "管理员B", Role: models.RoleAdmin}, "b@bss.local")

	// 不能停用自己
	if err := svc.SetStatus(ctx, a.ID, a.ID, false); err != ErrCannotSelfOp {
		t.Fatalf("停用自己应返回 ErrCannotSelfOp，得到 %v", err)
	}

	// 停用管理员B（A 仍是 active admin）→ 成功
	if err := svc.SetStatus(ctx, b.ID, a.ID, false); err != nil {
		t.Fatalf("停用非最后管理员应成功，得到 %v", err)
	}

	// 此时只剩 A 一个 active admin，停用 A 应被拒
	if err := svc.SetStatus(ctx, a.ID, b.ID, false); err != ErrLastAdmin {
		t.Fatalf("停用最后管理员应返回 ErrLastAdmin，得到 %v", err)
	}
}

func TestEmployeeResetPassword(t *testing.T) {
	svc := NewEmployeeService(setupEmpDB(t))
	ctx := context.Background()
	emp, _ := svc.Create(ctx, EmployeeInput{Name: "张三", Role: models.RoleSales}, "z@bss.local")

	// 重置后强制改密
	if err := svc.ResetPassword(ctx, emp.ID); err != nil {
		t.Fatalf("重置密码失败: %v", err)
	}
	var got models.Employee
	if err := svc.db.WithContext(ctx).Take(&got, emp.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !got.MustChangePwd {
		t.Fatalf("重置后应强制改密")
	}
}

func TestEmployeeDict(t *testing.T) {
	svc := NewEmployeeService(setupEmpDB(t))
	ctx := context.Background()

	// 新增 + 查重
	d, err := svc.AddDict(ctx, "dept", "研发")
	if err != nil {
		t.Fatalf("新增字典失败: %v", err)
	}
	if _, err := svc.AddDict(ctx, "dept", "研发"); err == nil || !strings.Contains(err.Error(), "已存在") {
		t.Fatalf("重复字典项应报错，得到 %v", err)
	}
	list, err := svc.ListDict(ctx, "dept")
	if err != nil || len(list) != 1 {
		t.Fatalf("ListDict 异常: list=%v err=%v", list, err)
	}

	// 部门被员工占用时禁止删除
	_, _ = svc.Create(ctx, EmployeeInput{Name: "占用者", Role: models.RoleSales, Dept: "研发"}, "occ@bss.local")
	if err := svc.RemoveDict(ctx, d.ID); err == nil || !strings.Contains(err.Error(), "仍有") {
		t.Fatalf("删除被占用部门应报错，得到 %v", err)
	}
}
