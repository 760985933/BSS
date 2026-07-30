package services

import (
	"context"
	"time"

	"bss/internal/models"

	"gorm.io/gorm"
)

// AuditQueryService 审计查询（M2-4）：按实体/操作/时间检索 audit_logs。
// 审计日志为全局合规记录，不做行级数据范围过滤，仅对管理/监督角色开放（路由层 RequireRole 把关）。
type AuditQueryService struct {
	db *gorm.DB
}

func NewAuditQueryService(db *gorm.DB) *AuditQueryService {
	return &AuditQueryService{db: db}
}

// AuditQuery 审计查询条件
type AuditQuery struct {
	EntityType string
	EntityID   uint
	Action     string // create/update/delete/transfer/status_change/offboard
	OperatorID uint
	Start      string // YYYY-MM-DD
	End        string // YYYY-MM-DD
	Page       int
	Size       int
}

// List 分页查询审计日志（按时间倒序）。
func (s *AuditQueryService) List(ctx context.Context, q AuditQuery) ([]models.AuditLog, int64, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Size < 1 || q.Size > 100 {
		q.Size = 20
	}
	db := s.db.WithContext(ctx).Model(&models.AuditLog{})
	if q.EntityType != "" {
		db = db.Where("entity_type = ?", q.EntityType)
	}
	if q.EntityID != 0 {
		db = db.Where("entity_id = ?", q.EntityID)
	}
	if q.Action != "" {
		db = db.Where("action = ?", q.Action)
	}
	if q.OperatorID != 0 {
		db = db.Where("operator_id = ?", q.OperatorID)
	}
	if q.Start != "" {
		db = db.Where("created_at >= ?", q.Start)
	}
	if q.End != "" {
		// 含当天 23:59:59
		end := q.End + "T23:59:59"
		if _, err := time.Parse("2006-01-02T15:04:05", end); err == nil {
			db = db.Where("created_at <= ?", end)
		}
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []models.AuditLog
	if err := db.Order("created_at DESC, id DESC").
		Offset((q.Page - 1) * q.Size).Limit(q.Size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
