package models

import (
	"time"

	"gorm.io/gorm"
)

// 角色枚举
const (
	RoleAdmin     = "admin"
	RoleSales     = "sales"
	RoleSalesLead = "sales_lead"
	RoleFinance   = "finance"
	RoleHR        = "hr"
)

// 商单状态（对齐 Salesforce Opportunity 裁剪 6 态）
const (
	DealProspecting  = "prospecting"
	DealQualifying   = "qualifying"
	DealProposal     = "proposal"
	DealNegotiating  = "negotiating"
	DealPendingApproval = "pending_approval" // 折扣审批提交后，等待审批通过才能赢单
	DealWon          = "won"
	DealLost         = "lost"
)

// 合同状态（对齐 SF CLM 裁剪；approving 二期预留）
const (
	ContractDraft      = "draft"
	ContractPending    = "pending"
	ContractPendingApproval = "pending_approval" // 签约审批提交后，等待审批通过才能签约
	ContractSigned     = "signed"
	ContractPerforming = "performing"
	ContractCompleted  = "completed"
	ContractCancelled  = "cancelled"
	ContractTerminated = "terminated"
	ContractExpired    = "expired"
)

// 回款计划状态（逾期为派生，不持久化）
const (
	PlanPending = "pending"
	PlanPartial = "partial"
	PlanPaid    = "paid"
)

// Base 公共字段；ID JSON 序列化为 string（防 JS Number 精度问题）
type Base struct {
	ID        uint           `gorm:"primaryKey" json:"id,string"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// GetDeletedAtValid 供审计回调识别软删除（deleted_at 由 null 变为非 null）
func (b Base) GetDeletedAtValid() bool { return b.DeletedAt.Valid }

type Employee struct {
	Base
	Name          string `json:"name"`
	Email         string `gorm:"uniqueIndex" json:"email"`
	Phone         string `json:"phone"`
	Dept          string `json:"dept"`
	Position      string `json:"position"`
	Role          string `json:"role"`
	PasswordHash  string `json:"-"`
	MustChangePwd bool   `json:"must_change_pwd"`
	Status        string `json:"status"` // active/disabled
}

type Customer struct {
	Base
	Code     string `gorm:"uniqueIndex" json:"code"`
	Name     string `gorm:"uniqueIndex" json:"name"`
	Industry string `json:"industry"`
	Source   string `json:"source"`
	Level    string `json:"level"`
	OwnerID  uint   `json:"owner_id,string"` // 0 = 无主，即公海客户
	Owner    *Employee `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
	Remark   string `json:"remark"`

	// M3-1 公海池：支撑领取保护期与超时回收判定
	LastFollowedAt *time.Time `json:"last_followed_at,omitempty"`
	ClaimedAt      *time.Time `json:"claimed_at,omitempty"`
	PoolReason     string     `json:"pool_reason"` // 进入公海的原因（release/recycle/offboard）
}

type Contact struct {
	Base
	CustomerID uint   `json:"customer_id,string"`
	Name       string `json:"name"`
	Phone      string `json:"phone"`
	Email      string `json:"email"`
	Position   string `json:"position"`
	IsPrimary  bool   `json:"is_primary"`
	Remark     string `json:"remark"`
}

type Deal struct {
	Base
	Code             string    `gorm:"uniqueIndex" json:"code"`
	CustomerID       uint      `json:"customer_id,string"`
	Customer         *Customer `gorm:"foreignKey:CustomerID" json:"customer,omitempty"`
	Title            string    `json:"title"`
	AmountCent       int64     `json:"amount_cent"`
	Probability      int       `json:"probability"`
	ExpectedSignDate string    `json:"expected_sign_date"`
	Status           string    `json:"status"`
	LostReason       string    `json:"lost_reason"`
	DiscountAmountCent int64   `json:"discount_amount_cent"` // 审批通过的折扣金额（分）；0 表示无折扣
	OwnerID          uint      `json:"owner_id,string"`
	Owner            *Employee `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
	Remark           string    `json:"remark"`
}

type Contract struct {
	Base
	Code            string    `gorm:"uniqueIndex" json:"code"`
	CustomerID      uint      `json:"customer_id,string"`
	Customer        *Customer `gorm:"foreignKey:CustomerID" json:"customer,omitempty"`
	Title           string    `json:"title"`
	AmountCent      int64     `json:"amount_cent"`
	SignDate        string    `json:"sign_date"`
	StartDate       string    `json:"start_date"`
	ExpireDate      string    `json:"expire_date"`
	Status          string    `json:"status"`
	TerminateReason string    `json:"terminate_reason"`
	OwnerID         uint      `json:"owner_id,string"`
	Owner           *Employee `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
	Remark          string    `json:"remark"`
	Deals           []Deal    `gorm:"many2many:deal_contracts" json:"deals,omitempty"`
}

