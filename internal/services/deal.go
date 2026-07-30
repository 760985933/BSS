package services

import (
	"context"
	"errors"

	"bss/internal/models"
	"bss/internal/pkg/code"

	"gorm.io/gorm"
)

var (
	ErrDealMissing        = errors.New("商单不存在")
	ErrInvalidTransition  = errors.New("非法的状态流转")
	ErrLostReasonRequired = errors.New("输单必须填写输单原因")
	ErrDealLocked         = errors.New("商单已关闭（won/lost），关键字段只读")
	ErrDealHasContract    = errors.New("该商单已关联合同，禁止删除")
	ErrDealClosed         = errors.New("商单已关闭，禁止删除（历史保留）")
)

// 商单状态机（TECH_DESIGN §6.2：对齐 SF 6 态 + 回退边；won/lost 为终态）
// negotiating 之后如需折扣审批，先进入 pending_approval（M2-1），审批通过后由审批服务推进至 won
var dealFlow = map[string][]string{
	models.DealProspecting:      {models.DealQualifying, models.DealLost},
	models.DealQualifying:       {models.DealProposal, models.DealProspecting, models.DealLost},
	models.DealProposal:         {models.DealNegotiating, models.DealQualifying, models.DealLost},
	models.DealNegotiating:      {models.DealPendingApproval, models.DealProposal, models.DealLost},
	models.DealPendingApproval:  {models.DealNegotiating}, // 仅审批驳回/撤回回到 negotiating；won 由审批服务推进
}

// 各阶段默认赢单概率（PRD §4.4）
var dealStageProbability = map[string]int{
	models.DealProspecting: 10,
	models.DealQualifying:  25,
	models.DealProposal:    60,
	models.DealNegotiating: 80,
	models.DealWon:         100,
	models.DealLost:        0,
}

// 输单原因枚举（PRD §4.4）
var validLostReasons = map[string]bool{
	"no_purchase": true, "competitor": true, "budget": true, "qualified_out": true, "other": true,
}

// 退出标准软校验适用阶段（PRD §4.4：进入报价及以后金额应 > 0）
var exitCheckStages = map[string]bool{
	models.DealProposal: true, models.DealNegotiating: true, models.DealWon: true,
}

// DealClosed 判定终态
func DealClosed(status string) bool {
	return status == models.DealWon || status == models.DealLost
}

type DealService struct {
	db      *gorm.DB
	codeGen *code.Generator
}

func NewDealService(db *gorm.DB) *DealService {
	return &DealService{db: db, codeGen: code.NewGenerator(db)}
}

// DealInput 商单可写字段（终态锁定后仅 remark 可改）
type DealInput struct {
	CustomerID       uint   `json:"customer_id,string"`
	Title            string `json:"title"`
	AmountCent       int64  `json:"amount_cent"`
	Probability      *int   `json:"probability"` // 可空=按阶段默认
	ExpectedSignDate string `json:"expected_sign_date"`
	Remark           string `json:"remark"`
}

