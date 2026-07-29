package services

import (
	"context"

	"bss/internal/middleware"
	"bss/internal/models"

	"gorm.io/gorm"
)

// ScopeOwner 数据范围过滤（TECH_DESIGN §6.5，PRD §6 权限矩阵）。
// 业务表（customers/deals/contracts 等含 owner_id 的表）统一走此函数：
//   - admin / finance / hr：不过滤（看全部）
//   - sales：仅本人（owner_id = 当前用户）
//   - sales_lead：本部门（owner 属于同部门员工）
//
// 注意：这里只约束"看/改谁的数据"；能不能操作由 middleware.RequireRole 把关。
func ScopeOwner(db *gorm.DB, ctx context.Context) *gorm.DB {
	c := middleware.UserFrom(ctx)
	if c == nil {
		return db.Where("1 = 0") // 未登录兜底：查不到任何数据
	}
	switch c.Role {
	case models.RoleAdmin, models.RoleFinance, models.RoleHR:
		return db
	case models.RoleSalesLead:
		return db.Where("owner_id IN (SELECT id FROM employees WHERE dept = ? AND deleted_at IS NULL)", c.Dept)
	default: // sales
		return db.Where("owner_id = ?", c.UserID)
	}
}

// CanAccessOwner 单条记录级校验：当前用户是否有权访问该 owner 的数据（写操作前调用）。
// 返回错误表示数据范围校验本身失败（DB 异常），调用方应按 500 处理而非静默放行/拒绝。
func CanAccessOwner(db *gorm.DB, ctx context.Context, ownerID uint) (bool, error) {
	c := middleware.UserFrom(ctx)
	if c == nil {
		return false, nil
	}
	switch c.Role {
	case models.RoleAdmin, models.RoleFinance, models.RoleHR:
		return true, nil
	case models.RoleSalesLead:
		var cnt int64
		if err := db.Model(&models.Employee{}).Where("id = ? AND dept = ?", ownerID, c.Dept).Count(&cnt).Error; err != nil {
			return false, err
		}
		return cnt > 0, nil
	default:
		return ownerID == c.UserID, nil
	}
}
