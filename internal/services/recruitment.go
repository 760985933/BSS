package services

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"bss/internal/models"
	"bss/internal/pkg/code"
)

var (
	ErrJobPostMissing     = errors.New("招聘职位不存在")
	ErrCandidateMissing   = errors.New("候选人不存在")
	ErrStageTerminal      = errors.New("该候选人已处于终态（已入职/已淘汰），不可再流转")
	ErrStageForceRequired = errors.New("阶段回退或跳级需确认（请携带 force=true 重试）")
)

// 候选人阶段前向图（仅允许单步前进）
var candidateForward = map[string]string{
	models.CandApply:     models.CandScreen,
	models.CandScreen:    models.CandInterview,
	models.CandInterview: models.CandOffer,
	models.CandOffer:     models.CandHired,
}

// 候选人的终态（不可再流转）
var candidateTerminal = map[string]bool{
	models.CandHired:    true,
	models.CandRejected: true,
}

// ---------- 招聘职位 ----------

// JobPostInput 职位可写字段
type JobPostInput struct {
	Title       string `json:"title"`
	Dept        string `json:"dept"`
	Headcount   int    `json:"headcount"`
	Status      string `json:"status"`
	Description string `json:"description"`
}

// CreateJobPost 创建招聘职位并生成 JP- 单号
func CreateJobPost(ctx context.Context, db *gorm.DB, gen *code.Generator, in JobPostInput, ownerID uint) (*models.JobPost, error) {
	if in.Title == "" {
		return nil, errors.New("职位名称不能为空")
	}
	status := in.Status
	if status != models.JobOpen && status != models.JobClosed {
		status = models.JobOpen
	}
	c, err := gen.Next(ctx, code.PrefixJobPost)
	if err != nil {
		return nil, err
	}
	jp := models.JobPost{
		Code: c, Title: in.Title, Dept: in.Dept, Headcount: in.Headcount,
		Status: status, Description: in.Description, OwnerID: ownerID,
	}
	if err := db.WithContext(ctx).Create(&jp).Error; err != nil {
		return nil, err
	}
	return &jp, nil
}

