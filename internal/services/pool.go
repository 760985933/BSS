package services

import (
	"context"
	"errors"
	"time"

	"bss/internal/middleware"
	"bss/internal/models"

	"gorm.io/gorm"
)

var (
	ErrNotInPool        = errors.New("该客户已有负责人，无法领取")
	ErrAlreadyInPool    = errors.New("该客户已在公海中")
	ErrClaimLimit       = errors.New("名下客户数已达上限，请先释放部分客户再领取")
	ErrReleaseHasDeal   = errors.New("该客户存在进行中的商单，不能释放到公海")
	ErrReleaseHasContra = errors.New("该客户存在有效合同，不能释放到公海")
	ErrPoolSettings     = errors.New("公海规则不存在")
)

// 进行中商单：未赢单也未丢单
var openDealStatuses = []string{
	models.DealProspecting, models.DealQualifying, models.DealProposal,
	models.DealNegotiating, models.DealPendingApproval,
}

// 有效合同：未取消/终止/到期
var deadContractStatuses = []string{
	models.ContractCancelled, models.ContractTerminated, models.ContractExpired,
}

type PoolService struct {
	db *gorm.DB
}

func NewPoolService(db *gorm.DB) *PoolService {
	return &PoolService{db: db}
}

// ---------- 规则 ----------

// Settings 读取公海规则；单行表缺失时返回内置默认值（兼容未跑迁移的测试库）
func (s *PoolService) Settings(ctx context.Context) (*models.PoolSettings, error) {
	var st models.PoolSettings
	err := s.db.WithContext(ctx).Take(&st, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &models.PoolSettings{
			ID: 1, Enabled: false, MaxClaimPerSales: 50,
			IdleDaysNoFollow: 30, IdleDaysNoDeal: 60, ProtectDays: 7,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return &st, nil
}

// PoolSettingsInput 规则可写字段
type PoolSettingsInput struct {
	Enabled          bool `json:"enabled"`
	MaxClaimPerSales int  `json:"max_claim_per_sales"`
	IdleDaysNoFollow int  `json:"idle_days_no_follow"`
	IdleDaysNoDeal   int  `json:"idle_days_no_deal"`
	ProtectDays      int  `json:"protect_days"`
}

func (s *PoolService) UpdateSettings(ctx context.Context, in PoolSettingsInput) (*models.PoolSettings, error) {
	if in.MaxClaimPerSales < 0 || in.IdleDaysNoFollow < 0 || in.IdleDaysNoDeal < 0 || in.ProtectDays < 0 {
		return nil, errors.New("规则天数与上限不能为负数")
	}
	st := models.PoolSettings{
		ID: 1, Enabled: in.Enabled, MaxClaimPerSales: in.MaxClaimPerSales,
		IdleDaysNoFollow: in.IdleDaysNoFollow, IdleDaysNoDeal: in.IdleDaysNoDeal,
		ProtectDays: in.ProtectDays, UpdatedAt: time.Now().UTC(),
	}
	if err := s.db.WithContext(ctx).Save(&st).Error; err != nil {
		return nil, err
	}
	return &st, nil
}

// ---------- 列表 ----------

// PoolFilter 公海列表筛选
type PoolFilter struct {
	Keyword  string
	Industry string
	Source   string
	Level    string
	Page     int
	Size     int
}

// List 公海客户（owner_id = 0）。公海对所有登录用户可见，不套 ScopeOwner。
func (s *PoolService) List(ctx context.Context, f PoolFilter) ([]models.Customer, int64, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Size < 1 || f.Size > 100 {
		f.Size = 20
	}
	q := s.db.WithContext(ctx).Model(&models.Customer{}).Where("owner_id = 0")
	if f.Keyword != "" {
		q = q.Where("name LIKE ?", "%"+f.Keyword+"%")
	}
	if f.Industry != "" {
		q = q.Where("industry = ?", f.Industry)
	}
	if f.Source != "" {
		q = q.Where("source = ?", f.Source)
	}
	if f.Level != "" {
		q = q.Where("level = ?", f.Level)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []models.Customer
	err := q.Order("updated_at DESC, id DESC").
		Offset((f.Page - 1) * f.Size).Limit(f.Size).Find(&list).Error
	return list, total, err
}

// Logs 某客户的公海流水（倒序）
func (s *PoolService) Logs(ctx context.Context, customerID uint) ([]models.CustomerPoolLog, error) {
	var list []models.CustomerPoolLog
	err := s.db.WithContext(ctx).Where("customer_id = ?", customerID).
		Order("id DESC").Find(&list).Error
	return list, err
}

// ---------- 领取 / 释放 ----------

// Claim 从公海领取客户：必须无主，且不超过领取上限。
func (s *PoolService) Claim(ctx context.Context, customerID, userID uint) error {
	st, err := s.Settings(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cust models.Customer
		if err := tx.Take(&cust, customerID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCustomerMissing
			}
			return err
		}
		if cust.OwnerID != 0 {
			return ErrNotInPool
		}
		if st.MaxClaimPerSales > 0 {
			var owned int64
			if err := tx.Model(&models.Customer{}).Where("owner_id = ?", userID).Count(&owned).Error; err != nil {
				return err
			}
			if owned >= int64(st.MaxClaimPerSales) {
				return ErrClaimLimit
			}
		}
		if err := tx.Model(&models.Customer{}).Where("id = ?", customerID).Updates(map[string]any{
			"owner_id":         userID,
			"claimed_at":       now,
			"last_followed_at": now,
			"pool_reason":      "",
		}).Error; err != nil {
			return err
		}
		return tx.Create(&models.CustomerPoolLog{
			CustomerID: customerID, Action: models.PoolActionClaim,
			FromOwnerID: 0, ToOwnerID: userID, OperatorID: userID,
			Reason: "从公海领取", CreatedAt: now,
		}).Error
	})
}

// Release 释放客户到公海：存在进行中商单或有效合同时禁止（防止在途业务失去归属）。
func (s *PoolService) Release(ctx context.Context, customerID, operatorID uint, reason string) error {
	if reason == "" {
		reason = models.PoolReasonRelease
	}
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cust models.Customer
		if err := tx.Take(&cust, customerID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCustomerMissing
			}
			return err
		}
		if cust.OwnerID == 0 {
			return ErrAlreadyInPool
		}
		blocked, err := hasOpenBusiness(tx, customerID)
		if err != nil {
			return err
		}
		if blocked != nil {
			return blocked
		}
		if err := tx.Model(&models.Customer{}).Where("id = ?", customerID).Updates(map[string]any{
			"owner_id":    0,
			"claimed_at":  nil,
			"pool_reason": reason,
		}).Error; err != nil {
			return err
		}
		return tx.Create(&models.CustomerPoolLog{
			CustomerID: customerID, Action: models.PoolActionRelease,
			FromOwnerID: cust.OwnerID, ToOwnerID: 0, OperatorID: operatorID,
			Reason: reason, CreatedAt: now,
		}).Error
	})
}

