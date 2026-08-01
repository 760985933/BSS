package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"bss/internal/models"
	"bss/internal/pkg/code"

	"gorm.io/gorm"
)

// 考勤相关错误（ErrEmployeeMissing 复用 employee 服务声明）
var (
	ErrScheduleMissing       = errors.New("排班记录不存在")
	ErrScheduleWeekdayInvalid = errors.New("星期取值须为 1-7")
	ErrLeaveMissing          = errors.New("请假申请不存在")
	ErrLeaveAlreadyDecided   = errors.New("请假申请已审批，不可重复操作")
	ErrLeaveDateInvalid      = errors.New("请假起止日期无效")
	ErrAttendanceMissing     = errors.New("考勤记录不存在")
)

// validLeaveTypes / validAttStatus / validShiftType 收敛枚举
var validLeaveTypes = map[string]bool{
	models.LeaveAnnual: true, models.LeaveSick: true, models.LeavePersonal: true,
	models.LeaveMarriage: true, models.LeaveMaternity: true, models.LeaveBereavement: true,
}
var validAttStatus = map[string]bool{
	models.AttNormal: true, models.AttLate: true, models.AttEarly: true,
	models.AttAbsent: true, models.AttLeave: true, models.AttHoliday: true,
}
var validShiftType = map[string]bool{
	models.ScheduleRegular: true, models.ScheduleNight: true, models.ScheduleShift: true,
}

// ---------------- 排班 ----------------

type ScheduleInput struct {
	EmployeeID uint   `json:"employee_id,string"`
	Weekday    int    `json:"weekday"`
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
	ShiftType  string `json:"shift_type"`
}

func (in ScheduleInput) normShift() string {
	if in.ShiftType == "" {
		return models.ScheduleRegular
	}
	return in.ShiftType
}

// CreateSchedule 新建排班（同员工同日多次保存视为不同班次，不做唯一约束）
func CreateSchedule(ctx context.Context, db *gorm.DB, in ScheduleInput) (*models.AttendanceSchedule, error) {
	if in.EmployeeID == 0 {
		return nil, ErrEmployeeMissing
	}
	if _, err := getEmployee(db, in.EmployeeID); err != nil {
		return nil, err
	}
	if in.Weekday < 1 || in.Weekday > 7 {
		return nil, ErrScheduleWeekdayInvalid
	}
	if strings.TrimSpace(in.StartTime) == "" || strings.TrimSpace(in.EndTime) == "" {
		return nil, errors.New("上下班时间必填")
	}
	s := models.AttendanceSchedule{
		EmployeeID: in.EmployeeID,
		Weekday:    in.Weekday,
		StartTime:  in.StartTime,
		EndTime:    in.EndTime,
		ShiftType:  in.normShift(),
	}
	if err := db.WithContext(ctx).Create(&s).Error; err != nil {
		return nil, err
	}
	return GetSchedule(ctx, db, s.ID)
}

func GetSchedule(ctx context.Context, db *gorm.DB, id uint) (*models.AttendanceSchedule, error) {
	var s models.AttendanceSchedule
	if err := db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).
		Preload("Employee").First(&s).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrScheduleMissing
		}
		return nil, err
	}
	return &s, nil
}

func ListSchedules(ctx context.Context, db *gorm.DB, employeeID string) ([]models.AttendanceSchedule, error) {
	q := db.WithContext(ctx).Where("deleted_at IS NULL").Preload("Employee")
	if employeeID != "" {
		q = q.Where("employee_id = ?", employeeID)
	}
	var list []models.AttendanceSchedule
	if err := q.Order("employee_id ASC, weekday ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func UpdateSchedule(ctx context.Context, db *gorm.DB, id uint, in ScheduleInput) (*models.AttendanceSchedule, error) {
	var s models.AttendanceSchedule
	if err := db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&s).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrScheduleMissing
		}
		return nil, err
	}
	if in.EmployeeID != 0 && in.EmployeeID != s.EmployeeID {
		if _, err := getEmployee(db, in.EmployeeID); err != nil {
			return nil, err
		}
	}
	if in.Weekday != 0 && (in.Weekday < 1 || in.Weekday > 7) {
		return nil, ErrScheduleWeekdayInvalid
	}
	updates := map[string]any{}
	if in.EmployeeID != 0 {
		updates["employee_id"] = in.EmployeeID
	}
	if in.Weekday != 0 {
		updates["weekday"] = in.Weekday
	}
	if strings.TrimSpace(in.StartTime) != "" {
		updates["start_time"] = in.StartTime
	}
	if strings.TrimSpace(in.EndTime) != "" {
		updates["end_time"] = in.EndTime
	}
	if in.ShiftType != "" {
		if !validShiftType[in.normShift()] {
			return nil, errors.New("不支持的班别")
		}
		updates["shift_type"] = in.normShift()
	}
	if err := db.WithContext(ctx).Model(&s).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetSchedule(ctx, db, id)
}