// ListJobPosts 查询职位；keyword 匹配名称/编号，status 过滤
func ListJobPosts(ctx context.Context, db *gorm.DB, keyword, status string) ([]models.JobPost, error) {
	q := db.WithContext(ctx).Preload("Owner").Where("deleted_at IS NULL")
	if keyword != "" {
		q = q.Where("title LIKE ? OR code LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var rows []models.JobPost
	if err := q.Order("created_at DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetJobPost 查询单个职位
func GetJobPost(ctx context.Context, db *gorm.DB, id uint) (*models.JobPost, error) {
	var jp models.JobPost
	if err := db.WithContext(ctx).Preload("Owner").Where("id = ? AND deleted_at IS NULL", id).First(&jp).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrJobPostMissing
		}
		return nil, err
	}
	return &jp, nil
}

// UpdateJobPost 更新职位基础信息
func UpdateJobPost(ctx context.Context, db *gorm.DB, id uint, in JobPostInput) error {
	jp, err := GetJobPost(ctx, db, id)
	if err != nil {
		return err
	}
	status := in.Status
	if status != models.JobOpen && status != models.JobClosed {
		status = jp.Status
	}
	return db.WithContext(ctx).Model(jp).Updates(map[string]any{
		"title": in.Title, "dept": in.Dept, "headcount": in.Headcount,
		"status": status, "description": in.Description,
	}).Error
}

// DeleteJobPost 软删职位，并将其名下候选人的 job_post_id 置 0（保留候选人）
func DeleteJobPost(ctx context.Context, db *gorm.DB, id uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		jp, err := GetJobPost(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := tx.Model(&models.Candidate{}).
			Where("job_post_id = ? AND deleted_at IS NULL", id).
			Update("job_post_id", nil).Error; err != nil {
			return err
		}
		return tx.Delete(jp).Error
	})
}

// ---------- 候选人 ----------

// CandidateInput 候选人可写字段（不含 stage，阶段流转走 AdvanceCandidate）
type CandidateInput struct {
	JobPostID *uint  `json:"job_post_id,string"`
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Source    string `json:"source"`
	ResumeURL string `json:"resume_url"`
}

// CreateCandidate 创建候选人（默认阶段 apply）
func CreateCandidate(ctx context.Context, db *gorm.DB, in CandidateInput, ownerID uint) (*models.Candidate, error) {
	if in.Name == "" {
		return nil, errors.New("候选人姓名不能为空")
	}
	if in.JobPostID != nil && *in.JobPostID != 0 {
		if _, err := GetJobPost(ctx, db, *in.JobPostID); err != nil {
			return nil, ErrJobPostMissing
		}
	}
	c := models.Candidate{
		JobPostID: in.JobPostID, Name: in.Name, Phone: in.Phone, Email: in.Email,
		Stage: models.CandApply, Source: in.Source, ResumeURL: in.ResumeURL, OwnerID: ownerID,
	}
	if err := db.WithContext(ctx).Create(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// ListCandidates 查询候选人；支持关键字/职位/阶段过滤
func ListCandidates(ctx context.Context, db *gorm.DB, keyword string, jobPostID uint, stage string) ([]models.Candidate, error) {
	q := db.WithContext(ctx).Preload("JobPost").Preload("Owner").Where("deleted_at IS NULL")
	if keyword != "" {
		q = q.Where("name LIKE ? OR phone LIKE ? OR email LIKE ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	if jobPostID != 0 {
		q = q.Where("job_post_id = ?", jobPostID)
	}
	if stage != "" {
		q = q.Where("stage = ?", stage)
	}
	var rows []models.Candidate
	if err := q.Order("created_at DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetCandidate 查询单个候选人
func GetCandidate(ctx context.Context, db *gorm.DB, id uint) (*models.Candidate, error) {
	var c models.Candidate
	if err := db.WithContext(ctx).Preload("JobPost").Preload("Owner").Where("id = ? AND deleted_at IS NULL", id).First(&c).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCandidateMissing
		}
		return nil, err
	}
	return &c, nil
}

// UpdateCandidate 更新候选人基础信息（不改 stage）
func UpdateCandidate(ctx context.Context, db *gorm.DB, id uint, in CandidateInput) error {
	c, err := GetCandidate(ctx, db, id)
	if err != nil {
		return err
	}
	if in.JobPostID != nil && *in.JobPostID != 0 {
		if c.JobPostID == nil || *c.JobPostID != *in.JobPostID {
			if _, err := GetJobPost(ctx, db, *in.JobPostID); err != nil {
				return ErrJobPostMissing
			}
		}
	}
	return db.WithContext(ctx).Model(c).Updates(map[string]any{
		"job_post_id": in.JobPostID, "name": in.Name, "phone": in.Phone,
		"email": in.Email, "source": in.Source, "resume_url": in.ResumeURL,
	}).Error
}

// DeleteCandidate 软删候选人
func DeleteCandidate(ctx context.Context, db *gorm.DB, id uint) error {
	c, err := GetCandidate(ctx, db, id)
	if err != nil {
		return err
	}
	return db.Delete(c).Error
}

// AdvanceCandidate 阶段流转：单步前进直接通过；回退/跳级需 force；终态锁定
func AdvanceCandidate(ctx context.Context, db *gorm.DB, id uint, to string, force bool) error {
	c, err := GetCandidate(ctx, db, id)
	if err != nil {
		return err
	}
	if candidateTerminal[c.Stage] {
		return ErrStageTerminal
	}
	if to == c.Stage {
		return nil
	}
	// 任意非终态 → 淘汰 允许
	if to == models.CandRejected {
		return db.Model(c).Update("stage", to).Error
	}
	// 单步前进
	if candidateForward[c.Stage] == to {
		return db.Model(c).Update("stage", to).Error
	}
	// 回退或跳级：需 force 确认
	if !force {
		return ErrStageForceRequired
	}
	return db.Model(c).Update("stage", to).Error
}

// CandidateFunnel 招聘漏斗单阶段计数
type CandidateFunnel struct {
	Stage string `json:"stage"`
	Count int64  `json:"count"`
}

// CandidateFunnelStats 按阶段统计候选人数（可选按职位过滤）
func CandidateFunnelStats(ctx context.Context, db *gorm.DB, jobPostID uint) ([]CandidateFunnel, error) {
	stages := []string{
		models.CandApply, models.CandScreen, models.CandInterview,
		models.CandOffer, models.CandHired, models.CandRejected,
	}
	out := make([]CandidateFunnel, 0, len(stages))
	for _, s := range stages {
		q := db.WithContext(ctx).Model(&models.Candidate{}).Where("deleted_at IS NULL AND stage = ?", s)
		if jobPostID != 0 {
			q = q.Where("job_post_id = ?", jobPostID)
		}
		var cnt int64
		if err := q.Count(&cnt).Error; err != nil {
			return nil, err
		}
		out = append(out, CandidateFunnel{Stage: s, Count: cnt})
	}
	return out, nil
}
