package services

import (
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"bss/internal/middleware"
	"bss/internal/models"

	"gorm.io/gorm"
)

// ReportService 报表中心（M2-3）：聚合签约/回款趋势、销售排行、转化漏斗，并支持 CSV 导出。
// 所有聚合均受 ScopeOwner 约束（通过 visibleOwnerIDs 落地：admin/finance/hr 看全部，
// sales 仅本人，sales_lead 仅本部门）。
type ReportService struct {
	db *gorm.DB
}

func NewReportService(db *gorm.DB) *ReportService {
	return &ReportService{db: db}
}

// visibleOwnerIDs 返回当前登录用户在"按负责人归属"口径下可见的 owner 集合。
// 返回 nil 表示不过滤（admin/finance/hr）；返回非空切片表示仅这些 owner_id 可见；
// 返回空切片表示无任何可见数据（如 sales_lead 所在部门无成员）。
func visibleOwnerIDs(db *gorm.DB, ctx context.Context) ([]uint, error) {
	c := middleware.UserFrom(ctx)
	if c == nil {
		return []uint{}, nil // 未登录兜底：无可见数据
	}
	switch c.Role {
	case models.RoleAdmin, models.RoleFinance, models.RoleHR:
		return nil, nil
	case models.RoleSalesLead:
		var ids []uint
		if err := db.Model(&models.Employee{}).Where("dept = ? AND deleted_at IS NULL", c.Dept).Pluck("id", &ids).Error; err != nil {
			return nil, err
		}
		return ids, nil
	default: // sales
		return []uint{c.UserID}, nil
	}
}

// scopeOwner 将可见 owner 集合应用到查询（ownerIDs 非 nil 时追加 IN 条件）。
// 当 ownerIDs 为空切片时返回 false，提示调用方直接短路为空结果。
func scopeOwner(q *gorm.DB, ownerIDs []uint, column string) (*gorm.DB, bool) {
	if ownerIDs == nil {
		return q, true
	}
	if len(ownerIDs) == 0 {
		return q, false
	}
	return q.Where(fmt.Sprintf("%s IN ?", column), ownerIDs), true
}

// ---------- 数据结构 ----------

type MonthPoint struct {
	Month      string `json:"month"`       // YYYY-MM
	AmountCent int64  `json:"amount_cent"` // 分
}

type SignTrendResult struct {
	Rows []MonthPoint `json:"rows"`
}

type PaymentTrendResult struct {
	Rows []MonthPoint `json:"rows"`
}

type SalesRankRow struct {
	OwnerID   uint   `json:"owner_id,string"`
	OwnerName string `json:"owner_name"`
	WonDeals  int64  `json:"won_deals"`
	SignedCent int64 `json:"signed_cent"`
	PaidCent   int64 `json:"paid_cent"`
}

type SalesRankResult struct {
	Rows []SalesRankRow `json:"rows"`
}

type FunnelRow struct {
	Stage      string `json:"stage"`
	Label      string `json:"label"`
	Count      int64  `json:"count"`
	AmountCent int64  `json:"amount_cent"`
}

type FunnelResult struct {
	Rows []FunnelRow `json:"rows"`
}

// lastNMonths 返回截至当前月往前 N 个月的 YYYY-MM 列表（升序）。N 限制在 1..36。
func lastNMonths(n int) []string {
	if n < 1 {
		n = 12
	}
	if n > 36 {
		n = 36
	}
	now := time.Now()
	out := make([]string, 0, n)
	for i := n - 1; i >= 0; i-- {
		out = append(out, now.AddDate(0, -i, 0).Format("2006-01"))
	}
	return out
}

// ---------- 月度签约趋势 ----------

func (s *ReportService) GetSignTrend(ctx context.Context, months int) (*SignTrendResult, error) {
	monthsList := lastNMonths(months)
	ownerIDs, err := visibleOwnerIDs(s.db, ctx)
	if err != nil {
		return nil, err
	}
	type row struct {
		M   string
		Sum int64
	}
	var rows []row
	q := s.db.Model(&models.Contract{}).
		Select("strftime('%Y-%m', sign_date) AS m, COALESCE(SUM(amount_cent), 0) AS sum").
		Where("sign_date <> '' AND status IN ?", []string{models.ContractSigned, models.ContractPerforming, models.ContractCompleted})
	q, ok := scopeOwner(q, ownerIDs, "owner_id")
	if !ok {
		return &SignTrendResult{Rows: emptyMonths(monthsList)}, nil
	}
	if err := q.Where("strftime('%Y-%m', sign_date) IN ?", monthsList).Group("m").Scan(&rows).Error; err != nil {
		return nil, err
	}
	set := map[string]int64{}
	for _, r := range rows {
		set[r.M] = r.Sum
	}
	return &SignTrendResult{Rows: fillMonths(monthsList, set)}, nil
}

// ---------- 月度回款趋势 ----------

