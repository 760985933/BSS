package services

import (
	"context"
	"errors"
	"strings"

	"bss/internal/models"
	"bss/internal/pkg/code"

	"gorm.io/gorm"
)

var (
	ErrCustomerNameExists = errors.New("已存在同名客户")
	ErrCustomerMissing    = errors.New("客户不存在")
	ErrCustomerHasChildren = errors.New("该客户名下存在商单/合同/回款数据，禁止删除")
	ErrNoPermission       = errors.New("无权操作该数据（不在你的数据范围内）")
)

type CustomerService struct {
	db      *gorm.DB
	codeGen *code.Generator
}

func NewCustomerService(db *gorm.DB) *CustomerService {
	return &CustomerService{db: db, codeGen: code.NewGenerator(db)}
}

// CustomerInput 客户可写字段
type CustomerInput struct {
	Name     string `json:"name"`
	Industry string `json:"industry"`
	Source   string `json:"source"`
	Level    string `json:"level"`
	Remark   string `json:"remark"`
}

// Create 新建客户：名称唯一、KH 单号、负责人=当前用户
func (s *CustomerService) Create(ctx context.Context, in CustomerInput, ownerID uint) (*models.Customer, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return nil, errors.New("客户名称不能为空")
	}
	var cnt int64
	s.db.WithContext(ctx).Model(&models.Customer{}).Where("name = ?", in.Name).Count(&cnt)
	if cnt > 0 {
		return nil, ErrCustomerNameExists
	}
	c, err := s.codeGen.Next(ctx, code.PrefixCustomer)
	if err != nil {
		return nil, err
	}
	cust := models.Customer{
		Code: c, Name: in.Name, Industry: in.Industry, Source: in.Source,
		Level: in.Level, Remark: in.Remark, OwnerID: ownerID,
	}
	if err := s.db.WithContext(ctx).Create(&cust).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, ErrCustomerNameExists
		}
		return nil, err
	}
	return &cust, nil
}

// Update 编辑客户：名称唯一（排除自身）；行级权限由 handler 校验后调用
func (s *CustomerService) Update(ctx context.Context, id uint, in CustomerInput) error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return errors.New("客户名称不能为空")
	}
	var cnt int64
	s.db.WithContext(ctx).Model(&models.Customer{}).Where("name = ? AND id <> ?", in.Name, id).Count(&cnt)
	if cnt > 0 {
		return ErrCustomerNameExists
	}
	res := s.db.WithContext(ctx).Model(&models.Customer{}).Where("id = ?", id).Updates(map[string]any{
		"name": in.Name, "industry": in.Industry, "source": in.Source,
		"level": in.Level, "remark": in.Remark,
	})
	if res.RowsAffected == 0 {
		return ErrCustomerMissing
	}
	return res.Error
}

// Transfer 转移负责人（审计由 GORM Hook 记录 owner_id 前后值）
func (s *CustomerService) Transfer(ctx context.Context, id uint, newOwnerID uint) error {
	var cnt int64
	s.db.WithContext(ctx).Model(&models.Employee{}).Where("id = ? AND status = 'active'", newOwnerID).Count(&cnt)
	if cnt == 0 {
		return errors.New("目标负责人不存在或已停用")
	}
	res := s.db.WithContext(ctx).Model(&models.Customer{}).Where("id = ?", id).
		Update("owner_id", newOwnerID)
	if res.RowsAffected == 0 {
		return ErrCustomerMissing
	}
	return res.Error
}

// Delete 软删除；存在下游数据（商单/合同/回款）禁止（PRD §4.3 删除约束）
func (s *CustomerService) Delete(ctx context.Context, id uint) error {
	var cnt int64
	s.db.WithContext(ctx).Model(&models.Deal{}).Where("customer_id = ?", id).Count(&cnt)
	if cnt > 0 {
		return ErrCustomerHasChildren
	}
	s.db.WithContext(ctx).Model(&models.Contract{}).Where("customer_id = ?", id).Count(&cnt)
	if cnt > 0 {
		return ErrCustomerHasChildren
	}
	res := s.db.WithContext(ctx).Delete(&models.Customer{}, id)
	if res.RowsAffected == 0 {
		return ErrCustomerMissing
	}
	return res.Error
}

// Get 详情（含负责人）
func (s *CustomerService) Get(ctx context.Context, id uint) (*models.Customer, error) {
	var cust models.Customer
	err := s.db.WithContext(ctx).Preload("Owner").Take(&cust, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCustomerMissing
	}
	return &cust, err
}

// ---------- 联系人 ----------

// ContactInput 联系人可写字段
type ContactInput struct {
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Position  string `json:"position"`
	IsPrimary bool   `json:"is_primary"`
	Remark    string `json:"remark"`
}

func (s *CustomerService) ListContacts(ctx context.Context, customerID uint) ([]models.Contact, error) {
	var list []models.Contact
	err := s.db.WithContext(ctx).Where("customer_id = ?", customerID).
		Order("is_primary DESC, id").Find(&list).Error
	return list, err
}

// CreateContact 新建联系人；is_primary=true 时事务内清除同客户其他首要标记
func (s *CustomerService) CreateContact(ctx context.Context, customerID uint, in ContactInput) (*models.Contact, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, errors.New("联系人姓名不能为空")
	}
	ct := models.Contact{
		CustomerID: customerID, Name: in.Name, Phone: in.Phone, Email: in.Email,
		Position: in.Position, IsPrimary: in.IsPrimary, Remark: in.Remark,
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if in.IsPrimary {
			if err := tx.Model(&models.Contact{}).Where("customer_id = ?", customerID).
				Update("is_primary", false).Error; err != nil {
				return err
			}
		}
		return tx.Create(&ct).Error
	})
	return &ct, err
}

func (s *CustomerService) UpdateContact(ctx context.Context, id uint, in ContactInput) error {
	var ct models.Contact
	if err := s.db.WithContext(ctx).Take(&ct, id).Error; err != nil {
		return errors.New("联系人不存在")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if in.IsPrimary {
			if err := tx.Model(&models.Contact{}).
				Where("customer_id = ? AND id <> ?", ct.CustomerID, id).
				Update("is_primary", false).Error; err != nil {
				return err
			}
		}
		return tx.Model(&ct).Updates(map[string]any{
			"name": in.Name, "phone": in.Phone, "email": in.Email,
			"position": in.Position, "is_primary": in.IsPrimary, "remark": in.Remark,
		}).Error
	})
}

func (s *CustomerService) DeleteContact(ctx context.Context, id uint) error {
	res := s.db.WithContext(ctx).Delete(&models.Contact{}, id)
	if res.RowsAffected == 0 {
		return errors.New("联系人不存在")
	}
	return res.Error
}

// HasContact / 归属辅助
func (s *CustomerService) OwnerOf(ctx context.Context, id uint) (uint, error) {
	var cust models.Customer
	err := s.db.WithContext(ctx).Select("owner_id").Take(&cust, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, ErrCustomerMissing
	}
	return cust.OwnerID, nil
}