// Assign 管理员从公海直接指派给某销售（跳过领取上限，用于主管分配线索）
func (s *PoolService) Assign(ctx context.Context, customerID, toOwnerID, operatorID uint) error {
	var cnt int64
	if err := s.db.WithContext(ctx).Model(&models.Employee{}).
		Where("id = ? AND status = 'active'", toOwnerID).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt == 0 {
		return errors.New("目标负责人不存在或已停用")
	}
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cust models.Customer
		if err := tx.Take(&cust, customerID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCustomerMissing
			}
			return err
		}
		if err := tx.Model(&models.Customer{}).Where("id = ?", customerID).Updates(map[string]any{
			"owner_id":         toOwnerID,
			"claimed_at":       now,
			"last_followed_at": now,
			"pool_reason":      "",
		}).Error; err != nil {
			return err
		}
		return tx.Create(&models.CustomerPoolLog{
			CustomerID: customerID, Action: models.PoolActionAssign,
			FromOwnerID: cust.OwnerID, ToOwnerID: toOwnerID, OperatorID: operatorID,
			Reason: "管理员指派", CreatedAt: now,
		}).Error
	})
}

// hasOpenBusiness 客户是否存在在途业务；有则返回对应业务错误，无则返回 nil
func hasOpenBusiness(tx *gorm.DB, customerID uint) (error, error) {
	var cnt int64
	if err := tx.Model(&models.Deal{}).
		Where("customer_id = ? AND status IN ?", customerID, openDealStatuses).
		Count(&cnt).Error; err != nil {
		return nil, err
	}
	if cnt > 0 {
		return ErrReleaseHasDeal, nil
	}
	if err := tx.Model(&models.Contract{}).
		Where("customer_id = ? AND status NOT IN ?", customerID, deadContractStatuses).
		Count(&cnt).Error; err != nil {
		return nil, err
	}
	if cnt > 0 {
		return ErrReleaseHasContra, nil
	}
	return nil, nil
}

// ---------- 回收 ----------

// RecycleItem 单条回收明细
type RecycleItem struct {
	CustomerID uint   `json:"customer_id,string"`
	Name       string `json:"name"`
	OwnerID    uint   `json:"owner_id,string"`
	Reason     string `json:"reason"`
}

// RecycleResult 回收结果
type RecycleResult struct {
	Total int           `json:"total"`
	Items []RecycleItem `json:"items"`
}