// Create 新建商单：SD 单号、默认 prospecting/10%
func (s *DealService) Create(ctx context.Context, in DealInput, ownerID uint) (*models.Deal, error) {
	if in.Title == "" {
		return nil, errors.New("商单标题不能为空")
	}
	if in.CustomerID == 0 {
		return nil, errors.New("必须关联客户")
	}
	var cnt int64
	if err := s.db.WithContext(ctx).Model(&models.Customer{}).Where("id = ?", in.CustomerID).Count(&cnt).Error; err != nil {
		return nil, err
	}
	if cnt == 0 {
		return nil, errors.New("关联客户不存在")
	}
	if in.AmountCent < 0 {
		return nil, errors.New("金额不能为负数")
	}
	c, err := s.codeGen.Next(ctx, code.PrefixDeal)
	if err != nil {
		return nil, err
	}
	prob := dealStageProbability[models.DealProspecting]
	if in.Probability != nil {
		if *in.Probability < 0 || *in.Probability > 100 {
			return nil, errors.New("概率须在 0-100 之间")
		}
		prob = *in.Probability
	}
	d := models.Deal{
		Code: c, CustomerID: in.CustomerID, Title: in.Title, AmountCent: in.AmountCent,
		Probability: prob, ExpectedSignDate: in.ExpectedSignDate,
		Status: models.DealProspecting, OwnerID: ownerID, Remark: in.Remark,
	}
	if err := s.db.WithContext(ctx).Create(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

// Update 编辑商单；终态锁定关键字段（PRD §7.4：仅 remark 可改）
func (s *DealService) Update(ctx context.Context, id uint, in DealInput) error {
	var d models.Deal
	if err := s.db.WithContext(ctx).Take(&d, id).Error; err != nil {
		return ErrDealMissing
	}
	if DealClosed(d.Status) {
		// 终态：除 remark 外任何字段变化都拒绝
		if in.Title != d.Title || in.AmountCent != d.AmountCent || in.CustomerID != d.CustomerID {
			return ErrDealLocked
		}
		return s.db.WithContext(ctx).Model(&d).Update("remark", in.Remark).Error
	}
	if in.Title == "" {
		return errors.New("商单标题不能为空")
	}
	if in.AmountCent < 0 {
		return errors.New("金额不能为负数")
	}
	updates := map[string]any{
		"title": in.Title, "amount_cent": in.AmountCent,
		"expected_sign_date": in.ExpectedSignDate, "remark": in.Remark,
	}
	if in.Probability != nil {
		if *in.Probability < 0 || *in.Probability > 100 {
			return errors.New("概率须在 0-100 之间")
		}
		updates["probability"] = *in.Probability
	}
	return s.db.WithContext(ctx).Model(&d).Updates(updates).Error
}

// ErrExitWarning 退出标准软校验未满足（前端弹确认后带 force 重发）
var ErrExitWarning = errors.New("exit_warning")

// ChangeStatus 状态流转唯一入口；返回 ErrExitWarning 表示软校验提示（可 force）
func (s *DealService) ChangeStatus(ctx context.Context, id uint, to, lostReason string, force bool) (*models.Deal, error) {
	var d models.Deal
	if err := s.db.WithContext(ctx).Take(&d, id).Error; err != nil {
		return nil, ErrDealMissing
	}
	if d.Status == to {
		return nil, errors.New("目标状态与当前状态相同")
	}
	legal := false
	for _, next := range dealFlow[d.Status] {
		if next == to {
			legal = true
			break
		}
	}
	if !legal {
		return nil, ErrInvalidTransition
	}
	updates := map[string]any{"status": to}
	if to == models.DealLost {
		if !validLostReasons[lostReason] {
			return nil, ErrLostReasonRequired
		}
		updates["lost_reason"] = lostReason
	}
	// 概率：切换到任何阶段都按目标阶段重置默认（回退同理），终端用户可再手调
	if p, ok := dealStageProbability[to]; ok {
		updates["probability"] = p
	}
	// 退出标准软校验：进入报价/谈判/赢单时金额应 > 0
	if exitCheckStages[to] && d.AmountCent == 0 && !force {
		return nil, ErrExitWarning
	}
	if err := s.db.WithContext(ctx).Model(&d).Updates(updates).Error; err != nil {
		return nil, err
	}
	d.Status = to
	if p, ok := updates["probability"]; ok {
		d.Probability = p.(int)
	}
	return &d, nil
}

// ApplyDiscountAndWin 审批通过后的折扣赢单推进（仅能从 pending_approval 进入 won，并落定折扣金额）；
// 普通 ChangeStatus 不允许 pending_approval→won，强制折扣须经审批流。
func (s *DealService) ApplyDiscountAndWin(ctx context.Context, id uint, discountCent int64) error {
	var d models.Deal
	if err := s.db.WithContext(ctx).Take(&d, id).Error; err != nil {
		return ErrDealMissing
	}
	if d.Status != models.DealPendingApproval {
		return errors.New("商单未处于待审批(pending_approval)状态，无法赢单")
	}
	if discountCent < 0 {
		return errors.New("折扣金额不能为负")
	}
	return s.db.WithContext(ctx).Model(&d).Updates(map[string]any{
		"status":               models.DealWon,
		"discount_amount_cent": discountCent,
	}).Error
}

// Delete 软删除；终态保留历史、已关联合同禁删
func (s *DealService) Delete(ctx context.Context, id uint) error {
	var d models.Deal
	if err := s.db.WithContext(ctx).Take(&d, id).Error; err != nil {
		return ErrDealMissing
	}
	if DealClosed(d.Status) {
		return ErrDealClosed
	}
	var cnt int64
	if err := s.db.WithContext(ctx).Model(&models.DealContract{}).Where("deal_id = ?", id).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt > 0 {
		return ErrDealHasContract
	}
	return s.db.WithContext(ctx).Delete(&d).Error
}

// Get 详情（含客户、负责人）
func (s *DealService) Get(ctx context.Context, id uint) (*models.Deal, error) {
	var d models.Deal
	if err := s.db.WithContext(ctx).Preload("Customer").Preload("Owner").Take(&d, id).Error; err != nil {
		return nil, ErrDealMissing
	}
	return &d, nil
}

// OwnerOf 归属辅助（行级权限校验用）
func (s *DealService) OwnerOf(ctx context.Context, id uint) (uint, error) {
	var d models.Deal
	if err := s.db.WithContext(ctx).Select("owner_id").Take(&d, id).Error; err != nil {
		return 0, ErrDealMissing
	}
	return d.OwnerID, nil
}

// Forecast 加权预测（PRD §4.4：进行中商单 Σ(金额×概率)），数据范围由调用方 ScopeOwner 注入
type ForecastResult struct {
	OpenCount     int64 `json:"open_count"`
	TotalCent     int64 `json:"total_cent"`     // 进行中商单金额合计
	WeightedCent  int64 `json:"weighted_cent"`  // 加权预测金额
}

func (s *DealService) Forecast(ctx context.Context, scoped *gorm.DB) (*ForecastResult, error) {
	var r ForecastResult
	err := scoped.WithContext(ctx).Model(&models.Deal{}).
		Where("status NOT IN (?, ?)", models.DealWon, models.DealLost).
		Select("COUNT(*) AS open_count, COALESCE(SUM(amount_cent),0) AS total_cent, COALESCE(SUM(amount_cent * probability / 100),0) AS weighted_cent").
		Scan(&r).Error
	return &r, err
}
