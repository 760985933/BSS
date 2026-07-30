package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"bss/internal/actor"
	"bss/internal/models"

	"gorm.io/gorm"
)

var (
	ErrPlanMissing        = errors.New("回款计划不存在")
	ErrPlanAmountExceed   = errors.New("回款计划总额已超过合同金额")
	ErrPlanLocked         = errors.New("该计划已核销（partial/paid），禁止修改或删除，请新增调整期")
	ErrPlanInvalid        = errors.New("回款计划参数非法")
	ErrRecordMissing      = errors.New("回款记录不存在")
	ErrRecordInvalid      = errors.New("回款记录参数非法")
	ErrRecordPlanMismatch = errors.New("回款记录关联的期次不属于该合同")
)

// parseIDStr 将字符串 ID（前端 JSON 统一为字符串）解析为 uint
func parseIDStr(s string) (uint, error) {
	var n uint64
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return 0, err
	}
	return uint(n), nil
}

// PlanInput 回款计划可写字段（contract_id 由路由注入，不从 body 信任）
type PlanInput struct {
	ContractID uint   `json:"-"`
	PeriodNo   int    `json:"period_no"`
	DueDate    string `json:"due_date"`
	AmountCent int64  `json:"amount_cent"`
}

// RecordInput 回款记录可写字段（plan_id 为字符串以便与前端一致，service 内转 uint）
type RecordInput struct {
	PlanID    *string `json:"plan_id"` // 可空=不核销计划
	AmountCent int64  `json:"amount_cent"`
	PaidAt    string  `json:"paid_at"`
	Method    string  `json:"method"`
	Remark    string  `json:"remark"`
}

// PlanView 计划视图：在模型基础上派生 is_overdue（逾期不持久化）
type PlanView struct {
	models.PaymentPlan
	IsOverdue bool `json:"is_overdue"`
}

// PaymentSummary 合同维度回款汇总
type PaymentSummary struct {
	ReceivableCent int64 `json:"receivable_cent"` // 应收（合同额）
	ReceivedCent   int64 `json:"received_cent"`   // 已收（记录累计）
	BalanceCent    int64 `json:"balance_cent"`    // 余额（应收-已收）
	OverdueCent    int64 `json:"overdue_cent"`    // 逾期额（逾期未核销部分）
}

// PaymentService 回款计划/记录/汇总（TECH_DESIGN §回款；PRD §回款管理）
type PaymentService struct {
	db *gorm.DB
}

func NewPaymentService(db *gorm.DB) *PaymentService {
	return &PaymentService{db: db}
}

// todayStr UTC 日期字符串（用于逾期比较，due_date 同为 YYYY-MM-DD 可字典序比较）
func todayStr() string { return time.Now().UTC().Format("2006-01-02") }

func planIsOverdue(due string) bool { return due != "" && due < todayStr() }

// contractAmount 取合同金额与归属人（用于总额校验与数据范围）
func (s *PaymentService) contractAmount(ctx context.Context, contractID uint) (int64, uint, error) {
	var c models.Contract
	if err := s.db.WithContext(ctx).Select("amount_cent", "owner_id").Take(&c, contractID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, 0, ErrContractMissing
		}
		return 0, 0, err
	}
	return c.AmountCent, c.OwnerID, nil
}

// sumPlanAmount 已存在计划金额合计（可排除某个 planID，用于编辑时重算）
func (s *PaymentService) sumPlanAmount(ctx context.Context, contractID, excludeID uint) (int64, error) {
	var sum int64
	q := s.db.WithContext(ctx).Model(&models.PaymentPlan{}).Where("contract_id = ?", contractID)
	if excludeID != 0 {
		q = q.Where("id <> ?", excludeID)
	}
	if err := q.Select("COALESCE(SUM(amount_cent),0)").Scan(&sum).Error; err != nil {
		return 0, err
	}
	return sum, nil
}

