package services

import (
	"context"
	"errors"
	"fmt"
	"log"

	"bss/internal/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AuthService 认证与账号初始化
type AuthService struct {
	db *gorm.DB
}

func NewAuthService(db *gorm.DB) *AuthService {
	return &AuthService{db: db}
}

// InitAdmin 首启初始化 admin（employees 为空时创建，强制首次登录改密）
func (s *AuthService) InitAdmin(ctx context.Context) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.Employee{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	admin := models.Employee{
		Name:          "系统管理员",
		Email:         "admin@bss.local",
		Role:          models.RoleAdmin,
		PasswordHash:  string(hash),
		MustChangePwd: true,
		Status:        "active",
	}
	if err := s.db.WithContext(ctx).Create(&admin).Error; err != nil {
		return fmt.Errorf("初始化 admin 失败: %w", err)
	}
	log.Println("[初始化] 已创建 admin 账号：admin@bss.local / admin123（首次登录强制改密）")
	return nil
}

// Login 校验邮箱密码，返回员工实体（调用方签发 JWT）
func (s *AuthService) Login(ctx context.Context, email, password string) (*models.Employee, error) {
	var emp models.Employee
	err := s.db.WithContext(ctx).Where("email = ?", email).Take(&emp).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("邮箱或密码错误")
	}
	if err != nil {
		return nil, err
	}
	if emp.Status != "active" {
		return nil, errors.New("账号已停用，请联系管理员")
	}
	if bcrypt.CompareHashAndPassword([]byte(emp.PasswordHash), []byte(password)) != nil {
		return nil, errors.New("邮箱或密码错误")
	}
	return &emp, nil
}

// ChangePassword 修改密码（校验旧密码，清除强制改密标记）
func (s *AuthService) ChangePassword(ctx context.Context, userID uint, oldPwd, newPwd string) error {
	if len(newPwd) < 8 {
		return errors.New("新密码长度至少 8 位")
	}
	var emp models.Employee
	if err := s.db.WithContext(ctx).Take(&emp, userID).Error; err != nil {
		return errors.New("账号不存在")
	}
	if bcrypt.CompareHashAndPassword([]byte(emp.PasswordHash), []byte(oldPwd)) != nil {
		return errors.New("原密码错误")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Model(&emp).Updates(map[string]any{
		"password_hash":   string(hash),
		"must_change_pwd": false,
	}).Error
}