func DeleteSchedule(ctx context.Context, db *gorm.DB, id uint) error {
	res := db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).Delete(&models.AttendanceSchedule{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrScheduleMissing
	}
	// 解除考勤记录对排班的引用
	db.WithContext(ctx).Model(&models.Attendance{}).Where("schedule_id = ?", id).
		Update("schedule_id", nil)
	return nil
}

// ---------------- 请假 ----------------

type LeaveRequestInput struct {
	EmployeeID uint   `json:"employee_id,string"`
	Type       string `json:"type"`
	StartDate  string `json:"start_date"`
	EndDate    string `json:"end_date"`
	Reason     string `json:"reason"`
}

func parseDateTime(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	for _, layout := range []string{"2006-01-02", "2006-01-02 15:04:05", "2006-01-02T15:04:05Z07:00"} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("日期格式应为 YYYY-MM-DD: %w", errors.New("无效日期"))
}

// CreateLeaveRequest 提交请假（默认 pending，生成 LR- 单号）
func CreateLeaveRequest(ctx context.Context, db *gorm.DB, gen *code.Generator, in LeaveRequestInput) (*models.LeaveRequest, error) {
	if in.EmployeeID == 0 {
		return nil, ErrEmployeeMissing
	}
	if _, err := getEmployee(db, in.EmployeeID); err != nil {
		return nil, err
	}
	if !validLeaveTypes[in.Type] {
		return nil, errors.New("不支持的请假类型")
	}
	sd, err := parseDateTime(in.StartDate)
	if err != nil {
		return nil, err
	}
	ed, err := parseDateTime(in.EndDate)
	if err != nil {
		return nil, err
	}
	if sd == nil || ed == nil || sd.After(*ed) {
		return nil, ErrLeaveDateInvalid
	}
	c, err := gen.Next(ctx, code.PrefixLeaveRequest)
	if err != nil {
		return nil, err
	}
	lr := models.LeaveRequest{
		Code:       c,
		EmployeeID: in.EmployeeID,
		Type:       in.Type,
		StartDate:  sd,
		EndDate:    ed,
		Reason:     in.Reason,
		Status:     models.LeavePending,
	}
	if err := db.WithContext(ctx).Create(&lr).Error; err != nil {
		return nil, err
	}
	return GetLeaveRequest(ctx, db, lr.ID)
}

func GetLeaveRequest(ctx context.Context, db *gorm.DB, id uint) (*models.LeaveRequest, error) {
	var lr models.LeaveRequest
	if err := db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).
		Preload("Employee").Preload("Approver").First(&lr).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrLeaveMissing
		}
		return nil, err
	}
	return &lr, nil
}

func ListLeaveRequests(ctx context.Context, db *gorm.DB, employeeID, status, leaveType string) ([]models.LeaveRequest, error) {
	q := db.WithContext(ctx).Where("deleted_at IS NULL").Preload("Employee").Preload("Approver")
	if employeeID != "" {
		q = q.Where("employee_id = ?", employeeID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if leaveType != "" {
		q = q.Where("type = ?", leaveType)
	}
	var list []models.LeaveRequest
	if err := q.Order("id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func DeleteLeaveRequest(ctx context.Context, db *gorm.DB, id uint) error {
	res := db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).Delete(&models.LeaveRequest{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrLeaveMissing
	}
	return nil
}

// DecideLeaveRequest 审批：approve=true 通过（记录 approver/approved_at），false 驳回（记录 reject_reason）
func DecideLeaveRequest(ctx context.Context, db *gorm.DB, id uint, approve bool, approverID uint, rejectReason string) (*models.LeaveRequest, error) {
	var lr models.LeaveRequest
	if err := db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&lr).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrLeaveMissing
		}
		return nil, err
	}
	if lr.Status != models.LeavePending {
		return nil, ErrLeaveAlreadyDecided
	}
	now := time.Now().UTC()
	updates := map[string]any{}
	if approve {
		updates["status"] = models.LeaveApproved
		updates["approver_id"] = approverID
		updates["approved_at"] = &now
	} else {
		updates["status"] = models.LeaveRejected
		updates["reject_reason"] = strings.TrimSpace(rejectReason)
	}
	if err := db.WithContext(ctx).Model(&lr).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetLeaveRequest(ctx, db, id)
}

// ---------------- 考勤记录 ----------------

type AttendanceInput struct {
	EmployeeID uint   `json:"employee_id,string"`
	Date       string `json:"date"` // YYYY-MM-DD
	ScheduleID *uint  `json:"schedule_id,string,omitempty"`
	Status     string `json:"status"`
	LeaveType  string `json:"leave_type,omitempty"`
	Remark     string `json:"remark,omitempty"`
}

