package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"bss/internal/models"
	"bss/internal/pkg/money"

	"gorm.io/gorm"
)

// 提醒类型
const (
	NotifContractExpiring = "contract_expiring"
	NotifPaymentOverdue   = "payment_overdue"
)

// ScanReminders 每日定时扫描：合同 30 天内到期 + 回款逾期 → 写入 notifications。
// 通过 dedup_key（type|entity_id|关联日期）去重，同一实体每天最多一条。
// 新建的通知会按配置外发到邮件/企业微信（M3-4，渠道默认关闭时零开销）。
// 返回本次新建的通知数量。
func ScanReminders(ctx context.Context, db *gorm.DB, now time.Time) (int, error) {
	today := now.Format("2006-01-02")
	limit := now.AddDate(0, 0, 30).Format("2006-01-02")
	created := 0
	var fresh []models.Notification
	defer func() { dispatchAll(ctx, db, fresh) }()

	// 1) 合同 30 天内到期（仅进行中的签约/履约状态）
	var contracts []models.Contract
	if err := db.Where("expire_date >= ? AND expire_date <= ? AND status IN ?",
		today, limit, []string{models.ContractSigned, models.ContractPerforming}).Find(&contracts).Error; err != nil {
		return 0, err
	}
	for _, c := range contracts {
		if c.OwnerID == 0 {
			continue
		}
		key := fmt.Sprintf("%s|%d|%s", NotifContractExpiring, c.ID, c.ExpireDate)
		if existsNotification(db, key) {
			continue
		}
		n := models.Notification{
			UserID:     c.OwnerID,
			Type:       NotifContractExpiring,
			Title:      fmt.Sprintf("合同 %s 将于 %s 到期", c.Code, c.ExpireDate),
			Content:    fmt.Sprintf("客户合同《%s》到期日 %s，请关注续签或回款。", c.Title, c.ExpireDate),
			EntityType: "contract",
			EntityID:   c.ID,
			DedupKey:   key,
		}
		if err := db.Create(&n).Error; err != nil {
			return created, err
		}
		fresh = append(fresh, n)
		created++
	}

	// 2) 回款逾期（到期日已过且未全额核销）
	var plans []models.PaymentPlan
	if err := db.Where("due_date < ? AND status IN ?", today,
		[]string{models.PlanPending, models.PlanPartial}).Find(&plans).Error; err != nil {
		return 0, err
	}
	for _, p := range plans {
		var ct models.Contract
		if err := db.First(&ct, p.ContractID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return created, err
		}
		if ct.OwnerID == 0 {
			continue
		}
		key := fmt.Sprintf("%s|%d|%s", NotifPaymentOverdue, p.ID, p.DueDate)
		if existsNotification(db, key) {
			continue
		}
		n := models.Notification{
			UserID:     ct.OwnerID,
			Type:       NotifPaymentOverdue,
			Title:      fmt.Sprintf("回款计划 #%d 已逾期", p.PeriodNo),
			Content:    fmt.Sprintf("合同 %s 第 %d 期回款 %s 应于 %s 到账，目前已逾期。", ct.Code, p.PeriodNo, money.Format(p.AmountCent), p.DueDate),
			EntityType: "payment_plan",
			EntityID:   p.ID,
			DedupKey:   key,
		}
		if err := db.Create(&n).Error; err != nil {
			return created, err
		}
		fresh = append(fresh, n)
		created++
	}
	return created, nil
}

// dispatchAll 将新生成的通知外发到已启用渠道；失败只记日志，不影响站内信。
func dispatchAll(ctx context.Context, db *gorm.DB, list []models.Notification) {
	if len(list) == 0 {
		return
	}
	svc := NewNotifyService(db)
	for i := range list {
		svc.Dispatch(ctx, &list[i])
	}
}

func existsNotification(db *gorm.DB, key string) bool {
	var cnt int64
	db.Model(&models.Notification{}).Where("dedup_key = ?", key).Count(&cnt)
	return cnt > 0
}

