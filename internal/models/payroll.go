package models

import "time"

// 薪资核算状态
const (
	PayrollDraft  = "draft"  // 草稿（可手动调整各项金额）
	PayrollCalced = "calced" // 已核算（自动计算实发，金额锁定不可改）
	PayrollPaid   = "paid"   // 已发放
)

// Payroll 薪资核算（按月按员工一条，金额整数分）
type Payroll struct {
	Base
	Code        string    `gorm:"uniqueIndex" json:"code"`
	EmployeeID  uint      `json:"employee_id,string"`
	Employee    *Employee `gorm:"foreignKey:EmployeeID" json:"employee,omitempty"`
	Period      string    `json:"period"` // YYYY-MM
	BaseCent    int64     `json:"base_cent"`
	BonusCent   int64     `json:"bonus_cent"`
	DeductionCent int64   `json:"deduction_cent"`
	SocialCent  int64     `json:"social_cent"`
	TaxCent     int64     `json:"tax_cent"`
	NetCent     int64     `json:"net_cent"`
	Status      string    `json:"status"`
	PaidAt      *time.Time `json:"paid_at,omitempty"`
	OwnerID     uint      `json:"owner_id,string"`
	Owner       *Employee `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
	Remark      string    `json:"remark,omitempty"`
}
