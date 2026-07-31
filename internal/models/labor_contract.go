package models

import "time"

// 劳动合同状态
const (
	LCStatusDraft      = "draft"     // 草稿（未生效）
	LCStatusActive     = "active"    // 生效中
	LCStatusExpired    = "expired"   // 已到期
	LCStatusRenewed    = "renewed"   // 已续签（旧合同归档）
	LCStatusTerminated = "terminated" // 已解除
)

// 劳动合同类型
const (
	LCTypeFixed      = "fixed"      // 固定期限
	LCTypeNonFixed   = "nonfixed"   // 无固定期限
	LCTypeInternship = "internship" // 实习
	LCTypeParttime   = "parttime"   // 兼职
)

// 入职步骤状态
const (
	OBStepPending = "pending"
	OBStepDone    = "done"
)

// 入职总状态（派生：四步全 done 即 completed）
const (
	OBStatusInProgress = "in_progress"
	OBStatusCompleted  = "completed"
)

// LaborContract 劳动合同（关联员工），含生命周期状态机
type LaborContract struct {
	Base
	Code            string     `gorm:"uniqueIndex" json:"code"`
	EmployeeID      uint       `json:"employee_id,string"`
	Employee        *Employee  `gorm:"foreignKey:EmployeeID" json:"employee,omitempty"`
	Type            string     `json:"type"`
	StartDate       *time.Time `json:"start_date,omitempty"`
	EndDate         *time.Time `json:"end_date,omitempty"`
	SignDate        *time.Time `json:"sign_date,omitempty"`
	ProbationMonths int        `json:"probation_months"`
	Status          string     `json:"status"`
	TerminateReason string     `json:"terminate_reason,omitempty"`
	OwnerID         uint       `json:"owner_id,string"`
	Owner           *Employee  `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
}

// Onboarding 入职流程（四步骤推进，可关联来源候选人）
type Onboarding struct {
	Base
	Code          string     `gorm:"uniqueIndex" json:"code"`
	EmployeeID    uint       `json:"employee_id,string"`
	Employee      *Employee  `gorm:"foreignKey:EmployeeID" json:"employee,omitempty"`
	CandidateID   *uint      `json:"candidate_id,string,omitempty"`
	Candidate     *Candidate `gorm:"foreignKey:CandidateID" json:"candidate,omitempty"`
	StepProfile   string     `json:"step_profile"`  // 资料登记
	StepEquip     string     `json:"step_equip"`    // 设备领用
	StepTraining  string     `json:"step_training"` // 入职培训
	StepProbation string     `json:"step_probation"` // 试用期
	Status        string     `json:"status"`
	OwnerID       uint       `json:"owner_id,string"`
	Owner         *Employee  `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
}

// IsTerminal 是否为终态（不可再流转）
func (lc LaborContract) IsTerminal() bool {
	return lc.Status == LCStatusExpired || lc.Status == LCStatusRenewed || lc.Status == LCStatusTerminated
}
