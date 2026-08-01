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

func setupAttDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "att.db")), &gorm.Config{
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Employee{}, &models.AttendanceSchedule{},
		&models.LeaveRequest{}, &models.Attendance{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS code_counters (
		prefix TEXT NOT NULL, year INTEGER NOT NULL, seq INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (prefix, year))`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func attMustEmployee(t *testing.T, db *gorm.DB, name string) uint {
	t.Helper()
	e := models.Employee{Name: name, Email: name + "@x.com", Role: "employee", Status: "active"}
	if err := db.Create(&e).Error; err != nil {
		t.Fatal(err)
	}
	return e.ID
}

func TestCreateScheduleValidates(t *testing.T) {
	db := setupAttDB(t)
	ctx := context.Background()
	empID := attMustEmployee(t, db, "排班员")

	// 非法星期
	if _, err := CreateSchedule(ctx, db, ScheduleInput{EmployeeID: empID, Weekday: 8, StartTime: "09:00", EndTime: "18:00"}); !errors.Is(err, ErrScheduleWeekdayInvalid) {
		t.Fatalf("星期越界应报错, got %v", err)
	}
	// 缺员工
	if _, err := CreateSchedule(ctx, db, ScheduleInput{Weekday: 1, StartTime: "09:00", EndTime: "18:00"}); !errors.Is(err, ErrEmployeeMissing) {
		t.Fatalf("缺员工应报错, got %v", err)
	}
	// 正常
	s, err := CreateSchedule(ctx, db, ScheduleInput{EmployeeID: empID, Weekday: 1, StartTime: "09:00", EndTime: "18:00", ShiftType: "night"})
	if err != nil {
		t.Fatalf("创建排班失败: %v", err)
	}
	if s.ShiftType != models.ScheduleNight || s.Employee == nil || s.Employee.Name != "排班员" {
		t.Fatalf("排班字段异常: %+v", s)
	}
	// 列表按员工过滤
	list, _ := ListSchedules(ctx, db, "")
	if len(list) != 1 {
		t.Fatalf("应有 1 条排班, got %d", len(list))
	}
}

func TestLeaveStateMachine(t *testing.T) {
	db := setupAttDB(t)
	gen := code.NewGenerator(db)
	ctx := context.Background()
	empID := attMustEmployee(t, db, "请假员")

	lr, err := CreateLeaveRequest(ctx, db, gen, LeaveRequestInput{
		EmployeeID: empID, Type: models.LeaveSick, StartDate: "2026-08-01", EndDate: "2026-08-03", Reason: "感冒",
	})
	if err != nil {
		t.Fatalf("创建请假失败: %v", err)
	}
	if lr.Code == "" || lr.Status != models.LeavePending {
		t.Fatalf("默认应为 pending, got %s / %s", lr.Code, lr.Status)
	}

	// 审批通过
	lr, err = DecideLeaveRequest(ctx, db, lr.ID, true, 1, "")
	if err != nil {
		t.Fatalf("审批通过失败: %v", err)
	}
	if lr.Status != models.LeaveApproved || lr.ApproverID == nil || *lr.ApproverID != 1 {
		t.Fatalf("审批应 approved, got %s approver=%d", lr.Status, lr.ApproverID)
	}

	// 重复审批：拒绝
	if _, err := DecideLeaveRequest(ctx, db, lr.ID, false, 1, "x"); !errors.Is(err, ErrLeaveAlreadyDecided) {
		t.Fatalf("已审批应冲突, got %v", err)
	}
}

func TestLeaveRejectStoresReason(t *testing.T) {
	db := setupAttDB(t)
	gen := code.NewGenerator(db)
	ctx := context.Background()
	empID := attMustEmployee(t, db, "请假员2")

	lr, _ := CreateLeaveRequest(ctx, db, gen, LeaveRequestInput{
		EmployeeID: empID, Type: models.LeaveAnnual, StartDate: "2026-08-10", EndDate: "2026-08-15",
	})
	lr, err := DecideLeaveRequest(ctx, db, lr.ID, false, 1, "与项目排期冲突")
	if err != nil {
		t.Fatalf("驳回失败: %v", err)
	}
	if lr.Status != models.LeaveRejected || lr.RejectReason != "与项目排期冲突" {
		t.Fatalf("驳回状态/原因异常: %+v", lr)
	}
}

func TestUpsertAttendanceSameDay(t *testing.T) {
	db := setupAttDB(t)
	ctx := context.Background()
	empID := attMustEmployee(t, db, "考勤员")

	a, err := UpsertAttendance(ctx, db, AttendanceInput{EmployeeID: empID, Date: "2026-08-01", Status: models.AttNormal})
	if err != nil {
		t.Fatalf("登记失败: %v", err)
	}
	// 同日更新（迟到）应覆盖而非新增
	a, err = UpsertAttendance(ctx, db, AttendanceInput{EmployeeID: empID, Date: "2026-08-01", Status: models.AttLate, Remark: "堵车"})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if a.Status != models.AttLate || a.Remark != "堵车" {
		t.Fatalf("应更新为迟到, got %+v", a)
	}
	list, _ := ListAttendances(ctx, db, "", "2026-08-01", "", "", "")
	if len(list) != 1 {
		t.Fatalf("同日应仅 1 条, got %d", len(list))
	}
}

func TestGenerateAttendanceFromSchedule(t *testing.T) {
	db := setupAttDB(t)
	ctx := context.Background()
	empID := attMustEmployee(t, db, "生成员")

	// 周一(weekday=1) 排班
	_, err := CreateSchedule(ctx, db, ScheduleInput{EmployeeID: empID, Weekday: 1, StartTime: "09:00", EndTime: "18:00"})
	if err != nil {
		t.Fatalf("排班失败: %v", err)
	}
	// 一条已批请假，覆盖 2026-08-03（该日为周一）
	gen := code.NewGenerator(db)
	_, err = CreateLeaveRequest(ctx, db, gen, LeaveRequestInput{
		EmployeeID: empID, Type: models.LeavePersonal, StartDate: "2026-08-03", EndDate: "2026-08-03",
	})
	if err != nil {
		t.Fatalf("请假失败: %v", err)
	}
	if _, err := DecideLeaveRequest(ctx, db, 1, true, 1, ""); err != nil {
		t.Fatalf("审批失败: %v", err)
	}

	// 生成 2026-08-03（周一）
	n, err := GenerateAttendance(ctx, db, "2026-08-03")
	if err != nil {
		t.Fatalf("生成失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("应生成 1 条, got %d", n)
	}
	list, _ := ListAttendances(ctx, db, "", "2026-08-03", "", "", "")
	if len(list) != 1 {
		t.Fatalf("应有 1 条考勤, got %d", len(list))
	}
	if list[0].Status != models.AttLeave || list[0].LeaveType != models.LeavePersonal {
		t.Fatalf("应识别为请假, got %+v", list[0])
	}

	// 再生成同日期：去重不重复
	n2, _ := GenerateAttendance(ctx, db, "2026-08-03")
	if n2 != 0 {
		t.Fatalf("重复生成应为 0, got %d", n2)
	}
}
