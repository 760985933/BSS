package services

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"bss/internal/actor"
	"bss/internal/models"
	"bss/internal/pkg/code"

	"gorm.io/gorm"
)

var (
	ErrContractMissing          = errors.New("合同不存在")
	ErrContractInvalidTransition = errors.New("非法的合同状态流转")
	ErrTerminateReasonRequired  = errors.New("合同终止必须填写终止原因")
	ErrCrossCustomerLink        = errors.New("关联商单与合同客户不一致")
	ErrDealNotWon               = errors.New("只能关联已赢单(won)的商单")
	ErrContractFieldLocked      = errors.New("合同已签约，关键字段只读")
	ErrContractLocked           = errors.New("合同已进入签约/终止等终态，禁止删除")
	ErrContractHasChildren      = errors.New("合同下存在回款计划，禁止删除")
	ErrAttachmentMissing        = errors.New("附件不存在")
)

// 合同状态机（TECH_DESIGN §6.2：主线 draft→pending→signed→performing→completed；
// 回退 pending→draft；旁路 cancelled(签约前)/terminated·expired(签约后)；approving 二期预留）
var contractFlow = map[string][]string{
	models.ContractDraft:      {models.ContractPending, models.ContractCancelled},
	models.ContractPending:    {models.ContractSigned, models.ContractDraft, models.ContractCancelled},
	models.ContractSigned:     {models.ContractPerforming, models.ContractTerminated, models.ContractExpired},
	models.ContractPerforming: {models.ContractCompleted, models.ContractTerminated, models.ContractExpired},
	models.ContractCompleted:  {},
	models.ContractCancelled:  {},
	models.ContractTerminated: {},
	models.ContractExpired:    {},
}

// contractLocked 终态（signed 及以后）：金额/客户/关联商单只读
func contractLocked(status string) bool {
	switch status {
	case models.ContractSigned, models.ContractPerforming, models.ContractCompleted,
		models.ContractTerminated, models.ContractExpired:
		return true
	}
	return false
}

// contractDeletable 可删除状态：仅签约前的草稿/待签/已取消可删，签约及以后保留历史
func contractDeletable(status string) bool {
	return status == models.ContractDraft || status == models.ContractPending || status == models.ContractCancelled
}

type ContractService struct {
	db      *gorm.DB
	codeGen *code.Generator
}

func NewContractService(db *gorm.DB) *ContractService {
	return &ContractService{db: db, codeGen: code.NewGenerator(db)}
}

// ContractInput 合同可写字段（创建/编辑共用；customer_id 仅创建时生效，编辑只读）
type ContractInput struct {
	CustomerID uint   `json:"customer_id,string"`
	Title      string `json:"title"`
	AmountCent int64  `json:"amount_cent"`
	SignDate   string `json:"sign_date"`
	StartDate  string `json:"start_date"`
	ExpireDate string `json:"expire_date"`
	Remark     string `json:"remark"`
	DealIDs    []uint `json:"deal_ids"`
}

