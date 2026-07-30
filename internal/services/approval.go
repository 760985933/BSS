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
	ErrApprovalMissing        = errors.New("审批单不存在")
	ErrApprovalInvalidKind    = errors.New("审批类型非法（仅支持 contract_sign / deal_discount）")
	ErrApprovalEntityState    = errors.New("审批对象未处于可提交审批的状态")
	ErrApprovalNotPending     = errors.New("审批单不处于待审状态，无法操作")
	ErrApprovalEntityMissing  = errors.New("审批对象不存在")
	ErrApprovalRejectReasonRequired = errors.New("驳回必须填写驳回原因")
)

// ApprovalService 审批流（M2-1）：合同签约审批、商单折扣审批
// 实体进入 pending_approval 后，只有本服务的 Approve/Reject 能将其推进，确保审批闸门有效。
type ApprovalService struct {
	db         *gorm.DB
	codeGen    *code.Generator
	contractSvc *ContractService
	dealSvc     *DealService
}

func NewApprovalService(db *gorm.DB) *ApprovalService {
	return &ApprovalService{
		db:         db,
		codeGen:    code.NewGenerator(db),
		contractSvc: NewContractService(db),
		dealSvc:     NewDealService(db),
	}
}

// ApprovalInput 审批单可写字段
type ApprovalInput struct {
	EntityType string `json:"entity_type"` // contract | deal
	EntityID   uint   `json:"entity_id,string"`
	Kind       string `json:"kind"` // contract_sign | deal_discount
	AmountCent int64  `json:"amount_cent"` // 商单折扣金额（分）
	Note       string `json:"note"`
}

// Create 提交审批：校验对象状态 → 推进对象到 pending_approval → 生成 SP 单号 → 落审批单(pending)
func (s *ApprovalService) Create(ctx context.Context, in ApprovalInput, applicantID uint) (*models.Approval, error) {
	if in.Kind != models.ApprovalContractSign && in.Kind != models.ApprovalDealDiscount {
		return nil, ErrApprovalInvalidKind
	}
	if in.EntityType != "contract" && in.EntityType != "deal" {
		return nil, errors.New("审批对象类型非法（仅支持 contract / deal）")
	}

	ap := &models.Approval{
		EntityType:  in.EntityType,
		EntityID:    in.EntityID,
		Kind:        in.Kind,
		Status:      models.ApprovalPending,
		ApplicantID: applicantID,
		Note:        in.Note,
	}

	// 推进对象到 pending_approval（校验前置状态）
	switch in.Kind {
	case models.ApprovalContractSign:
		var c models.Contract
		if err := s.db.WithContext(ctx).Take(&c, in.EntityID).Error; err != nil {
			return nil, ErrApprovalEntityMissing
		}
		if c.Status != models.ContractPending {
			return nil, ErrApprovalEntityState
		}
		if _, err := s.contractSvc.ChangeStatus(ctx, in.EntityID, models.ContractPendingApproval, ""); err != nil {
			return nil, err
		}
	case models.ApprovalDealDiscount:
		var d models.Deal
		if err := s.db.WithContext(ctx).Take(&d, in.EntityID).Error; err != nil {
			return nil, ErrApprovalEntityMissing
		}
		if d.Status != models.DealNegotiating {
			return nil, ErrApprovalEntityState
		}
		if in.AmountCent < 0 {
			return nil, errors.New("折扣金额不能为负")
		}
		ap.AmountCent = in.AmountCent
		if _, err := s.dealSvc.ChangeStatus(ctx, in.EntityID, models.DealPendingApproval, "", false); err != nil {
			return nil, err
		}
	}

	c, err := s.codeGen.Next(ctx, code.PrefixApproval)
	if err != nil {
		return nil, err
	}
	ap.Code = c
	if err := s.db.WithContext(ctx).Create(ap).Error; err != nil {
		return nil, err
	}
	return ap, nil
}

// Approve 审批通过：执行侧效（推进合同签约 / 商单折扣赢单）
func (s *ApprovalService) Approve(ctx context.Context, id uint, approverID uint) error {
	var ap models.Approval
	if err := s.db.WithContext(ctx).Take(&ap, id).Error; err != nil {
		return ErrApprovalMissing
	}
	if ap.Status != models.ApprovalPending {
		return ErrApprovalNotPending
	}
	// 侧效：推进关联实体
	switch ap.Kind {
	case models.ApprovalContractSign:
		if err := s.contractSvc.AdvanceToSigned(ctx, ap.EntityID); err != nil {
			return err
		}
	case models.ApprovalDealDiscount:
		if err := s.dealSvc.ApplyDiscountAndWin(ctx, ap.EntityID, ap.AmountCent); err != nil {
			return err
		}
	}
	ap.Status = models.ApprovalApproved
	ap.ApproverID = approverID
	return s.db.WithContext(ctx).Model(&ap).Updates(map[string]any{
		"status":       models.ApprovalApproved,
		"approver_id":  approverID,
	}).Error
}

// Reject 审批驳回：退回关联实体到前置状态，记录驳回原因
func (s *ApprovalService) Reject(ctx context.Context, id uint, approverID uint, reason string) error {
	var ap models.Approval
	if err := s.db.WithContext(ctx).Take(&ap, id).Error; err != nil {
		return ErrApprovalMissing
	}
	if ap.Status != models.ApprovalPending {
		return ErrApprovalNotPending
	}
	if strings.TrimSpace(reason) == "" {
		return ErrApprovalRejectReasonRequired
	}
	// 退回关联实体
	switch ap.Kind {
	case models.ApprovalContractSign:
		if _, err := s.contractSvc.ChangeStatus(ctx, ap.EntityID, models.ContractPending, ""); err != nil {
			return err
		}
	case models.ApprovalDealDiscount:
		if _, err := s.dealSvc.ChangeStatus(ctx, ap.EntityID, models.DealNegotiating, "", false); err != nil {
			return err
		}
	}
	return s.db.WithContext(ctx).Model(&ap).Updates(map[string]any{
		"status":        models.ApprovalRejected,
		"approver_id":   approverID,
		"reject_reason": reason,
	}).Error
}

// List 审批列表（含申请人/审批人姓名）
func (s *ApprovalService) List(ctx context.Context, page, size int, status, kind, entityType string) ([]models.Approval, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	base := s.db.WithContext(ctx).Model(&models.Approval{})
	if status != "" {
		base = base.Where("status = ?", status)
	}
	if kind != "" {
		base = base.Where("kind = ?", kind)
	}
	if entityType != "" {
		base = base.Where("entity_type = ?", entityType)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []models.Approval
	if err := base.Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// Get 审批详情
func (s *ApprovalService) Get(ctx context.Context, id uint) (*models.Approval, error) {
	var ap models.Approval
	if err := s.db.WithContext(ctx).Take(&ap, id).Error; err != nil {
		return nil, ErrApprovalMissing
	}
	return &ap, nil
}