// DealContract N:N 中间表（显式模型便于审计与查询）
type DealContract struct {
	ID         uint      `gorm:"primaryKey" json:"id,string"`
	DealID     uint      `gorm:"uniqueIndex:idx_dc_pair" json:"deal_id,string"`
	ContractID uint      `gorm:"uniqueIndex:idx_dc_pair" json:"contract_id,string"`
	CreatedAt  time.Time `json:"created_at"`
}

func (DealContract) TableName() string { return "deal_contracts" }

type PaymentPlan struct {
	Base
	ContractID uint   `json:"contract_id,string"`
	PeriodNo   int    `json:"period_no"`
	DueDate    string `json:"due_date"`
	AmountCent int64  `json:"amount_cent"`
	Status     string `json:"status"` // pending/partial/paid
}

type PaymentRecord struct {
	Base
	ContractID uint   `json:"contract_id,string"`
	PlanID     *uint  `json:"plan_id,string"` // 可空=不核销计划
	AmountCent int64  `json:"amount_cent"`
	PaidAt     string `json:"paid_at"`
	Method     string `json:"method"`
	Remark     string `json:"remark"`
	CreatedBy  uint   `json:"created_by,string"`
}

type Notification struct {
	ID         uint      `gorm:"primaryKey" json:"id,string"`
	UserID     uint      `json:"user_id,string"`
	Type       string    `json:"type"` // contract_expiring/payment_overdue
	Title      string    `json:"title"`
	Content    string    `json:"content"`
	EntityType string    `json:"entity_type"`
	EntityID   uint      `json:"entity_id,string"`
	DedupKey   string    `gorm:"uniqueIndex" json:"-"`
	IsRead     bool      `json:"is_read"`
	CreatedAt  time.Time `json:"created_at"`
}

