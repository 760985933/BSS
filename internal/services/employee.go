package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"bss/internal/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// InitialPassword 新建/重置后的初始密码（首次登录强制改密，PRD §4.1）
const InitialPassword = "Bss@1234"

// 业务错误（handler 映射为 4xx）
var (
	ErrEmailExists     = errors.New("该邮箱已被使用")
	ErrInvalidRole     = errors.New("非法角色")
	ErrLastAdmin       = errors.New("系统至少保留一个启用状态的管理员")
	ErrCannotSelfOp    = errors.New("不能对自己执行此操作")
	ErrEmployeeMissing = errors.New("员工不存在")
	ErrOffboardSameEmployee = errors.New("交接人不能是本人")
	ErrSuccessorMissing     = errors.New("请选择交接人")
	ErrSuccessorNotActive   = errors.New("交接人必须是启用状态的员工")
	ErrSuccessorRequired    = errors.New("该员工名下有商单或合同，必须指定交接人（不能仅退回公海）")
)

var validRoles = map[string]bool{
	models.RoleAdmin: true, models.RoleSales: true, models.RoleSalesLead: true,
	models.RoleFinance: true, models.RoleHR: true,
}

// EmployeeInput 新建/编辑员工的可写字段（email、密码不经过此结构修改）
type EmployeeInput struct {
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Dept     string `json:"dept"`
	Position string `json:"position"`
	Role     string `json:"role"`
}

type EmployeeService struct {
	db *gorm.DB
}

func NewEmployeeService(db *gorm.DB) *EmployeeService {
	return &EmployeeService{db: db}
}

