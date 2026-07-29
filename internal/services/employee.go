package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