type Attachment struct {
	ID         uint      `gorm:"primaryKey" json:"id,string"`
	EntityType string    `json:"entity_type"` // contract（一期仅合同）
	EntityID   uint      `json:"entity_id,string"`
	FileName   string    `json:"file_name"`
	FilePath   string    `json:"-"` // 相对路径，不对外暴露
	FileSize   int64     `json:"file_size"`
	Mime       string    `json:"mime"`
	UploadedBy uint      `json:"uploaded_by,string"`
	CreatedAt  time.Time `json:"created_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

type AuditLog struct {
	ID         uint      `gorm:"primaryKey" json:"id,string"`
	EntityType string    `json:"entity_type"`
	EntityID   uint      `json:"entity_id,string"`
	Action     string    `json:"action"` // create/update/delete/transfer/status_change
	OperatorID uint      `json:"operator_id,string"` // 0=系统
	BeforeJSON string    `json:"before_json"`
	AfterJSON  string    `json:"after_json"`
	CreatedAt  time.Time `json:"created_at"`
}

// 审批流（M2-1）：合同签约审批、商单折扣审批
const (
	ApprovalPending  = "pending"
	ApprovalApproved = "approved"
	ApprovalRejected = "rejected"

	ApprovalContractSign = "contract_sign" // 合同签约审批
	ApprovalDealDiscount = "deal_discount" // 商单折扣审批
)

type Approval struct {
	Base
	Code        string `gorm:"uniqueIndex" json:"code"`
	EntityType  string `json:"entity_type"` // contract | deal
	EntityID    uint   `json:"entity_id,string"`
	Kind        string `json:"kind"` // contract_sign | deal_discount
	Status      string `json:"status"`
	ApplicantID uint   `json:"applicant_id,string"`
	ApproverID  uint   `json:"approver_id,string"`
	AmountCent  int64  `json:"amount_cent"` // 商单折扣金额（分）
	Note        string `json:"note"`
	RejectReason string `json:"reject_reason"`
}

// 开票（M2-2）：与合同关联，金额受合同额约束
const (
	InvoiceDraft  = "draft"  // 待开
	InvoiceIssued = "issued" // 已开
	InvoiceVoided = "voided" // 已作废
)

type Invoice struct {
	Base
	Code           string    `gorm:"uniqueIndex" json:"code"`
	ContractID     uint      `json:"contract_id,string"`
	Contract       *Contract `gorm:"foreignKey:ContractID" json:"contract,omitempty"`
	PaymentRecordID *uint    `json:"payment_record_id,string"` // 可选：关联回款记录用于对账
	AmountCent     int64     `json:"amount_cent"`
	Status         string    `json:"status"`
	IssuedAt       string    `json:"issued_at"`
	Remark         string    `json:"remark"`
	CreatedBy      uint      `json:"created_by,string"`
}

// 客户公海池（M3-1）
const (
	PoolActionClaim   = "claim"   // 销售从公海领取
	PoolActionRelease = "release" // 负责人主动释放回公海
	PoolActionRecycle = "recycle" // 规则超时自动/手动回收
	PoolActionAssign  = "assign"  // 管理员从公海直接指派

	PoolReasonRelease  = "主动释放"
	PoolReasonNoFollow = "超期未跟进自动回收"
	PoolReasonNoDeal   = "领取后长期未建商单自动回收"
	PoolReasonOffboard = "负责人离职未指定交接人"
)

// CustomerPoolLog 公海流水：业务可查的领取/释放/回收轨迹（区别于通用 audit_logs）
type CustomerPoolLog struct {
	ID          uint      `gorm:"primaryKey" json:"id,string"`
	CustomerID  uint      `json:"customer_id,string"`
	Action      string    `json:"action"`
	FromOwnerID uint      `json:"from_owner_id,string"`
	ToOwnerID   uint      `json:"to_owner_id,string"`
	OperatorID  uint      `json:"operator_id,string"` // 0 = 系统（定时回收）
	Reason      string    `json:"reason"`
	CreatedAt   time.Time `json:"created_at"`
}

func (CustomerPoolLog) TableName() string { return "customer_pool_logs" }

// PoolSettings 公海规则（单行，id 恒为 1）
type PoolSettings struct {
	ID               uint      `gorm:"primaryKey" json:"id,string"`
	Enabled          bool      `json:"enabled"`             // 是否启用自动回收
	MaxClaimPerSales int       `json:"max_claim_per_sales"` // 单个销售名下客户数上限（0=不限）
	IdleDaysNoFollow int       `json:"idle_days_no_follow"` // 超过 N 天无跟进则回收（0=不启用该条）
	IdleDaysNoDeal   int       `json:"idle_days_no_deal"`   // 领取后超过 N 天未建商单则回收（0=不启用该条）
	ProtectDays      int       `json:"protect_days"`        // 领取后保护期内不回收
	UpdatedAt        time.Time `json:"updated_at"`
}

func (PoolSettings) TableName() string { return "pool_settings" }

type CodeCounter struct {
	Prefix string `gorm:"primaryKey"`
	Year   int    `gorm:"primaryKey"`
	Seq    int    `json:"seq"`
}

// Dict 通用数据字典（type：dept/industry/source/level/pay_method）
// 唯一性由 DB 部分索引 uq_dicts_type_value 保证（仅未删除行）
type Dict struct {
	Base
	Type  string `json:"type"`
	Value string `json:"value"`
	Sort  int    `json:"sort"`
}
