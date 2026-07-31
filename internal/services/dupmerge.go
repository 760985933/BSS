package services

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"bss/internal/models"
)

var (
	// ErrDupSameCustomer 不能与自身合并
	ErrDupSameCustomer = errors.New("不能与自身合并")
	// ErrDupCustomerMissing 主/从客户不存在或已删除
	ErrDupCustomerMissing = errors.New("客户不存在或已删除")
)

// DuplicateGroup 一组疑似重复客户：共享同一联系人手机或邮箱。
type DuplicateGroup struct {
	Field     string            `json:"field"` // phone | email
	Value     string            `json:"value"`
	Customers []models.Customer `json:"customers"`
}

// FindDuplicates 基于联系人手机/邮箱硬证据查找疑似重复客户。
// 说明：Customer.Name 有唯一约束，完全同名在 DB 层被拦，因此以"跨客户共享的联系方式"
// 作为重复证据，比名称模糊匹配更可靠。
func FindDuplicates(ctx context.Context, db *gorm.DB) ([]DuplicateGroup, error) {
	groups := []DuplicateGroup{}
	for _, field := range []string{"phone", "email"} {
		// 字段名来自白名单，可安全拼接；值通过参数无法注入列名
		q := fmt.Sprintf(`
			SELECT value AS val, customer_id AS cid FROM (
				SELECT %s AS value, customer_id FROM contacts
				WHERE %s <> '' AND deleted_at IS NULL
			) c WHERE c.value IN (
				SELECT %s FROM contacts
				WHERE %s <> '' AND deleted_at IS NULL
				GROUP BY %s HAVING COUNT(DISTINCT customer_id) >= 2
			)`, field, field, field, field, field)
		type row struct {
			Val string
			Cid uint
		}
		var rows []row
		if err := db.Raw(q).Scan(&rows).Error; err != nil {
			return nil, err
		}
		byVal := map[string][]uint{}
		for _, r := range rows {
			byVal[r.Val] = append(byVal[r.Val], r.Cid)
		}
		for val, cids := range byVal {
			seen := map[uint]bool{}
			uniq := []uint{}
			for _, c := range cids {
				if !seen[c] {
					seen[c] = true
					uniq = append(uniq, c)
				}
			}
			if len(uniq) < 2 {
				continue
			}
			var cs []models.Customer
			if err := db.Where("id IN ? AND deleted_at IS NULL", uniq).Find(&cs).Error; err != nil {
				return nil, err
			}
			if len(cs) < 2 {
				continue
			}
			groups = append(groups, DuplicateGroup{Field: field, Value: val, Customers: cs})
		}
	}
	return groups, nil
}

// MergeCustomers 将 secondary 客户并入 primary：
// 迁移其联系人/商单/合同后软删 secondary。
// 回款计划/记录仅挂在合同上，随合同 customer_id 变更自动跟随，无需单独处理。
// 该操作跨 owner，仅限 admin 调用。
func MergeCustomers(ctx context.Context, db *gorm.DB, primaryID, secondaryID uint) error {
	if primaryID == secondaryID {
		return ErrDupSameCustomer
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var primary, secondary models.Customer
		if err := tx.Where("id = ? AND deleted_at IS NULL", primaryID).First(&primary).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrDupCustomerMissing
			}
			return err
		}
		if err := tx.Where("id = ? AND deleted_at IS NULL", secondaryID).First(&secondary).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrDupCustomerMissing
			}
			return err
		}
		// 降级从客户的 primary 联系人，避免主客户出现多个首要联系人
		if err := tx.Model(&models.Contact{}).
			Where("customer_id = ? AND is_primary = 1", secondaryID).
			Update("is_primary", false).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Contact{}).
			Where("customer_id = ? AND deleted_at IS NULL", secondaryID).
			Update("customer_id", primaryID).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Deal{}).
			Where("customer_id = ? AND deleted_at IS NULL", secondaryID).
			Update("customer_id", primaryID).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Contract{}).
			Where("customer_id = ? AND deleted_at IS NULL", secondaryID).
			Update("customer_id", primaryID).Error; err != nil {
			return err
		}
		// 软删从客户（触发审计 delete 钩子）
		if err := tx.Delete(&secondary).Error; err != nil {
			return err
		}
		return nil
	})
}
