// Package services 项目/交付管理（M3-3）：项目 + 成员(人天) + 任务/里程碑。
package services

import (
	"context"
	"errors"
	"strings"

	"bss/internal/middleware"
	"bss/internal/models"
	"bss/internal/pkg/code"

	"gorm.io/gorm"
)

var (
	ErrProjectMissing     = errors.New("项目不存在")
	ErrProjectNameEmpty   = errors.New("项目名称不能为空")
	ErrMemberMissing      = errors.New("成员不存在")
	ErrTaskMissing        = errors.New("任务不存在")
	ErrNoProjectAccess    = errors.New("无权操作该项目（不在你的数据范围内）")
)

// ProjectService 项目/交付管理
type ProjectService struct {
	db      *gorm.DB
	codeGen *code.Generator
}

func NewProjectService(db *gorm.DB) *ProjectService {
	return &ProjectService{db: db, codeGen: code.NewGenerator(db)}
}

// ScopeProject 数据范围：admin/finance/hr 看全部；销售/主管看「自己负责 或 自己是成员」的项目。
func ScopeProject(db *gorm.DB, ctx context.Context) *gorm.DB {
	c := middleware.UserFrom(ctx)
	if c == nil {
		return db.Where("1 = 0")
	}
	switch c.Role {
	case models.RoleAdmin, models.RoleFinance, models.RoleHR:
		return db
	default:
		return db.Where("owner_id = ? OR id IN (SELECT project_id FROM project_members WHERE employee_id = ?)",
			c.UserID, c.UserID)
	}
}

// AccessOK 单条项目写操作鉴权（mutation 前调用）
func (s *ProjectService) AccessOK(ctx context.Context, projectID uint) (bool, error) {
	c := middleware.UserFrom(ctx)
	if c == nil {
		return false, nil
	}
	switch c.Role {
	case models.RoleAdmin, models.RoleFinance, models.RoleHR:
		return true, nil
	default:
		var p models.Project
		if err := s.db.WithContext(ctx).Select("owner_id").Take(&p, projectID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return false, nil
			}
			return false, err
		}
		if p.OwnerID == c.UserID {
			return true, nil
		}
		var cnt int64
		if err := s.db.WithContext(ctx).Model(&models.ProjectMember{}).
			Where("project_id = ? AND employee_id = ?", projectID, c.UserID).Count(&cnt).Error; err != nil {
			return false, err
		}
		return cnt > 0, nil
	}
}

// ---------- 项目 ----------