func (s *ReportService) GetPaymentTrend(ctx context.Context, months int) (*PaymentTrendResult, error) {
	monthsList := lastNMonths(months)
	ownerIDs, err := visibleOwnerIDs(s.db, ctx)
	if err != nil {
		return nil, err
	}
	type row struct {
		M   string
		Sum int64
	}
	var rows []row
	q := s.db.Table("payment_records AS pr").
		Select("strftime('%Y-%m', pr.paid_at) AS m, COALESCE(SUM(pr.amount_cent), 0) AS sum").
		Joins("JOIN contracts AS c ON pr.contract_id = c.id").
		Where("pr.paid_at <> ''")
	q, ok := scopeOwner(q, ownerIDs, "c.owner_id")
	if !ok {
		return &PaymentTrendResult{Rows: emptyMonths(monthsList)}, nil
	}
	if err := q.Where("strftime('%Y-%m', pr.paid_at) IN ?", monthsList).Group("m").Scan(&rows).Error; err != nil {
		return nil, err
	}
	set := map[string]int64{}
	for _, r := range rows {
		set[r.M] = r.Sum
	}
	return &PaymentTrendResult{Rows: fillMonths(monthsList, set)}, nil
}

// ---------- 销售排行 ----------

func (s *ReportService) GetSalesRank(ctx context.Context) (*SalesRankResult, error) {
	ownerIDs, err := visibleOwnerIDs(s.db, ctx)
	if err != nil {
		return nil, err
	}

	signedMap := map[uint]int64{}
	{
		type os struct {
			OwnerID uint
			Sum     int64
		}
		var rs []os
		q := s.db.Model(&models.Contract{}).
			Select("owner_id AS owner_id, COALESCE(SUM(amount_cent), 0) AS sum").
			Where("status IN ?", []string{models.ContractSigned, models.ContractPerforming, models.ContractCompleted})
		q, ok := scopeOwner(q, ownerIDs, "owner_id")
		if !ok {
			return &SalesRankResult{Rows: []SalesRankRow{}}, nil
		}
		if err := q.Group("owner_id").Scan(&rs).Error; err != nil {
			return nil, err
		}
		for _, r := range rs {
			signedMap[r.OwnerID] = r.Sum
		}
	}

	wonMap := map[uint]int64{}
	wonSumMap := map[uint]int64{}
	{
		type ow struct {
			OwnerID uint
			Cnt     int64
			Sum     int64
		}
		var rs []ow
		q := s.db.Model(&models.Deal{}).
			Select("owner_id AS owner_id, COUNT(*) AS cnt, COALESCE(SUM(amount_cent), 0) AS sum").
			Where("status = ?", models.DealWon)
		q, ok := scopeOwner(q, ownerIDs, "owner_id")
		if !ok {
			return &SalesRankResult{Rows: []SalesRankRow{}}, nil
		}
		if err := q.Group("owner_id").Scan(&rs).Error; err != nil {
			return nil, err
		}
		for _, r := range rs {
			wonMap[r.OwnerID] = r.Cnt
			wonSumMap[r.OwnerID] = r.Sum
		}
	}

	paidMap := map[uint]int64{}
	{
		type op struct {
			OwnerID uint
			Sum     int64
		}
		var rs []op
		q := s.db.Table("payment_records AS pr").
			Select("c.owner_id AS owner_id, COALESCE(SUM(pr.amount_cent), 0) AS sum").
			Joins("JOIN contracts AS c ON pr.contract_id = c.id")
		q, ok := scopeOwner(q, ownerIDs, "c.owner_id")
		if !ok {
			return &SalesRankResult{Rows: []SalesRankRow{}}, nil
		}
		if err := q.Group("c.owner_id").Scan(&rs).Error; err != nil {
			return nil, err
		}
		for _, r := range rs {
			paidMap[r.OwnerID] = r.Sum
		}
	}

	// 汇总 owner 集合
	owners := map[uint]bool{}
	for id := range signedMap {
		owners[id] = true
	}
	for id := range wonMap {
		owners[id] = true
	}
	for id := range paidMap {
		owners[id] = true
	}

	// 取 owner 名称
	nameMap := map[uint]string{}
	if len(owners) > 0 {
		var emps []models.Employee
		if err := s.db.Where("id IN ?", keysOf(owners)).Find(&emps).Error; err != nil {
			return nil, err
		}
		for _, e := range emps {
			nameMap[e.ID] = e.Name
		}
	}

	result := &SalesRankResult{Rows: make([]SalesRankRow, 0, len(owners))}
	for id := range owners {
		result.Rows = append(result.Rows, SalesRankRow{
			OwnerID:   id,
			OwnerName: nameMap[id],
			WonDeals:  wonMap[id],
			SignedCent: signedMap[id],
			PaidCent:   paidMap[id],
		})
	}
	// 按签约金额降序
	sortRowsBySigned(result.Rows)
	return result, nil
}

// ---------- 客户转化漏斗（商单阶段分布） ----------

var funnelStages = []struct {
	stage string
	label string
}{
	{models.DealProspecting, "线索"},
	{models.DealQualifying, "需求确认"},
	{models.DealProposal, "方案报价"},
	{models.DealNegotiating, "谈判中"},
	{models.DealWon, "赢单"},
}

