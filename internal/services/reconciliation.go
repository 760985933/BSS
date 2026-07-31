package services

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"bss/internal/models"
)

var (
	// ErrStatementMissing 银行流水不存在或已删除
	ErrStatementMissing = errors.New("银行流水不存在")
	// ErrPaymentMissing 回款记录不存在或已删除
	ErrPaymentMissing = errors.New("回款记录不存在")
	// ErrAlreadyReconciled 该回款记录已被其他流水勾对
	ErrAlreadyReconciled = errors.New("该回款记录已被其他流水勾对")
)

// BankStatementInput 单条银行流水录入
type BankStatementInput struct {
	TransDate    string `json:"trans_date"`
	Counterparty string `json:"counterparty"`
	AmountCent   int64  `json:"amount_cent"`
	Direction    string `json:"direction"` // income / expend
	Summary      string `json:"summary"`
}

// CreateBankStatements 批量录入银行流水（payment_record_id 初始为空，即未勾对）
func CreateBankStatements(ctx context.Context, db *gorm.DB, items []BankStatementInput) (int, error) {
	rows := make([]models.BankStatement, 0, len(items))
	for _, it := range items {
		dir := it.Direction
		if dir != "income" && dir != "expend" {
			dir = "income"
		}
		rows = append(rows, models.BankStatement{
			TransDate:    it.TransDate,
			Counterparty: it.Counterparty,
			AmountCent:   it.AmountCent,
			Direction:    dir,
			Summary:      it.Summary,
		})
	}
	if err := db.Create(&rows).Error; err != nil {
		return 0, err
	}
	return len(rows), nil
}

// ListBankStatements 查询流水列表；reconciled 非空时按是否已勾对过滤
func ListBankStatements(ctx context.Context, db *gorm.DB, reconciled *bool) ([]models.BankStatement, error) {
	q := db.Where("deleted_at IS NULL")
	if reconciled != nil {
		if *reconciled {
			q = q.Where("payment_record_id IS NOT NULL")
		} else {
			q = q.Where("payment_record_id IS NULL")
		}
	}
	var rows []models.BankStatement
	if err := q.Order("trans_date DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// Reconcile 将一条流水勾对到一条回款记录（一对多不允许多条流水勾对同一回款）
func Reconcile(ctx context.Context, db *gorm.DB, statementID, paymentRecordID uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var st models.BankStatement
		if err := tx.Where("id = ? AND deleted_at IS NULL", statementID).First(&st).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrStatementMissing
			}
			return err
		}
		var pr models.PaymentRecord
		if err := tx.Where("id = ? AND deleted_at IS NULL", paymentRecordID).First(&pr).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPaymentMissing
			}
			return err
		}
		var cnt int64
		tx.Model(&models.BankStatement{}).
			Where("payment_record_id = ? AND id <> ? AND deleted_at IS NULL", paymentRecordID, statementID).
			Count(&cnt)
		if cnt > 0 {
			return ErrAlreadyReconciled
		}
		return tx.Model(&st).Update("payment_record_id", paymentRecordID).Error
	})
}

// Unreconcile 取消勾对（流水 payment_record_id 置空）
func Unreconcile(ctx context.Context, db *gorm.DB, statementID uint) error {
	var st models.BankStatement
	if err := db.Where("id = ? AND deleted_at IS NULL", statementID).First(&st).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrStatementMissing
		}
		return err
	}
	return db.Model(&st).Update("payment_record_id", nil).Error
}

// ReconItem 未达账项条目
type ReconItem struct {
	ID           uint   `json:"id"`
	TransDate    string `json:"trans_date"`
	Counterparty string `json:"counterparty"`
	AmountCent   int64  `json:"amount_cent"`
	Direction    string `json:"direction"`
	Summary      string `json:"summary"`
}

// Reconciliation 未达账项汇总
type Reconciliation struct {
	BankOnly    []ReconItem `json:"bank_only"`    // 银行已收企业未收
	CompanyOnly []ReconItem `json:"company_only"` // 企业已收银行未收
}

func toReconItem(b models.BankStatement) ReconItem {
	return ReconItem{
		ID:           b.ID,
		TransDate:    b.TransDate,
		Counterparty: b.Counterparty,
		AmountCent:   b.AmountCent,
		Direction:    b.Direction,
		Summary:      b.Summary,
	}
}

// ReconciliationSummary 输出未达账项：
// - 银行已收企业未收：income 流水且未勾对
// - 企业已收银行未收：回款记录未被任何流水勾对
func ReconciliationSummary(ctx context.Context, db *gorm.DB) (*Reconciliation, error) {
	res := &Reconciliation{}

	var bs []models.BankStatement
	if err := db.Where("deleted_at IS NULL AND payment_record_id IS NULL AND direction = 'income'").
		Find(&bs).Error; err != nil {
		return nil, err
	}
	for _, b := range bs {
		res.BankOnly = append(res.BankOnly, toReconItem(b))
	}

	var prs []models.PaymentRecord
	if err := db.Where("deleted_at IS NULL AND id NOT IN ("+
		"SELECT payment_record_id FROM bank_statements WHERE payment_record_id IS NOT NULL AND deleted_at IS NULL)").
		Find(&prs).Error; err != nil {
		return nil, err
	}
	for _, p := range prs {
		res.CompanyOnly = append(res.CompanyOnly, ReconItem{
			ID:         p.ID,
			TransDate:  p.PaidAt,
			AmountCent: p.AmountCent,
			Direction:  "income",
			Summary:    "回款记录",
		})
	}
	return res, nil
}
