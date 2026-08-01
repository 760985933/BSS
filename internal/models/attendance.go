package models

import "time"

// 排班班别
const (
	ScheduleRegular = "regular" // 常日班
	ScheduleNight   = "night"   // 夜班
	ScheduleShift   = "shift"   // 倒班
)

// 请假类型
const (
	LeaveAnnual      = "annual"      // 年假
	LeaveSick        = "sick"        // 病假
	LeavePersonal    = "personal"    // 事假
	LeaveMarriage    = "marriage"    // 婚假
	LeaveMaternity   = "maternity"   // 产假/陪产假
	LeaveBereavement = "bereavement" // 丧假
)

// 请假状态
const (
	LeavePending  = "pending"
	LeaveApproved = "approved"
	LeaveRejected = "rejected"
)

// 出勤状态
const (
	AttNormal  = "normal"  // 正常出勤
	AttLate    = "late"    // 迟到
	AttEarly   = "early"   // 早退
	AttAbsent  = "absent"  // 缺勤
	AttLeave   = "leave"   // 请假（已批）
	AttHoliday = "holiday" // 法定节假日
)

// AttendanceSchedule 排班配置（按员工 / 星期 / 班别）
type AttendanceSchedule struct {
	Base
	EmployeeID uint      `json:"employee_id,string"`
	Employee   *Employee `gorm:"foreignKey:EmployeeID" json:"employee,omitempty"`
	Weekday    int       `json:"weekday"` // 1=周一 .. 7=周日
	StartTime  string    `json:"start_time"`
	EndTime    string    `json:"end_time"`
	ShiftType  string    `json:"shift_type"`
}

// LeaveRequest 请假申请（含审批状态机）
type LeaveRequest struct {
	Base
	Code         string    `gorm:"uniqueIndex" json:"code"`
	EmployeeID   uint      `json:"employee_id,string"`
	Employee     *Employee `gorm:"foreignKey:EmployeeID" json:"employee,omitempty"`
	Type         string    `json:"type"`
	StartDate    *time.Time `json:"start_date,omitempty"`
	EndDate      *time.Time `json:"end_date,omitempty"`
	Reason       string    `json:"reason,omitempty"`
	Status       string    `json:"status"`
	ApproverID   *uint     `json:"approver_id,string,omitempty"`
	Approver     *Employee `gorm:"foreignKey:ApproverID" json:"approver,omitempty"`
	ApprovedAt   *time.Time `json:"approved_at,omitempty"`
	RejectReason string    `json:"reject_reason,omitempty"`
}

// Attendance 考勤记录（按员工 + 日期标记出勤状态）
type Attendance struct {
	Base
	EmployeeID uint                `json:"employee_id,string"`
	Employee   *Employee           `gorm:"foreignKey:EmployeeID" json:"employee,omitempty"`
	Date       string              `json:"date"` // YYYY-MM-DD
	ScheduleID *uint               `json:"schedule_id,string,omitempty"`
	Schedule   *AttendanceSchedule `gorm:"foreignKey:ScheduleID" json:"schedule,omitempty"`
	Status     string              `json:"status"`
	LeaveType  string              `json:"leave_type,omitempty"`
	Remark     string              `json:"remark,omitempty"`
}