// ListNotifications 当前用户的通知列表（按创建时间倒序），支持 is_read / type 过滤与分页。
func ListNotifications(ctx context.Context, db *gorm.DB, userID uint, isRead *bool, typ string, page, size int) ([]models.Notification, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	q := db.Model(&models.Notification{}).Where("user_id = ?", userID)
	if isRead != nil {
		if *isRead {
			q = q.Where("is_read = 1")
		} else {
			q = q.Where("is_read = 0")
		}
	}
	if typ != "" {
		q = q.Where("type = ?", typ)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []models.Notification
	if err := q.Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// UnreadCount 当前用户未读通知数。
func UnreadCount(ctx context.Context, db *gorm.DB, userID uint) (int64, error) {
	var cnt int64
	if err := db.Model(&models.Notification{}).Where("user_id = ? AND is_read = 0", userID).Count(&cnt).Error; err != nil {
		return 0, err
	}
	return cnt, nil
}

// MarkRead 标记单条通知为已读；仅通知归属人可操作（强制所有权校验）。
func MarkRead(ctx context.Context, db *gorm.DB, userID, id uint) error {
	var n models.Notification
	if err := db.First(&n, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotificationNotFound
		}
		return err
	}
	if n.UserID != userID {
		return ErrNotificationForbidden
	}
	if n.IsRead {
		return nil
	}
	return db.Model(&n).Update("is_read", true).Error
}

// MarkAllRead 标记当前用户全部未读为已读。
func MarkAllRead(ctx context.Context, db *gorm.DB, userID uint) error {
	return db.Model(&models.Notification{}).
		Where("user_id = ? AND is_read = 0", userID).
		Update("is_read", true).Error
}

// Dashboard 仪表盘聚合（数据范围由 ScopeOwner 控制）。
type Dashboard struct {
	Cards struct {
		SignedThisMonth int64 `json:"signed_this_month_cent"` // 本月签约金额（分）
		PaidThisMonth   int64 `json:"paid_this_month_cent"`   // 本月回款金额（分）
		OpenDeals       int64 `json:"open_deals"`             // 进行中商单数
		OverdueAmount   int64 `json:"overdue_amount_cent"`    // 逾期回款金额（分）
	} `json:"cards"`
	ExpiringContracts []ContractLite  `json:"expiring_contracts"`
	OverduePlans      []PlanLite      `json:"overdue_plans"`
	RecentWonDeals    []DealLite      `json:"recent_won_deals"`
}

type ContractLite struct {
	ID         uint   `json:"id,string"`
	Code       string `json:"code"`
	Title      string `json:"title"`
	Customer   string `json:"customer"`
	AmountCent int64  `json:"amount_cent"`
	ExpireDate string `json:"expire_date"`
	Status     string `json:"status"`
}

type PlanLite struct {
	ID            uint   `json:"id,string"`
	ContractCode  string `json:"contract_code"`
	PeriodNo      int    `json:"period_no"`
	DueDate       string `json:"due_date"`
	AmountCent    int64  `json:"amount_cent"`
	PaidCent      int64  `json:"paid_cent"`
	Outstanding   int64  `json:"outstanding_cent"`
}

type DealLite struct {
	ID           uint   `json:"id,string"`
	Code         string `json:"code"`
	Title        string `json:"title"`
	Customer     string `json:"customer"`
	AmountCent   int64  `json:"amount_cent"`
	Probability  int    `json:"probability"`
	Status       string `json:"status"`
}

// GetDashboard 计算仪表盘数据，所有列表与卡片均受 ScopeOwner 约束。
func GetDashboard(ctx context.Context, db *gorm.DB, now time.Time) (*Dashboard, error) {
	d := &Dashboard{}
	month := now.Format("2006-01")
	today := now.Format("2006-01-02")
	cutoff := now.AddDate(0, 0, -30).Format("2006-01-02 15:04:05")

	// —— 卡片 1：本月签约金额 ——
	db.Table("contracts").Scopes(func(tx *gorm.DB) *gorm.DB { return ScopeOwner(tx, ctx) }).
		Where("status IN ? AND sign_date LIKE ?", []string{models.ContractSigned, models.ContractPerforming, models.ContractCompleted}, month+"%").
		Select("COALESCE(SUM(amount_cent),0)").Scan(&d.Cards.SignedThisMonth)

	// —— 卡片 2：本月回款金额（按合同归属过滤）——
	allowedC := db.Table("contracts").Scopes(func(tx *gorm.DB) *gorm.DB { return ScopeOwner(tx, ctx) }).Select("id")
	db.Table("payment_records").
		Where("contract_id IN (?) AND created_at LIKE ?", allowedC, month+"%").
		Select("COALESCE(SUM(amount_cent),0)").Scan(&d.Cards.PaidThisMonth)

	// —— 卡片 3：进行中商单数 ——
	db.Table("deals").Scopes(func(tx *gorm.DB) *gorm.DB { return ScopeOwner(tx, ctx) }).
		Where("status IN ?", []string{models.DealProspecting, models.DealQualifying, models.DealProposal, models.DealNegotiating}).
		Count(&d.Cards.OpenDeals)

	// —— 卡片 4：逾期回款金额（逾期且未全额核销的计划，未核销部分之和）——
	allowed := db.Table("contracts").Scopes(func(tx *gorm.DB) *gorm.DB { return ScopeOwner(tx, ctx) }).Select("id")
	var overdue []models.PaymentPlan
	if err := db.Where("contract_id IN (?) AND due_date < ? AND status IN ?", allowed, today, []string{models.PlanPending, models.PlanPartial}).
		Find(&overdue).Error; err != nil {
		return nil, err
	}
	received := planReceivedMap(db, collectPlanIDs(overdue))
	for _, p := range overdue {
		rcv := received[p.ID]
		if rcv < p.AmountCent {
			d.Cards.OverdueAmount += p.AmountCent - rcv
		}
	}

	// —— 列表 1：即将到期合同 ——
	var expiring []models.Contract
	if err := db.Table("contracts").Scopes(func(tx *gorm.DB) *gorm.DB { return ScopeOwner(tx, ctx) }).
		Where("expire_date >= ? AND expire_date <= ? AND status IN ?", today, now.AddDate(0, 0, 30).Format("2006-01-02"),
			[]string{models.ContractSigned, models.ContractPerforming}).
		Order("expire_date ASC").Limit(10).Find(&expiring).Error; err != nil {
		return nil, err
	}
	for _, c := range expiring {
		custName := ""
		if c.CustomerID != 0 {
			var cu models.Customer
			if db.First(&cu, c.CustomerID).Error == nil {
				custName = cu.Name
			}
		}
		d.ExpiringContracts = append(d.ExpiringContracts, ContractLite{
			ID: c.ID, Code: c.Code, Title: c.Title, Customer: custName,
			AmountCent: c.AmountCent, ExpireDate: c.ExpireDate, Status: c.Status,
		})
	}

	// —— 列表 2：逾期回款计划 ——
	for _, p := range overdue {
		var ct models.Contract
		code := ""
		if db.First(&ct, p.ContractID).Error == nil {
			code = ct.Code
		}
		d.OverduePlans = append(d.OverduePlans, PlanLite{
			ID: p.ID, ContractCode: code, PeriodNo: p.PeriodNo, DueDate: p.DueDate,
			AmountCent: p.AmountCent, PaidCent: received[p.ID], Outstanding: p.AmountCent - received[p.ID],
		})
	}

	// —— 列表 3：近期赢单（近 30 天）——
	var won []models.Deal
	if err := db.Table("deals").Scopes(func(tx *gorm.DB) *gorm.DB { return ScopeOwner(tx, ctx) }).
		Where("status = ? AND updated_at >= ?", models.DealWon, cutoff).
		Order("updated_at DESC").Limit(10).Find(&won).Error; err != nil {
		return nil, err
	}
	for _, dl := range won {
		custName := ""
		if dl.CustomerID != 0 {
			var cu models.Customer
			if db.First(&cu, dl.CustomerID).Error == nil {
				custName = cu.Name
			}
		}
		d.RecentWonDeals = append(d.RecentWonDeals, DealLite{
			ID: dl.ID, Code: dl.Code, Title: dl.Title, Customer: custName,
			AmountCent: dl.AmountCent, Probability: dl.Probability, Status: dl.Status,
		})
	}

	return d, nil
}

// planReceivedMap 计算每个计划已核销金额（plan_id 非空的记录之和）。
func planReceivedMap(db *gorm.DB, ids []uint) map[uint]int64 {
	m := make(map[uint]int64)
	if len(ids) == 0 {
		return m
	}
	type row struct {
		PlanID uint
		Sum    int64
	}
	var rows []row
	db.Model(&models.PaymentRecord{}).
		Select("plan_id AS plan_id, COALESCE(SUM(amount_cent),0) AS sum").
		Where("plan_id IN ?", ids).Group("plan_id").Scan(&rows)
	for _, r := range rows {
		m[r.PlanID] = r.Sum
	}
	return m
}

func collectPlanIDs(plans []models.PaymentPlan) []uint {
	ids := make([]uint, 0, len(plans))
	for _, p := range plans {
		ids = append(ids, p.ID)
	}
	return ids
}

// 通知相关错误
var (
	ErrNotificationNotFound = errors.New("通知不存在")
	ErrNotificationForbidden = errors.New("无权操作该通知")
)
