package models

// 招聘职位状态
const (
	JobOpen   = "open"   // 招聘中
	JobClosed = "closed" // 已关闭
)

// 候选人阶段（招聘漏斗）
const (
	CandApply     = "apply"     // 投递
	CandScreen    = "screen"    // 筛选
	CandInterview = "interview" // 面试
	CandOffer     = "offer"     // Offer
	CandHired     = "hired"     // 已入职
	CandRejected  = "rejected"  // 已淘汰
)

// JobPost 招聘职位
type JobPost struct {
	Base
	Code        string    `gorm:"uniqueIndex" json:"code"`
	Title       string    `json:"title"`
	Dept        string    `json:"dept"`
	Headcount   int       `json:"headcount"`
	Status      string    `json:"status"` // open / closed
	Description string    `json:"description"`
	OwnerID     uint      `json:"owner_id,string"`
	Owner       *Employee `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
}

// Candidate 候选人（招聘漏斗）
type Candidate struct {
	Base
	JobPostID *uint     `json:"job_post_id,string"`
	JobPost   *JobPost  `gorm:"foreignKey:JobPostID" json:"job_post,omitempty"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	Email     string    `json:"email"`
	Stage     string    `json:"stage"` // apply/screen/interview/offer/hired/rejected
	Source    string    `json:"source"`
	ResumeURL string    `json:"resume_url"`
	OwnerID   uint      `json:"owner_id,string"`
	Owner     *Employee `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
}