// CreatePlan 新建回款计划：总额 ≤ 合同额
func (s *PaymentService) CreatePlan(ctx context.Context, in PlanInput) (*models.PaymentPlan, error) {
	if in.DueDate == "" {
		return nil, errors.New("到期日不能为空")
	}
	if in.AmountCent <= 0 {
		return nil, errors.New("计划金额必须为正数")
	}
	if in.PeriodNo <= 0 {
		return nil, errors.New("期次必须为正整数")
	}
	contractAmt, _, err := s.contractAmount(ctx, in.ContractID)
	if err != nil {
		return nil, err
	}
	existing, err := s.sumPlanAmount(ctx, in.ContractID, 0)
	if err != nil {
		return nil, err
	}
	if existing+in.AmountCent > contractAmt {
		return nil, ErrPlanAmountExceed
	}
	p := &models.PaymentPlan{
		ContractID: in.ContractID, PeriodNo: in.PeriodNo,
		DueDate: in.DueDate, AmountCent: in.AmountCent, Status: models.PlanPending,
	}
	if err := s.db.WithContext(ctx).Create(p).Error; err != nil {
		return nil, err
	}
	return p, nil
}

// UpdatePlan 编辑计划：仅 pending（无核销）可改；重算总额上限
func (s *PaymentService) UpdatePlan(ctx context.Context, id, contractID uint, in PlanInput) error {
	var p models.PaymentPlan
	if err := s.db.WithContext(ctx).Take(&p, id).Error; err != nil {
		return ErrPlanMissing
	}
	if p.ContractID != contractID {
		return ErrPlanMissing
	}
	if p.Status != models.PlanPending {
		return ErrPlanLocked
	}
	if in.DueDate == "" {
		return errors.New("到期日不能为空")
	}
	if in.AmountCent <= 0 {
		return errors.New("计划金额必须为正数")
	}
	if in.PeriodNo <= 0 {
		return errors.New("期次必须为正整数")
	}
	contractAmt, _, err := s.contractAmount(ctx, contractID)
	if err != nil {
		return err
	}
	existing, err := s.sumPlanAmount(ctx, contractID, id)
	if err != nil {
		return err
	}
	if existing+in.AmountCent > contractAmt {
		return ErrPlanAmountExceed
	}
	return s.db.WithContext(ctx).Model(&p).Updates(map[string]any{
		"period_no":   in.PeriodNo,
		"due_date":    in.DueDate,
		"amount_cent": in.AmountCent,
	}).Error
}

// DeletePlan 删除计划：仅 pending（无核销）可删
func (s *PaymentService) DeletePlan(ctx context.Context, id, contractID uint) error {
	var p models.PaymentPlan
	if err := s.db.WithContext(ctx).Take(&p, id).Error; err != nil {
		return ErrPlanMissing
	}
	if p.ContractID != contractID {
		return ErrPlanMissing
	}
	if p.Status != models.PlanPending {
		return ErrPlanLocked
	}
	return s.db.WithContext(ctx).Delete(&p).Error
}