func (in AttendanceInput) validate() error {
	if in.EmployeeID == 0 {
		return ErrEmployeeMissing
	}
	if !validAttStatus[in.Status] {
		return errors.New("不支持的出勤状态")
	}
	if in.Date == "" {
		return errors.New("考勤日期必填")
	}
	return nil
}

// UpsertAttendance 按 (员工, 日期) 标记/更新出勤状态（同一天仅一条有效记录）
func UpsertAttendance(ctx context.Context, db *gorm.DB, in AttendanceInput) (*models.Attendance, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	if _, err := getEmployee(db, in.EmployeeID); err != nil {
		return nil, err
	}
	var existing models.Attendance
	err := db.WithContext(ctx).Where("employee_id = ? AND date = ? AND deleted_at IS NULL", in.EmployeeID, in.Date).First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		a := models.Attendance{
			EmployeeID: in.EmployeeID,
			Date:       in.Date,
			ScheduleID: in.ScheduleID,
			Status:     in.Status,
			LeaveType:  in.LeaveType,
			Remark:     in.Remark,
		}
		if err := db.WithContext(ctx).Create(&a).Error; err != nil {
			return nil, err
		}
		return GetAttendance(ctx, db, a.ID)
	}
	updates := map[string]any{
		"status":     in.Status,
		"leave_type": in.LeaveType,
		"remark":     in.Remark,
	}
	updates["schedule_id"] = in.ScheduleID
	if in.EmployeeID != 0 {
		updates["employee_id"] = in.EmployeeID
	}
	if err := db.WithContext(ctx).Model(&existing).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetAttendance(ctx, db, existing.ID)
}

func GetAttendance(ctx context.Context, db *gorm.DB, id uint) (*models.Attendance, error) {
	var a models.Attendance
	if err := db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).
		Preload("Employee").Preload("Schedule").First(&a).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAttendanceMissing
		}
		return nil, err
	}
	return &a, nil
}

func ListAttendances(ctx context.Context, db *gorm.DB, employeeID, date, status, from, to string) ([]models.Attendance, error) {
	q := db.WithContext(ctx).Where("deleted_at IS NULL").Preload("Employee").Preload("Schedule")
	if employeeID != "" {
		q = q.Where("employee_id = ?", employeeID)
	}
	if date != "" {
		q = q.Where("date = ?", date)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if from != "" {
		q = q.Where("date >= ?", from)
	}
	if to != "" {
		q = q.Where("date <= ?", to)
	}
	var list []models.Attendance
	if err := q.Order("date DESC, employee_id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func DeleteAttendance(ctx context.Context, db *gorm.DB, id uint) error {
	res := db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).Delete(&models.Attendance{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrAttendanceMissing
	}
	return nil
}

// GenerateAttendance 依据排班 + 已批请假，为指定日期自动生成考勤记录：
//   - 该星期有排班的员工 → 默认 normal（已存在则跳过）
//   - 若该日期落在某条已批请假区间内 → leave（带 leave_type）
//
// 返回本次新建记录数。
func GenerateAttendance(ctx context.Context, db *gorm.DB, dateStr string) (int, error) {
	if dateStr == "" {
		return 0, errors.New("日期必填")
	}
	day, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return 0, fmt.Errorf("日期格式应为 YYYY-MM-DD: %w", err)
	}
	w := int(day.Weekday()) // Sunday=0
	if w == 0 {
		w = 7
	}

	var schedules []models.AttendanceSchedule
	if err := db.WithContext(ctx).Where("weekday = ? AND deleted_at IS NULL", w).Find(&schedules).Error; err != nil {
		return 0, err
	}
	created := 0
	for _, sc := range schedules {
		var dup models.Attendance
		dErr := db.WithContext(ctx).Where("employee_id = ? AND date = ? AND deleted_at IS NULL", sc.EmployeeID, dateStr).First(&dup).Error
		if dErr == nil {
			continue // 已存在，跳过
		}
		if !errors.Is(dErr, gorm.ErrRecordNotFound) {
			return created, dErr
		}
		// 是否处于已批请假区间（按日期字符串比较，ISO 格式可直接比较）
		var lr models.LeaveRequest
		fErr := db.WithContext(ctx).Where("employee_id = ? AND status = ? AND deleted_at IS NULL AND start_date <= ? AND end_date >= ?",
			sc.EmployeeID, models.LeaveApproved, day, day).First(&lr).Error
		status := models.AttNormal
		leaveType := ""
		if fErr == nil {
			status = models.AttLeave
			leaveType = lr.Type
		} else if !errors.Is(fErr, gorm.ErrRecordNotFound) {
			return created, fErr
		}
		sid := sc.ID
		a := models.Attendance{
			EmployeeID: sc.EmployeeID,
			Date:       dateStr,
			ScheduleID: &sid,
			Status:     status,
			LeaveType:  leaveType,
		}
		if err := db.WithContext(ctx).Create(&a).Error; err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}
