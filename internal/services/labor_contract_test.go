package services

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"bss/internal/models"
	"bss/internal/pkg/code"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupHRDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "hr.db")), &gorm.Config{
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Employee{}, &models.Candidate{},
		&models.LaborContract{}, &models.Onboarding{}, &models.Notification{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS code_counters (
		prefix TEXT NOT NULL, year INTEGER NOT NULL, seq INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (prefix, year))`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func ptrT(t time.Time) *time.Time { return &t }

func hrMustEmployee(t *testing.T, db *gorm.DB, name string) uint {
	t.Helper()
	e := models.Employee{Name: name, Email: name + "@x.com", Role: "employee", Status: "active"}
	if err := db.Create(&e).Error; err != nil {
		t.Fatal(err)
	}
	return e.ID
}

func TestCreateLaborContractGeneratesCode(t *testing.T) {
	db := setupHRDB(t)
	gen := code.NewGenerator(db)
	ctx := context.Background()
	empID := hrMustEmployee(t, db, "张三")
	lc, err := CreateLaborContract(ctx, db, gen, LaborContractInput{
		EmployeeID: empID, Type: models.LCTypeFixed, ProbationMonths: 3,
	}, 1)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if lc.Code == "" || len(lc.Code) < 4 {
		t.Fatalf("编号生成异常: %q", lc.Code)
	}
	if lc.Status != models.LCStatusDraft {
		t.Fatalf("默认应为 draft, got %s", lc.Status)
	}
	if lc.Employee == nil || lc.Employee.Name != "张三" {
		t.Fatal("应预加载员工姓名")
	}
}

func TestLaborContractStateMachine(t *testing.T) {
	db := setupHRDB(t)
	gen := code.NewGenerator(db)
	ctx := context.Background()
	empID := hrMustEmployee(t, db, "员工甲")
	lc, _ := CreateLaborContract(ctx, db, gen, LaborContractInput{EmployeeID: empID}, 1)

	// draft -> active 直接过
	if _, err := TransitionLaborContract(ctx, db, lc.ID, models.LCStatusActive, "", false); err != nil {
		t.Fatalf("draft->active 应成功: %v", err)
	}
	// active -> terminated 无原因：拒绝
	if _, err := TransitionLaborContract(ctx, db, lc.ID, models.LCStatusTerminated, "", false); !errors.Is(err, ErrLCTerminateReasonRequired) {
		t.Fatalf("terminated 需原因, got err=%v", err)
	}
	// active -> terminated 带原因：成功
	if _, err := TransitionLaborContract(ctx, db, lc.ID, models.LCStatusTerminated, "协商一致解除", false); err != nil {
		t.Fatalf("带原因应成功: %v", err)
	}
	// 终态再流转：拒绝
	if _, err := TransitionLaborContract(ctx, db, lc.ID, models.LCStatusActive, "", false); !errors.Is(err, ErrLCStatusTerminal) {
		t.Fatalf("终态应锁定, got err=%v", err)
	}
}

func TestLaborContractBacktrackNeedsForce(t *testing.T) {
	db := setupHRDB(t)
	gen := code.NewGenerator(db)
	ctx := context.Background()
	empID := hrMustEmployee(t, db, "员工乙")
	lc, _ := CreateLaborContract(ctx, db, gen, LaborContractInput{EmployeeID: empID}, 1)
	TransitionLaborContract(ctx, db, lc.ID, models.LCStatusActive, "", false)

	// active -> draft 无 force：软校验拦截
	if _, err := TransitionLaborContract(ctx, db, lc.ID, models.LCStatusDraft, "", false); !errors.Is(err, ErrLCTransitionForceReq) {
		t.Fatalf("回退需 force, got err=%v", err)
	}
	// active -> draft 带 force：成功
	if _, err := TransitionLaborContract(ctx, db, lc.ID, models.LCStatusDraft, "", true); err != nil {
		t.Fatalf("带 force 回退应成功: %v", err)
	}
	// 非法流转 active(已是draft)->foo：拒绝
	if _, err := TransitionLaborContract(ctx, db, lc.ID, "foo", "", false); !errors.Is(err, ErrLCInvalidTransition) {
		t.Fatalf("非法流转应拒绝, got err=%v", err)
	}
}

func TestUpdateTerminatedContractLocked(t *testing.T) {
	db := setupHRDB(t)
	gen := code.NewGenerator(db)
	ctx := context.Background()
	empID := hrMustEmployee(t, db, "员工丙")
	lc, _ := CreateLaborContract(ctx, db, gen, LaborContractInput{EmployeeID: empID}, 1)
	TransitionLaborContract(ctx, db, lc.ID, models.LCStatusActive, "", false)
	TransitionLaborContract(ctx, db, lc.ID, models.LCStatusTerminated, "原因", false)
	// 终态更新核心字段：锁定
	if _, err := UpdateLaborContract(ctx, db, lc.ID, LaborContractInput{EmployeeID: empID, Type: models.LCTypeParttime}); !errors.Is(err, ErrLCStatusTerminal) {
		t.Fatalf("终态应锁定更新, got err=%v", err)
	}
}

func TestOnboardingStepProgress(t *testing.T) {
	db := setupHRDB(t)
	gen := code.NewGenerator(db)
	ctx := context.Background()
	empID := hrMustEmployee(t, db, "新人丁")
	ob, err := CreateOnboarding(ctx, db, gen, OnboardingInput{EmployeeID: empID}, 1)
	if err != nil {
		t.Fatalf("create onboarding: %v", err)
	}
	if ob.Status != models.OBStatusInProgress {
		t.Fatalf("默认 in_progress, got %s", ob.Status)
	}
	// 四步全 done -> completed
	ob, err = UpdateOnboarding(ctx, db, ob.ID, OnboardingInput{
		EmployeeID: empID, StepProfile: models.OBStepDone, StepEquip: models.OBStepDone,
		StepTraining: models.OBStepDone, StepProbation: models.OBStepDone,
	})
	if err != nil {
		t.Fatalf("update onboarding: %v", err)
	}
	if ob.Status != models.OBStatusCompleted {
		t.Fatalf("应 completed, got %s", ob.Status)
	}
	// 关联不存在候选人：报错
	if _, err := CreateOnboarding(ctx, db, gen, OnboardingInput{EmployeeID: empID, CandidateID: uip(99999)}, 1); !errors.Is(err, ErrCandidateMissing) {
		t.Fatalf("关联不存在候选人应报错, got err=%v", err)
	}
}

func TestScanLaborContractReminders(t *testing.T) {
	db := setupHRDB(t)
	gen := code.NewGenerator(db)
	ctx := context.Background()
	empID := hrMustEmployee(t, db, "员工戊")
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// 到期提醒：active + 25 天后到期
	lcExp, _ := CreateLaborContract(ctx, db, gen, LaborContractInput{EmployeeID: empID, ProbationMonths: 0}, 1)
	TransitionLaborContract(ctx, db, lcExp.ID, models.LCStatusActive, "", false)
	db.Model(&models.LaborContract{}).Where("id = ?", lcExp.ID).
		Updates(map[string]any{"end_date": ptrT(now.AddDate(0, 0, 25)), "owner_id": 1})
	// 转正提醒：active + start 使得转正日落在 7 天窗口内
	lcProb, _ := CreateLaborContract(ctx, db, gen, LaborContractInput{EmployeeID: empID, ProbationMonths: 1}, 1)
	TransitionLaborContract(ctx, db, lcProb.ID, models.LCStatusActive, "", false)
	db.Model(&models.LaborContract{}).Where("id = ?", lcProb.ID).
		Updates(map[string]any{"start_date": ptrT(now.AddDate(0, 0, -25)), "owner_id": 1})

	created, err := ScanLaborContractReminders(ctx, db, now)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(created) != 2 {
		t.Fatalf("应生成 2 条提醒(到期+转正), got %d", len(created))
	}
	// 去重：再次扫描不重复生成
	again, err := ScanLaborContractReminders(ctx, db, now)
	if err != nil {
		t.Fatalf("scan again: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("去重后应 0 条, got %d", len(again))
	}
}