// Create 新建员工：邮箱唯一、角色合法、初始密码 + 强制改密
func (s *EmployeeService) Create(ctx context.Context, in EmployeeInput, email string) (*models.Employee, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || in.Name == "" {
		return nil, errors.New("姓名和邮箱不能为空")
	}
	if !validRoles[in.Role] {
		return nil, ErrInvalidRole
	}
	var cnt int64
	if err := s.db.WithContext(ctx).Model(&models.Employee{}).Where("email = ?", email).Count(&cnt).Error; err != nil {
		return nil, err
	}
	if cnt > 0 {
		return nil, ErrEmailExists
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(InitialPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	emp := models.Employee{
		Name: in.Name, Email: email, Phone: in.Phone, Dept: in.Dept,
		Position: in.Position, Role: in.Role,
		PasswordHash: string(hash), MustChangePwd: true, Status: "active",
	}
	if err := s.db.WithContext(ctx).Create(&emp).Error; err != nil {
		return nil, err
	}
	return &emp, nil
}

// Update 编辑员工基础信息与角色；降级最后一个 admin 时拒绝
func (s *EmployeeService) Update(ctx context.Context, id uint, in EmployeeInput) error {
	var emp models.Employee
	if err := s.db.WithContext(ctx).Take(&emp, id).Error; err != nil {
		return ErrEmployeeMissing
	}
	if in.Name == "" {
		return errors.New("姓名不能为空")
	}
	if !validRoles[in.Role] {
		return ErrInvalidRole
	}
	if emp.Role == models.RoleAdmin && in.Role != models.RoleAdmin {
		if err := s.ensureAnotherAdmin(ctx, id); err != nil {
			return err
		}
	}
	return s.db.WithContext(ctx).Model(&emp).Updates(map[string]any{
		"name": in.Name, "phone": in.Phone, "dept": in.Dept,
		"position": in.Position, "role": in.Role,
	}).Error
}

// SetStatus 停用/启用；不能停自己；保留至少一个 active admin
func (s *EmployeeService) SetStatus(ctx context.Context, id uint, operatorID uint, active bool) error {
	var emp models.Employee
	if err := s.db.WithContext(ctx).Take(&emp, id).Error; err != nil {
		return ErrEmployeeMissing
	}
	if !active {
		if id == operatorID {
			return ErrCannotSelfOp
		}
		if emp.Role == models.RoleAdmin {
			if err := s.ensureAnotherAdmin(ctx, id); err != nil {
				return err
			}
		}
	}
	status := "active"
	if !active {
		status = "disabled"
	}
	return s.db.WithContext(ctx).Model(&emp).Update("status", status).Error
}

// ResetPassword admin 重置为初始密码并强制改密（PRD §4.1）
func (s *EmployeeService) ResetPassword(ctx context.Context, id uint) error {
	var emp models.Employee
	if err := s.db.WithContext(ctx).Take(&emp, id).Error; err != nil {
		return ErrEmployeeMissing
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(InitialPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Model(&emp).Updates(map[string]any{
		"password_hash": string(hash), "must_change_pwd": true,
	}).Error
}

// ensureAnotherAdmin 保证 id 之外还有至少一个 active admin
func (s *EmployeeService) ensureAnotherAdmin(ctx context.Context, excludeID uint) error {
	var cnt int64
	if err := s.db.WithContext(ctx).Model(&models.Employee{}).
		Where("role = ? AND status = 'active' AND id <> ?", models.RoleAdmin, excludeID).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt == 0 {
		return ErrLastAdmin
	}
	return nil
}

// ListDict / AddDict / RemoveDict —— 数据字典维护

func (s *EmployeeService) ListDict(ctx context.Context, dictType string) ([]models.Dict, error) {
	var list []models.Dict
	q := s.db.WithContext(ctx).Order("type, sort, id")
	if dictType != "" {
		q = q.Where("type = ?", dictType)
	}
	return list, q.Find(&list).Error
}

func (s *EmployeeService) AddDict(ctx context.Context, dictType, value string) (*models.Dict, error) {
	value = strings.TrimSpace(value)
	if dictType == "" || value == "" {
		return nil, errors.New("字典类型和值不能为空")
	}
	d := models.Dict{Type: dictType, Value: value}
	if err := s.db.WithContext(ctx).Create(&d).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, fmt.Errorf("「%s」已存在", value)
		}
		return nil, err
	}
	return &d, nil
}

// RemoveDict 删除字典项；部门被在职员工使用时禁止删除
func (s *EmployeeService) RemoveDict(ctx context.Context, id uint) error {
	var d models.Dict
	if err := s.db.WithContext(ctx).Take(&d, id).Error; err != nil {
		return errors.New("字典项不存在")
	}
	if d.Type == "dept" {
		var cnt int64
		if err := s.db.WithContext(ctx).Model(&models.Employee{}).Where("dept = ?", d.Value).Count(&cnt).Error; err != nil {
			return err
		}
		if cnt > 0 {
			return fmt.Errorf("部门「%s」下仍有 %d 名员工，不能删除", d.Value, cnt)
		}
	}
	return s.db.WithContext(ctx).Delete(&d).Error
}

// ---------- 离职交接（M2-4） ----------

// OffboardPreview 交接预览：目标员工是否在职、名下待转移数据量。
type OffboardPreview struct {
	Active      bool  `json:"active"`
	HasData     bool  `json:"has_data"`
	Customers   int64 `json:"customers"`
	Deals       int64 `json:"deals"`
	Contracts   int64 `json:"contracts"`
}

// OffboardResult 交接结果（供前端提示）
type OffboardResult struct {
	Customers int64 `json:"customers"`
	Deals     int64 `json:"deals"`
	Contracts int64 `json:"contracts"`
}

func (s *EmployeeService) countOwned(ctx context.Context, empID uint) (customers, deals, contracts int64, err error) {
	if e := s.db.WithContext(ctx).Model(&models.Customer{}).Where("owner_id = ?", empID).Count(&customers).Error; e != nil {
		return 0, 0, 0, e
	}
	if e := s.db.WithContext(ctx).Model(&models.Deal{}).Where("owner_id = ?", empID).Count(&deals).Error; e != nil {
		return 0, 0, 0, e
	}
	if e := s.db.WithContext(ctx).Model(&models.Contract{}).Where("owner_id = ?", empID).Count(&contracts).Error; e != nil {
		return 0, 0, 0, e
	}
	return
}

// OffboardPreview 返回交接前预览信息。
func (s *EmployeeService) OffboardPreview(ctx context.Context, empID uint) (*OffboardPreview, error) {
	var emp models.Employee
	if err := s.db.WithContext(ctx).Take(&emp, empID).Error; err != nil {
		return nil, ErrEmployeeMissing
	}
	c, d, ct, err := s.countOwned(ctx, empID)
	if err != nil {
		return nil, err
	}
	return &OffboardPreview{
		Active:    emp.Status == "active",
		HasData:   c+d+ct > 0,
		Customers: c, Deals: d, Contracts: ct,
	}, nil
}

// Offboard 离职交接：将名下客户/商单/合同转移给交接人，并停用该员工（事务内 + 审计）。
// successorID = 0 表示不指定交接人，名下客户退回公海（M3-1）；但商单/合同不允许无主，
// 此时若该员工仍有商单或合同，必须指定交接人。operatorID 为执行人（写入审计）。
func (s *EmployeeService) Offboard(ctx context.Context, empID, successorID, operatorID uint) (*OffboardResult, error) {
	toPool := successorID == 0
	if !toPool && empID == successorID {
		return nil, ErrOffboardSameEmployee
	}
	var emp, succ models.Employee
	if err := s.db.WithContext(ctx).Take(&emp, empID).Error; err != nil {
		return nil, ErrEmployeeMissing
	}
	if !toPool {
		if err := s.db.WithContext(ctx).Take(&succ, successorID).Error; err != nil {
			return nil, ErrSuccessorMissing
		}
		if succ.Status != "active" {
			return nil, ErrSuccessorNotActive
		}
	}
	// 不能交接自己（停自己）
	if empID == operatorID {
		return nil, ErrCannotSelfOp
	}
	// 若目标是最后的 admin，禁止停用
	if emp.Role == models.RoleAdmin {
		if err := s.ensureAnotherAdmin(ctx, empID); err != nil {
			return nil, err
		}
	}
	c, d, ct, err := s.countOwned(ctx, empID)
	if err != nil {
		return nil, err
	}
	// 商单/合同不能无主
	if toPool && (d > 0 || ct > 0) {
		return nil, ErrSuccessorRequired
	}

	// 退公海时先取客户 ID，用于写公海流水
	var poolIDs []uint
	if toPool && c > 0 {
		if err := s.db.WithContext(ctx).Model(&models.Customer{}).
			Where("owner_id = ?", empID).Pluck("id", &poolIDs).Error; err != nil {
			return nil, err
		}
	}

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	// 批量转移（不触发逐行审计，改用下方汇总审计）
	if toPool {
		if err := tx.Model(&models.Customer{}).Where("owner_id = ?", empID).Updates(map[string]any{
			"owner_id":    0,
			"claimed_at":  nil,
			"pool_reason": models.PoolReasonOffboard,
		}).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		if len(poolIDs) > 0 {
			now := time.Now().UTC()
			logs := make([]models.CustomerPoolLog, 0, len(poolIDs))
			for _, id := range poolIDs {
				logs = append(logs, models.CustomerPoolLog{
					CustomerID: id, Action: models.PoolActionRecycle,
					FromOwnerID: empID, ToOwnerID: 0, OperatorID: operatorID,
					Reason: models.PoolReasonOffboard, CreatedAt: now,
				})
			}
			if err := tx.Create(&logs).Error; err != nil {
				tx.Rollback()
				return nil, err
			}
		}
	} else if err := tx.Model(&models.Customer{}).Where("owner_id = ?", empID).Update("owner_id", successorID).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Model(&models.Deal{}).Where("owner_id = ?", empID).Update("owner_id", successorID).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Model(&models.Contract{}).Where("owner_id = ?", empID).Update("owner_id", successorID).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	// 停用员工
	if err := tx.Model(&models.Employee{}).Where("id = ?", empID).Update("status", "disabled").Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	// 汇总审计：记录交接动作
	afterJSON, _ := json.Marshal(map[string]any{
		"successor_id": successorID,
		"counts":       map[string]int64{"customers": c, "deals": d, "contracts": ct},
	})
	if err := tx.Exec(
		`INSERT INTO audit_logs (entity_type, entity_id, action, operator_id, before_json, after_json, created_at)
		 VALUES (?, ?, 'offboard', ?, '', ?, ?)`,
		"employee", empID, operatorID, string(afterJSON), time.Now().UTC(),
	).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return &OffboardResult{Customers: c, Deals: d, Contracts: ct}, nil
}
