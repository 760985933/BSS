package services

import (
	"context"
	"errors"
	"time"

	"bss/internal/models"
	"bss/internal/pkg/code"

	"gorm.io/gorm"
)

var (
	ErrInvoiceMissing        = errors.New("发票不存在")
	ErrInvoiceAmountExceed   = errors.New("开票累计金额已超过合同额")
	ErrInvoiceInvalidState   = errors.New("发票状态流转非法")
	ErrInvoiceNotDraft       = errors.New("仅待开(draft)发票可编辑/删除")
	ErrInvoiceNegativeAmount = errors.New("开票金额必须为正数")
)

// InvoiceService 开票管理（M2-2）：与合同关联，累计金额受合同额约束
type InvoiceService struct {
	db      *gorm.DB
	codeGen *code.Generator
}

func NewInvoiceService(db *gorm.DB) *InvoiceService {
	return &InvoiceService{db: db, codeGen: code.NewGenerator(db)}
}

// InvoiceInput 开票可写字段
type InvoiceInput struct {
	ContractID      uint  `json:"contract_id,string"`
	PaymentRecordID *uint `json:"payment_record_id,string"`
	AmountCent      int64 `json:"amount_cent"`
	Remark          string `json:"remark"`
}

// sumInvoiceCent 统计合同下未作废(draft+issued)发票累计金额
func (s *InvoiceService) sumInvoiceCent(ctx context.Context, contractID uint, excludeID uint) (int64, error) {
	var sum int64
	err := s.db.WithContext(ctx).Model(&models.Invoice{}).
		Where("contract_id = ? AND status IN (?, ?) AND id <> ?", contractID, models.InvoiceDraft, models.InvoiceIssued, excludeID).
		Select("COALESCE(SUM(amount_cent), 0)").Scan(&sum).Error
	return sum, err
}

// Create 新建发票（默认 draft，待开）；累计金额不得超合同额
func (s *InvoiceService) Create(ctx context.Context, in InvoiceInput, createdBy uint) (*models.Invoice, error) {
	if in.AmountCent <= 0 {
		return nil, ErrInvoiceNegativeAmount
	}
	var ct models.Contract
	if err := s.db.WithContext(ctx).Take(&ct, in.ContractID).Error; err != nil {
		return nil, ErrContractMissing
	}
	sum, err := s.sumInvoiceCent(ctx, in.ContractID, 0)
	if err != nil {
		return nil, err
	}
	if sum+in.AmountCent > ct.AmountCent {
		return nil, ErrInvoiceAmountExceed
	}
	c, err := s.codeGen.Next(ctx, code.PrefixInvoice)
	if err != nil {
		return nil, err
	}
	inv := &models.Invoice{
		Code: c, ContractID: in.ContractID, PaymentRecordID: in.PaymentRecordID,
		AmountCent: in.AmountCent, Status: models.InvoiceDraft, Remark: in.Remark, CreatedBy: createdBy,
	}
	if err := s.db.WithContext(ctx).Create(inv).Error; err != nil {
		return nil, err
	}
	return inv, nil
}

// Issue 开票：draft → issued
func (s *InvoiceService) Issue(ctx context.Context, id uint) error {
	var inv models.Invoice
	if err := s.db.WithContext(ctx).Take(&inv, id).Error; err != nil {
		return ErrInvoiceMissing
	}
	if inv.Status != models.InvoiceDraft {
		return ErrInvoiceInvalidState
	}
	return s.db.WithContext(ctx).Model(&inv).Updates(map[string]any{
		"status":    models.InvoiceIssued,
		"issued_at": time.Now().UTC().Format("2006-01-02"),
	}).Error
}

// Void 作废：draft/issued → voided
func (s *InvoiceService) Void(ctx context.Context, id uint) error {
	var inv models.Invoice
	if err := s.db.WithContext(ctx).Take(&inv, id).Error; err != nil {
		return ErrInvoiceMissing
	}
	if inv.Status == models.InvoiceVoided {
		return ErrInvoiceInvalidState
	}
	return s.db.WithContext(ctx).Model(&inv).Update("status", models.InvoiceVoided).Error
}

// Update 编辑发票（仅 draft，且金额仍不超合同额）
func (s *InvoiceService) Update(ctx context.Context, id uint, in InvoiceInput) error {
	var inv models.Invoice
	if err := s.db.WithContext(ctx).Take(&inv, id).Error; err != nil {
		return ErrInvoiceMissing
	}
	if inv.Status != models.InvoiceDraft {
		return ErrInvoiceNotDraft
	}
	if in.AmountCent <= 0 {
		return ErrInvoiceNegativeAmount
	}
	var ct models.Contract
	if err := s.db.WithContext(ctx).Take(&ct, inv.ContractID).Error; err != nil {
		return ErrContractMissing
	}
	sum, err := s.sumInvoiceCent(ctx, inv.ContractID, id)
	if err != nil {
		return err
	}
	if sum+in.AmountCent > ct.AmountCent {
		return ErrInvoiceAmountExceed
	}
	return s.db.WithContext(ctx).Model(&inv).Updates(map[string]any{
		"amount_cent": in.AmountCent, "remark": in.Remark, "payment_record_id": in.PaymentRecordID,
	}).Error
}

// Delete 删除发票（仅 draft）
func (s *InvoiceService) Delete(ctx context.Context, id uint) error {
	var inv models.Invoice
	if err := s.db.WithContext(ctx).Take(&inv, id).Error; err != nil {
		return ErrInvoiceMissing
	}
	if inv.Status != models.InvoiceDraft {
		return ErrInvoiceNotDraft
	}
	return s.db.WithContext(ctx).Delete(&inv).Error
}

// List 发票列表（按合同归属 ScopeOwner 过滤 + 筛选 + 分页）
func (s *InvoiceService) List(ctx context.Context, page, size int, contractID, status string) ([]models.Invoice, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	// 行级范围：发票归属合同的负责人
	var allowed []uint
	if err := s.db.WithContext(ctx).Model(&models.Contract{}).
		Scopes(func(tx *gorm.DB) *gorm.DB { return ScopeOwner(tx, ctx) }).
		Pluck("id", &allowed).Error; err != nil {
		return nil, 0, err
	}
	base := s.db.WithContext(ctx).Model(&models.Invoice{}).Preload("Contract").
		Where("contract_id IN (?)", allowed)
	if contractID != "" {
		base = base.Where("contract_id = ?", contractID)
	}
	if status != "" {
		base = base.Where("status = ?", status)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []models.Invoice
	if err := base.Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// Get 发票详情
func (s *InvoiceService) Get(ctx context.Context, id uint) (*models.Invoice, error) {
	var inv models.Invoice
	if err := s.db.WithContext(ctx).Preload("Contract").Take(&inv, id).Error; err != nil {
		return nil, ErrInvoiceMissing
	}
	return &inv, nil
}

// ContractInvoiceTotal 合同已开票(未作废)累计，供前端提示
func (s *InvoiceService) ContractInvoiceTotal(ctx context.Context, contractID uint) (int64, error) {
	return s.sumInvoiceCent(ctx, contractID, 0)
}
