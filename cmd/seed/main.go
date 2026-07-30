// 造数脚本：构建一条可演示 M1 全链路的样例数据。
// 用法：BSS_DATA=/path/to/data go run ./cmd/seed
// 反复执行幂等（按 code/email 去重）。
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"bss/internal/config"
	"bss/internal/db"
	"bss/internal/models"
	"bss/internal/services"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	demoSalesEmail = "demo-sales@bss.local"
	demoPassword   = "Bss@1234"
)

func main() {
	cfg := config.Load()
	gdb, err := db.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	ctx := context.Background()

	// 确保 admin 存在
	authSvc := services.NewAuthService(gdb)
	if err := authSvc.InitAdmin(ctx); err != nil {
		log.Fatalf("admin 初始化失败: %v", err)
	}

	salesID := ensureSales(gdb)
	custID := ensureCustomer(gdb, salesID)
	ensureWonDeal(gdb, custID, salesID)
	ctID := ensureSignedContract(gdb, custID, salesID)
	ensureOverduePlan(gdb, ctID, salesID)
	ensurePartialRecord(gdb, ctID, salesID)

	// 触发提醒扫描，生成到期/逾期通知
	created, err := services.ScanReminders(ctx, gdb, time.Now())
	if err != nil {
		log.Fatalf("提醒扫描失败: %v", err)
	}

	fmt.Println("=== 演示数据已就绪 ===")
	fmt.Printf("数据目录: %s\n", cfg.DataDir)
	fmt.Printf("演示销售账号: %s / %s (角色 sales，可见其名下通知)\n", demoSalesEmail, demoPassword)
	fmt.Printf("管理员账号: admin@bss.local / admin123\n")
	fmt.Printf("本次新生成提醒通知: %d 条\n", created)
	fmt.Println("建议访问「仪表盘」查看 4 卡片 + 3 列表，并以演示销售账号登录查看通知中心。")
}

func ensureSales(gdb *gorm.DB) uint {
	var e models.Employee
	if err := gdb.Where("email = ?", demoSalesEmail).First(&e).Error; err == nil {
		return e.ID
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(demoPassword), bcrypt.DefaultCost)
	e = models.Employee{
		Name: "演示销售", Email: demoSalesEmail, Phone: "13800000000",
		Dept: "销售一部", Position: "销售经理", Role: models.RoleSales,
		PasswordHash: string(hash), MustChangePwd: false, Status: "active",
	}
	if err := gdb.Create(&e).Error; err != nil {
		log.Fatalf("创建演示销售失败: %v", err)
	}
	return e.ID
}

func ensureCustomer(gdb *gorm.DB, owner uint) uint {
	var c models.Customer
	if err := gdb.Where("code = ?", "KH-DEMO").First(&c).Error; err == nil {
		return c.ID
	}
	c = models.Customer{Code: "KH-DEMO", Name: "示例科技有限公司", Industry: "软件", Source: "web", Level: "A", OwnerID: owner, Remark: "种子数据"}
	if err := gdb.Create(&c).Error; err != nil {
		log.Fatalf("创建客户失败: %v", err)
	}
	return c.ID
}

func ensureWonDeal(gdb *gorm.DB, custID, owner uint) uint {
	var d models.Deal
	if err := gdb.Where("code = ?", "SD-DEMO-0001").First(&d).Error; err == nil {
		return d.ID
	}
	d = models.Deal{
		Code: "SD-DEMO-0001", CustomerID: custID, Title: "云CRM采购项目", AmountCent: 200000,
		Probability: 100, ExpectedSignDate: time.Now().Format("2006-01-02"),
		Status: models.DealWon, OwnerID: owner, Remark: "种子数据",
	}
	if err := gdb.Create(&d).Error; err != nil {
		log.Fatalf("创建赢单失败: %v", err)
	}
	return d.ID
}

func ensureSignedContract(gdb *gorm.DB, custID, owner uint) uint {
	var c models.Contract
	if err := gdb.Where("code = ?", "HT-DEMO-0001").First(&c).Error; err == nil {
		return c.ID
	}
	now := time.Now()
	c = models.Contract{
		Code: "HT-DEMO-0001", CustomerID: custID, Title: "云CRM年度服务合同", AmountCent: 200000,
		SignDate: now.Format("2006-01-02"), StartDate: now.Format("2006-01-02"),
		ExpireDate: now.AddDate(0, 0, 20).Format("2006-01-02"),
		Status: models.ContractSigned, OwnerID: owner, Remark: "种子数据",
	}
	if err := gdb.Create(&c).Error; err != nil {
		log.Fatalf("创建合同失败: %v", err)
	}
	return c.ID
}

func ensureOverduePlan(gdb *gorm.DB, ctID, owner uint) {
	var p models.PaymentPlan
	if err := gdb.Where("contract_id = ? AND period_no = ?", ctID, 1).First(&p).Error; err == nil {
		return
	}
	p = models.PaymentPlan{
		ContractID: ctID, PeriodNo: 1,
		DueDate: time.Now().AddDate(0, 0, -10).Format("2006-01-02"),
		AmountCent: 80000, Status: models.PlanPending,
	}
	if err := gdb.Create(&p).Error; err != nil {
		log.Fatalf("创建逾期计划失败: %v", err)
	}
	_ = owner
}

func ensurePartialRecord(gdb *gorm.DB, ctID, owner uint) {
	var p models.PaymentPlan
	if err := gdb.Where("contract_id = ? AND period_no = ?", ctID, 2).First(&p).Error; err != nil {
		p = models.PaymentPlan{ContractID: ctID, PeriodNo: 2,
			DueDate: time.Now().AddDate(0, 1, 10).Format("2006-01-02"),
			AmountCent: 60000, Status: models.PlanPending}
		if err := gdb.Create(&p).Error; err != nil {
			log.Fatalf("创建第二期计划失败: %v", err)
		}
	}
	// 仅当该计划尚无回款时插入一条部分到账（演示本月回款）
	var cnt int64
	gdb.Model(&models.PaymentRecord{}).Where("plan_id = ?", p.ID).Count(&cnt)
	if cnt > 0 {
		return
	}
	rec := models.PaymentRecord{
		ContractID: ctID, PlanID: &p.ID, AmountCent: 30000,
		PaidAt: time.Now().Format("2006-01-02"), Method: "bank", CreatedBy: owner,
	}
	if err := gdb.Create(&rec).Error; err != nil {
		log.Fatalf("创建回款记录失败: %v", err)
	}
}