// Recycle 按规则回收超时客户到公海。dryRun=true 时只返回候选、不落库。
// 豁免：有进行中商单、有有效合同、仍在保护期内的客户不回收。
func (s *PoolService) Recycle(ctx context.Context, now time.Time, dryRun bool) (*RecycleResult, error) {
	st, err := s.Settings(ctx)
	if err != nil {
		return nil, err
	}
	res := &RecycleResult{Items: []RecycleItem{}}
	if st.IdleDaysNoFollow <= 0 && st.IdleDaysNoDeal <= 0 {
		return res, nil // 两条规则都关掉，等于不回收
	}
	now = now.UTC()
	protectBefore := now.AddDate(0, 0, -st.ProtectDays)

	// 候选：有主 + 已过保护期 + 无在途商单 + 无有效合同
	q := s.db.WithContext(ctx).Model(&models.Customer{}).
		Where("owner_id <> 0").
		Where("claimed_at IS NOT NULL AND claimed_at <= ?", protectBefore).
		Where("NOT EXISTS (SELECT 1 FROM deals d WHERE d.customer_id = customers.id AND d.deleted_at IS NULL AND d.status IN ?)", openDealStatuses).
		Where("NOT EXISTS (SELECT 1 FROM contracts c WHERE c.customer_id = customers.id AND c.deleted_at IS NULL AND c.status NOT IN ?)", deadContractStatuses)

	conds := s.db.Session(&gorm.Session{NewDB: true})
	var or *gorm.DB
	if st.IdleDaysNoFollow > 0 {
		cutoff := now.AddDate(0, 0, -st.IdleDaysNoFollow)
		or = conds.Where("last_followed_at IS NULL OR last_followed_at <= ?", cutoff)
	}
	if st.IdleDaysNoDeal > 0 {
		cutoff := now.AddDate(0, 0, -st.IdleDaysNoDeal)
		noDeal := "claimed_at <= ? AND NOT EXISTS (SELECT 1 FROM deals d2 WHERE d2.customer_id = customers.id AND d2.deleted_at IS NULL)"
		if or == nil {
			or = conds.Where(noDeal, cutoff)
		} else {
			or = or.Or(noDeal, cutoff)
		}
	}
	q = q.Where(or)

	var candidates []models.Customer
	if err := q.Find(&candidates).Error; err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return res, nil
	}

	followCutoff := now.AddDate(0, 0, -st.IdleDaysNoFollow)
	ids := make([]uint, 0, len(candidates))
	logs := make([]models.CustomerPoolLog, 0, len(candidates))
	for _, c := range candidates {
		reason := models.PoolReasonNoDeal
		if st.IdleDaysNoFollow > 0 && (c.LastFollowedAt == nil || !c.LastFollowedAt.After(followCutoff)) {
			reason = models.PoolReasonNoFollow
		}
		res.Items = append(res.Items, RecycleItem{
			CustomerID: c.ID, Name: c.Name, OwnerID: c.OwnerID, Reason: reason,
		})
		ids = append(ids, c.ID)
		logs = append(logs, models.CustomerPoolLog{
			CustomerID: c.ID, Action: models.PoolActionRecycle,
			FromOwnerID: c.OwnerID, ToOwnerID: 0, OperatorID: operatorFrom(ctx),
			Reason: reason, CreatedAt: now,
		})
	}
	res.Total = len(ids)
	if dryRun {
		return res, nil
	}

	// 批量回收：单条 UPDATE 避免逐行审计噪音，轨迹由 pool_logs 承载
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range logs {
			if err := tx.Model(&models.Customer{}).Where("id = ?", logs[i].CustomerID).
				Updates(map[string]any{
					"owner_id":    0,
					"claimed_at":  nil,
					"pool_reason": logs[i].Reason,
				}).Error; err != nil {
				return err
			}
		}
		return tx.Create(&logs).Error
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// operatorFrom 取当前操作人；无登录上下文（定时任务）返回 0 表示系统
func operatorFrom(ctx context.Context) uint {
	if c := middleware.UserFrom(ctx); c != nil {
		return c.UserID
	}
	return 0
}

// ---------- 跟进时间 ----------

// TouchFollow 刷新客户最后跟进时间。供客户编辑、新增联系人、商单变更等业务动作调用。
// 静默失败：跟进时间是辅助字段，不应阻断主业务。
func TouchFollow(db *gorm.DB, ctx context.Context, customerID uint) {
	if customerID == 0 {
		return
	}
	db.WithContext(ctx).Model(&models.Customer{}).
		Where("id = ? AND owner_id <> 0", customerID).
		UpdateColumn("last_followed_at", time.Now().UTC())
}