// ListPlans 列表（含 is_overdue 派生）
func (s *PaymentService) ListPlans(ctx context.Context, contractID uint) ([]PlanView, error) {
	var list []models.PaymentPlan
	if err := s.db.WithContext(ctx).Where("contract_id = ?", contractID).
		Order("period_no ASC, id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	out := make([]PlanView, 0, len(list))
	for _, p := range list {
		out = append(out, PlanView{PaymentPlan: p, IsOverdue: planIsOverdue(p.DueDate) && p.Status != models.PlanPaid})
	}
	return out, nil
}

// ListRecords 回款记录列表
func (s *PaymentService) ListRecords(ctx context.Context, contractID uint) ([]models.PaymentRecord, error) {
	var list []models.PaymentRecord
	if err := s.db.WithContext(ctx).Where("contract_id = ?", contractID).
		Order("paid_at DESC, id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// recomputePlanStatus 依据已核销金额重算计划状态（pending/partial/paid）
func (s *PaymentService) recomputePlanStatus(ctx context.Context, tx *gorm.DB, planID uint) error {
	var plan models.PaymentPlan
	if err := tx.WithContext(ctx).Take(&plan, planID).Error; err != nil {
		return err
	}
	var paid int64
	if err := tx.WithContext(ctx).Model(&models.PaymentRecord{}).
		Where("plan_id = ?", planID).Select("COALESCE(SUM(amount_cent),0)").Scan(&paid).Error; err != nil {
		return err
	}
	status := models.PlanPending
	switch {
	case paid >= plan.AmountCent:
		status = models.PlanPaid
	case paid > 0:
		status = models.PlanPartial
	}
	return tx.WithContext(ctx).Model(&plan).Update("status", status).Error
}

// CreateRecords 批量录入回款记录；关联计划时校验同合同并自动推进计划状态
func (s *PaymentService) CreateRecords(ctx context.Context, contractID uint, in []RecordInput, operatorID uint) error {
	if len(in) == 0 {
		return errors.New("请至少录入一条回款")
	}
	affected := map[uint]bool{}
	for _, r := range in {
		if r.AmountCent <= 0 {
			return ErrRecordInvalid
		}
		if r.PaidAt == "" {
			return ErrRecordInvalid
		}
		if r.PlanID != nil && *r.PlanID != "" {
			pid, err := parseIDStr(*r.PlanID)
			if err != nil {
				return ErrRecordInvalid
			}
			var p models.PaymentPlan
			if err := s.db.WithContext(ctx).Take(&p, pid).Error; err != nil {
				return ErrRecordPlanMismatch
			}
			if p.ContractID != contractID {
				return ErrRecordPlanMismatch
			}
			affected[pid] = true
		}
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ctx2 := actor.WithActor(ctx, operatorID)
		for _, r := range in {
			rec := models.PaymentRecord{ContractID: contractID, AmountCent: r.AmountCent, PaidAt: r.PaidAt, Method: r.Method, Remark: r.Remark, CreatedBy: operatorID}
			if r.PlanID != nil && *r.PlanID != "" {
				if pid, err := parseIDStr(*r.PlanID); err == nil {
					rec.PlanID = &pid
				}
			}
			if err := tx.WithContext(ctx2).Create(&rec).Error; err != nil {
				return err
			}
		}
		for pid := range affected {
			if err := s.recomputePlanStatus(ctx2, tx, pid); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteRecord 删除回款记录（财务操作），并重算关联计划状态
func (s *PaymentService) DeleteRecord(ctx context.Context, id, contractID, operatorID uint) error {
	var rec models.PaymentRecord
	if err := s.db.WithContext(ctx).Take(&rec, id).Error; err != nil {
		return ErrRecordMissing
	}
	if rec.ContractID != contractID {
		return ErrRecordMissing
	}
	pid := rec.PlanID
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ctx2 := actor.WithActor(ctx, operatorID)
		if err := tx.WithContext(ctx2).Delete(&rec).Error; err != nil {
			return err
		}
		if pid != nil {
			if err := s.recomputePlanStatus(ctx2, tx, *pid); err != nil {
				return err
			}
		}
		return nil
	})
}

// Summary 合同维度回款汇总（应收/已收/余额/逾期额）
func (s *PaymentService) Summary(ctx context.Context, contractID uint) (PaymentSummary, error) {
	var sum PaymentSummary
	contractAmt, _, err := s.contractAmount(ctx, contractID)
	if err != nil {
		return sum, err
	}
	sum.ReceivableCent = contractAmt

	var received int64
	if err := s.db.WithContext(ctx).Model(&models.PaymentRecord{}).
		Where("contract_id = ?", contractID).Select("COALESCE(SUM(amount_cent),0)").Scan(&received).Error; err != nil {
		return sum, err
	}
	sum.ReceivedCent = received
	sum.BalanceCent = contractAmt - received

	// 逾期未核销部分：逐计划计算（仅统计关联计划的核销进度）
	var plans []models.PaymentPlan
	if err := s.db.WithContext(ctx).Where("contract_id = ?", contractID).Find(&plans).Error; err != nil {
		return sum, err
	}
	for _, p := range plans {
		if !planIsOverdue(p.DueDate) || p.Status == models.PlanPaid {
			continue
		}
		var paid int64
		if err := s.db.WithContext(ctx).Model(&models.PaymentRecord{}).
			Where("plan_id = ?", p.ID).Select("COALESCE(SUM(amount_cent),0)").Scan(&paid).Error; err != nil {
			return sum, err
		}
		remain := p.AmountCent - paid
		if remain > 0 {
			sum.OverdueCent += remain
		}
	}
	return sum, nil
}