// Create 新建合同：HT 单号、默认 draft、处理关联 won 商单（同客户校验）
func (s *ContractService) Create(ctx context.Context, in ContractInput, ownerID uint) (*models.Contract, error) {
	if in.Title == "" {
		return nil, errors.New("合同标题不能为空")
	}
	if in.CustomerID == 0 {
		return nil, errors.New("必须关联客户")
	}
	if in.AmountCent < 0 {
		return nil, errors.New("金额不能为负数")
	}
	var cnt int64
	if err := s.db.WithContext(ctx).Model(&models.Customer{}).Where("id = ?", in.CustomerID).Count(&cnt).Error; err != nil {
		return nil, err
	}
	if cnt == 0 {
		return nil, errors.New("关联客户不存在")
	}
	c, err := s.codeGen.Next(ctx, code.PrefixContract)
	if err != nil {
		return nil, err
	}
	ct := &models.Contract{
		Code: c, CustomerID: in.CustomerID, Title: in.Title, AmountCent: in.AmountCent,
		SignDate: in.SignDate, StartDate: in.StartDate, ExpireDate: in.ExpireDate,
		Status: models.ContractDraft, OwnerID: ownerID, Remark: in.Remark,
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(ct).Error; err != nil {
			return err
		}
		if len(in.DealIDs) > 0 {
			if err := linkDeals(tx, ct.ID, ct.CustomerID, in.DealIDs); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ct, nil
}

// linkDeals 关联商单：仅 won 且 customer 与合同一致（去重；校验失败整体回滚）
func linkDeals(tx *gorm.DB, contractID, customerID uint, dealIDs []uint) error {
	seen := map[uint]bool{}
	for _, did := range dealIDs {
		if did == 0 || seen[did] {
			continue
		}
		seen[did] = true
		var d models.Deal
		if err := tx.Take(&d, did).Error; err != nil {
			return ErrDealMissing
		}
		if d.Status != models.DealWon {
			return ErrDealNotWon
		}
		if d.CustomerID != customerID {
			return ErrCrossCustomerLink
		}
		if err := tx.Create(&models.DealContract{DealID: did, ContractID: contractID}).Error; err != nil {
			return err
		}
	}
	return nil
}

// Update 编辑合同：终态锁定金额（客户/关联商单创建绑定或走 ReplaceDeals，编辑不可改）
func (s *ContractService) Update(ctx context.Context, id uint, in ContractInput) error {
	var c models.Contract
	if err := s.db.WithContext(ctx).Take(&c, id).Error; err != nil {
		return ErrContractMissing
	}
	if contractLocked(c.Status) {
		if in.AmountCent != c.AmountCent {
			return ErrContractFieldLocked
		}
	}
	if in.Title == "" {
		return errors.New("合同标题不能为空")
	}
	if in.AmountCent < 0 {
		return errors.New("金额不能为负数")
	}
	updates := map[string]any{
		"title":       in.Title,
		"amount_cent": in.AmountCent,
		"sign_date":   in.SignDate,
		"start_date":  in.StartDate,
		"expire_date": in.ExpireDate,
		"remark":      in.Remark,
	}
	return s.db.WithContext(ctx).Model(&c).Updates(updates).Error
}

// ChangeStatus 状态流转唯一入口；terminated 必填原因
func (s *ContractService) ChangeStatus(ctx context.Context, id uint, to, terminateReason string) (*models.Contract, error) {
	var c models.Contract
	if err := s.db.WithContext(ctx).Take(&c, id).Error; err != nil {
		return nil, ErrContractMissing
	}
	if c.Status == to {
		return nil, errors.New("目标状态与当前状态相同")
	}
	legal := false
	for _, next := range contractFlow[c.Status] {
		if next == to {
			legal = true
			break
		}
	}
	if !legal {
		return nil, ErrContractInvalidTransition
	}
	updates := map[string]any{"status": to}
	if to == models.ContractTerminated {
		if strings.TrimSpace(terminateReason) == "" {
			return nil, ErrTerminateReasonRequired
		}
		updates["terminate_reason"] = terminateReason
	}
	if err := s.db.WithContext(ctx).Model(&c).Updates(updates).Error; err != nil {
		return nil, err
	}
	c.Status = to
	return &c, nil
}

// ReplaceDeals 调整关联商单（PUT /contracts/:id/deals）：终态锁定拒绝；校验 won+同客户；写审计
func (s *ContractService) ReplaceDeals(ctx context.Context, id uint, dealIDs []uint) error {
	var c models.Contract
	if err := s.db.WithContext(ctx).Take(&c, id).Error; err != nil {
		return ErrContractMissing
	}
	if contractLocked(c.Status) {
		return ErrContractFieldLocked
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var old []models.DealContract
		if err := tx.Where("contract_id = ?", id).Find(&old).Error; err != nil {
			return err
		}
		if err := tx.Where("contract_id = ?", id).Delete(&models.DealContract{}).Error; err != nil {
			return err
		}
		if err := linkDeals(tx, id, c.CustomerID, dealIDs); err != nil {
			return err
		}
		// 解除/重关联写审计日志（不物理删合同）
		before, _ := json.Marshal(map[string]any{"deal_ids": dealIDsOf(old)})
		after, _ := json.Marshal(map[string]any{"deal_ids": dealIDs})
		if err := tx.Exec(
			`INSERT INTO audit_logs (entity_type, entity_id, action, operator_id, before_json, after_json, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			"contracts", id, "update", actor.From(ctx), string(before), string(after), time.Now().UTC(),
		).Error; err != nil {
			return err
		}
		return nil
	})
}

// dealIDsOf 提取关联记录中的商单 ID 列表
func dealIDsOf(rows []models.DealContract) []uint {
	out := make([]uint, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.DealID)
	}
	return out
}

// Get 详情（含客户、负责人、关联商单）
func (s *ContractService) Get(ctx context.Context, id uint) (*models.Contract, error) {
	var c models.Contract
	if err := s.db.WithContext(ctx).Preload("Customer").Preload("Owner").Preload("Deals").Take(&c, id).Error; err != nil {
		return nil, ErrContractMissing
	}
	return &c, nil
}

// List 列表（ScopeOwner 数据范围 + 筛选 + 分页）
func (s *ContractService) List(ctx context.Context, page, size int, keyword, status, customerID string) ([]models.Contract, int64, error) {
	base := s.db.WithContext(ctx).Model(&models.Contract{}).Preload("Customer").Preload("Owner")
	base = ScopeOwner(base, ctx)
	if keyword != "" {
		base = base.Where("title LIKE ?", "%"+keyword+"%")
	}
	if status != "" {
		base = base.Where("status = ?", status)
	}
	if customerID != "" {
		if id, err := strconv.ParseUint(customerID, 10, 64); err == nil {
			base = base.Where("customer_id = ?", id)
		}
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []models.Contract
	if err := base.Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// OwnerOf 归属辅助（行级权限）
func (s *ContractService) OwnerOf(ctx context.Context, id uint) (uint, error) {
	var c models.Contract
	if err := s.db.WithContext(ctx).Select("owner_id").Take(&c, id).Error; err != nil {
		return 0, ErrContractMissing
	}
	return c.OwnerID, nil
}

// Delete 软删除：仅签约前状态可删；存在回款计划禁删
func (s *ContractService) Delete(ctx context.Context, id uint) error {
	var c models.Contract
	if err := s.db.WithContext(ctx).Take(&c, id).Error; err != nil {
		return ErrContractMissing
	}
	if !contractDeletable(c.Status) {
		return ErrContractLocked
	}
	var pc int64
	if err := s.db.WithContext(ctx).Model(&models.PaymentPlan{}).Where("contract_id = ?", id).Count(&pc).Error; err != nil {
		return err
	}
	if pc > 0 {
		return ErrContractHasChildren
	}
	return s.db.WithContext(ctx).Delete(&c).Error
}

// ---------- 附件 ----------

// AllowedAttachmentExts 附件类型白名单（前端展示与后端校验共用）
var AllowedAttachmentExts = map[string]bool{
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
	".ppt": true, ".pptx": true, ".png": true, ".jpg": true, ".jpeg": true,
	".txt": true, ".csv": true, ".zip": true, ".rar": true,
}

// MaxAttachmentSize 单附件上限 20MB
const MaxAttachmentSize = 20 * 1024 * 1024

// ValidateAttachment 校验附件类型与大小（白名单 + 20MB）
func ValidateAttachment(fileName string, size int64) error {
	if size <= 0 {
		return errors.New("文件为空")
	}
	if size > MaxAttachmentSize {
		return errors.New("文件大小超过 20MB 限制")
	}
	ext := strings.ToLower(filepath.Ext(fileName))
	if !AllowedAttachmentExts[ext] {
		return errors.New("不支持的文件类型：" + ext)
	}
	return nil
}

// CreateAttachment 写入附件记录（文件落盘由 handler 负责，这里只存元数据）
func (s *ContractService) CreateAttachment(ctx context.Context, in models.Attachment) (*models.Attachment, error) {
	if err := s.db.WithContext(ctx).Create(&in).Error; err != nil {
		return nil, err
	}
	return &in, nil
}

// ListAttachments 列出某业务实体的附件
func (s *ContractService) ListAttachments(ctx context.Context, entityType string, entityID uint) ([]models.Attachment, error) {
	var list []models.Attachment
	if err := s.db.WithContext(ctx).Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Order("created_at DESC, id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// GetAttachment 取附件（含物理路径，仅鉴权下载用）
func (s *ContractService) GetAttachment(ctx context.Context, id uint) (*models.Attachment, error) {
	var a models.Attachment
	if err := s.db.WithContext(ctx).Take(&a, id).Error; err != nil {
		return nil, ErrAttachmentMissing
	}
	return &a, nil
}

// DeleteAttachment 删除附件（软删除）
func (s *ContractService) DeleteAttachment(ctx context.Context, id uint) error {
	var a models.Attachment
	if err := s.db.WithContext(ctx).Take(&a, id).Error; err != nil {
		return ErrAttachmentMissing
	}
	return s.db.WithContext(ctx).Delete(&a).Error
}