func (s *ReportService) GetFunnel(ctx context.Context) (*FunnelResult, error) {
	ownerIDs, err := visibleOwnerIDs(s.db, ctx)
	if err != nil {
		return nil, err
	}
	res := &FunnelResult{Rows: make([]FunnelRow, 0, len(funnelStages))}
	for _, fs := range funnelStages {
		type rc struct {
			Cnt int64
			Sum int64
		}
		var r rc
		q := s.db.Model(&models.Deal{}).
			Select("COUNT(*) AS cnt, COALESCE(SUM(amount_cent), 0) AS sum").
			Where("status = ?", fs.stage)
		q, ok := scopeOwner(q, ownerIDs, "owner_id")
		if !ok {
			res.Rows = append(res.Rows, FunnelRow{Stage: fs.stage, Label: fs.label})
			continue
		}
		if err := q.Scan(&r).Error; err != nil {
			return nil, err
		}
		res.Rows = append(res.Rows, FunnelRow{Stage: fs.stage, Label: fs.label, Count: r.Cnt, AmountCent: r.Sum})
	}
	return res, nil
}

// ---------- CSV 导出 ----------

// ReportType 报表类型
type ReportType string

const (
	ReportSignTrend   ReportType = "sign_trend"
	ReportPaymentTrend ReportType = "payment_trend"
	ReportSalesRank    ReportType = "sales_rank"
	ReportFunnel       ReportType = "funnel"
)

// ExportCSV 生成指定报表的 CSV（含 UTF-8 BOM，便于 Excel 直接打开）。返回内容、文件名、错误。
func (s *ReportService) ExportCSV(ctx context.Context, typ string) (string, string, error) {
	switch ReportType(typ) {
	case ReportSignTrend:
		res, err := s.GetSignTrend(ctx, 12)
		if err != nil {
			return "", "", err
		}
		recs := [][]string{{"月份", "签约金额(元)"}}
		for _, r := range res.Rows {
			recs = append(recs, []string{r.Month, centToYuan(r.AmountCent)})
		}
		return csvWithBOM(recs), "bss_sign_trend.csv", nil
	case ReportPaymentTrend:
		res, err := s.GetPaymentTrend(ctx, 12)
		if err != nil {
			return "", "", err
		}
		recs := [][]string{{"月份", "回款金额(元)"}}
		for _, r := range res.Rows {
			recs = append(recs, []string{r.Month, centToYuan(r.AmountCent)})
		}
		return csvWithBOM(recs), "bss_payment_trend.csv", nil
	case ReportSalesRank:
		res, err := s.GetSalesRank(ctx)
		if err != nil {
			return "", "", err
		}
		recs := [][]string{{"销售", "赢单数", "签约金额(元)", "回款金额(元)"}}
		for _, r := range res.Rows {
			recs = append(recs, []string{r.OwnerName, strconv.FormatInt(r.WonDeals, 10), centToYuan(r.SignedCent), centToYuan(r.PaidCent)})
		}
		return csvWithBOM(recs), "bss_sales_rank.csv", nil
	case ReportFunnel:
		res, err := s.GetFunnel(ctx)
		if err != nil {
			return "", "", err
		}
		recs := [][]string{{"阶段", "数量", "金额(元)"}}
		for _, r := range res.Rows {
			recs = append(recs, []string{r.Label, strconv.FormatInt(r.Count, 10), centToYuan(r.AmountCent)})
		}
		return csvWithBOM(recs), "bss_funnel.csv", nil
	default:
		return "", "", fmt.Errorf("未知报表类型: %s", typ)
	}
}

// ---------- 辅助 ----------

func emptyMonths(list []string) []MonthPoint {
	out := make([]MonthPoint, 0, len(list))
	for _, m := range list {
		out = append(out, MonthPoint{Month: m})
	}
	return out
}

func fillMonths(list []string, set map[string]int64) []MonthPoint {
	out := make([]MonthPoint, 0, len(list))
	for _, m := range list {
		out = append(out, MonthPoint{Month: m, AmountCent: set[m]})
	}
	return out
}

func keysOf(m map[uint]bool) []uint {
	out := make([]uint, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func sortRowsBySigned(rows []SalesRankRow) {
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			if rows[j].SignedCent > rows[i].SignedCent {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
}

func centToYuan(cent int64) string {
	neg := cent < 0
	if neg {
		cent = -cent
	}
	yuan := cent / 100
	frac := cent % 100
	s := strconv.FormatInt(yuan, 10) + "." + fmt.Sprintf("%02d", frac)
	if neg {
		s = "-" + s
	}
	return s
}

func csvField(s string) string {
	if strings.ContainsAny(s, ",\"\n\r") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}

func csvWithBOM(records [][]string) string {
	var b strings.Builder
	b.WriteString("\uFEFF") // UTF-8 BOM
	w := csv.NewWriter(&b)
	for _, rec := range records {
		fields := make([]string, len(rec))
		for i, f := range rec {
			fields[i] = csvField(f)
		}
		_ = w.Write(fields)
	}
	w.Flush()
	return b.String()
}