type ProjectInput struct {
	Name        string `json:"name"`
	CustomerID  *uint  `json:"customer_id,string"`
	OwnerID     uint   `json:"owner_id,string"`
	Status      string `json:"status"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	Description string `json:"description"`
}

func (s *ProjectService) Create(ctx context.Context, in ProjectInput, operatorID uint) (*models.Project, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return nil, ErrProjectNameEmpty
	}
	status := in.Status
	if status == "" {
		status = models.ProjPlanning
	}
	ownerID := in.OwnerID
	if ownerID == 0 {
		ownerID = operatorID
	}
	c, err := s.codeGen.Next(ctx, code.PrefixProject)
	if err != nil {
		return nil, err
	}
	p := models.Project{
		Code: c, Name: in.Name, CustomerID: in.CustomerID, OwnerID: ownerID,
		Status: status, StartDate: in.StartDate, EndDate: in.EndDate, Description: in.Description,
	}
	if err := s.db.WithContext(ctx).Create(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *ProjectService) Update(ctx context.Context, id uint, in ProjectInput) error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return ErrProjectNameEmpty
	}
	res := s.db.WithContext(ctx).Model(&models.Project{}).Where("id = ?", id).Updates(map[string]any{
		"name": in.Name, "customer_id": in.CustomerID, "owner_id": in.OwnerID,
		"status": in.Status, "start_date": in.StartDate, "end_date": in.EndDate, "description": in.Description,
	})
	if res.RowsAffected == 0 {
		return ErrProjectMissing
	}
	return res.Error
}

func (s *ProjectService) Get(ctx context.Context, id uint) (*models.Project, error) {
	var p models.Project
	err := s.db.WithContext(ctx).
		Preload("Owner").Preload("Customer").
		Preload("Members").Preload("Members.Employee").
		Preload("Tasks", func(tx *gorm.DB) *gorm.DB {
			return tx.Order("kind DESC, sort ASC, id ASC") // milestone 优先展示
		}).Preload("Tasks.Assignee").
		Take(&p, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrProjectMissing
	}
	return &p, err
}

func (s *ProjectService) List(ctx context.Context, page, size int, keyword, status, ownerID string) ([]models.Project, int64, error) {
	base := ScopeProject(s.db.WithContext(ctx).Model(&models.Project{}), ctx).Preload("Owner").Preload("Customer")
	if kw := strings.TrimSpace(keyword); kw != "" {
		base = base.Where("name LIKE ?", "%"+kw+"%")
	}
	if status != "" {
		base = base.Where("status = ?", status)
	}
	if ownerID != "" {
		base = base.Where("owner_id = ?", ownerID)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []models.Project
	if err := base.Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (s *ProjectService) Delete(ctx context.Context, id uint) error {
	res := s.db.WithContext(ctx).Delete(&models.Project{}, id)
	if res.RowsAffected == 0 {
		return ErrProjectMissing
	}
	return res.Error
}

// ---------- 成员（人天） ----------

type MemberInput struct {
	EmployeeID  uint    `json:"employee_id,string"`
	Role        string  `json:"role"`
	PlannedDays float64 `json:"planned_days"`
	ActualDays  float64 `json:"actual_days"`
}

func (s *ProjectService) AddMember(ctx context.Context, projectID uint, in MemberInput) (*models.ProjectMember, error) {
	var emp models.Employee
	if err := s.db.WithContext(ctx).Where("id = ? AND status = 'active' AND deleted_at IS NULL", in.EmployeeID).First(&emp).Error; err != nil {
		return nil, ErrEmployeeMissing
	}
	m := models.ProjectMember{
		ProjectID: projectID, EmployeeID: in.EmployeeID, Role: strings.TrimSpace(in.Role),
		PlannedDays: in.PlannedDays, ActualDays: in.ActualDays,
	}
	if err := s.db.WithContext(ctx).Create(&m).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, errors.New("该成员已在项目中")
		}
		return nil, err
	}
	return &m, nil
}

func (s *ProjectService) UpdateMember(ctx context.Context, id uint, in MemberInput) error {
	res := s.db.WithContext(ctx).Model(&models.ProjectMember{}).Where("id = ?", id).Updates(map[string]any{
		"role": in.Role, "planned_days": in.PlannedDays, "actual_days": in.ActualDays,
	})
	if res.RowsAffected == 0 {
		return ErrMemberMissing
	}
	return res.Error
}

func (s *ProjectService) RemoveMember(ctx context.Context, id uint) error {
	res := s.db.WithContext(ctx).Delete(&models.ProjectMember{}, id)
	if res.RowsAffected == 0 {
		return ErrMemberMissing
	}
	return res.Error
}

func (s *ProjectService) ListMembers(ctx context.Context, projectID uint) ([]models.ProjectMember, error) {
	var list []models.ProjectMember
	err := s.db.WithContext(ctx).Preload("Employee").Where("project_id = ?", projectID).
		Order("id").Find(&list).Error
	return list, err
}

// ---------- 任务 / 里程碑 ----------

type TaskInput struct {
	Kind         string  `json:"kind"`
	Title        string  `json:"title"`
	AssigneeID   *uint   `json:"assignee_id,string"`
	DueDate      string  `json:"due_date"`
	Status       string  `json:"status"`
	EstimateDays float64 `json:"estimate_days"`
	Sort         int     `json:"sort"`
}

func (s *ProjectService) AddTask(ctx context.Context, projectID uint, in TaskInput) (*models.ProjectTask, error) {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return nil, errors.New("任务标题不能为空")
	}
	kind := in.Kind
	if kind != models.TaskKindMilestone {
		kind = models.TaskKindTask
	}
	status := in.Status
	if status == "" {
		status = models.TaskTodo
	}
	sort := in.Sort
	if sort == 0 {
		var cnt int64
		s.db.WithContext(ctx).Model(&models.ProjectTask{}).Where("project_id = ?", projectID).Count(&cnt)
		sort = int(cnt) + 1
	}
	t := models.ProjectTask{
		ProjectID: projectID, Kind: kind, Title: in.Title, AssigneeID: in.AssigneeID,
		DueDate: in.DueDate, Status: status, EstimateDays: in.EstimateDays, Sort: sort,
	}
	if err := s.db.WithContext(ctx).Create(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *ProjectService) UpdateTask(ctx context.Context, id uint, in TaskInput) error {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return errors.New("任务标题不能为空")
	}
	kind := in.Kind
	if kind != models.TaskKindMilestone {
		kind = models.TaskKindTask
	}
	if in.Status == "" {
		in.Status = models.TaskTodo
	}
	res := s.db.WithContext(ctx).Model(&models.ProjectTask{}).Where("id = ?", id).Updates(map[string]any{
		"kind": in.Kind, "title": in.Title, "assignee_id": in.AssigneeID,
		"due_date": in.DueDate, "status": in.Status, "estimate_days": in.EstimateDays, "sort": in.Sort,
	})
	if res.RowsAffected == 0 {
		return ErrTaskMissing
	}
	return res.Error
}

func (s *ProjectService) DeleteTask(ctx context.Context, id uint) error {
	res := s.db.WithContext(ctx).Delete(&models.ProjectTask{}, id)
	if res.RowsAffected == 0 {
		return ErrTaskMissing
	}
	return res.Error
}

func (s *ProjectService) ListTasks(ctx context.Context, projectID uint) ([]models.ProjectTask, error) {
	var list []models.ProjectTask
	err := s.db.WithContext(ctx).Preload("Assignee").Where("project_id = ?", projectID).
		Order("kind DESC, sort ASC, id ASC").Find(&list).Error
	return list, err
}
