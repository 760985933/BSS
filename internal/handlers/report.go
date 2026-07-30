package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"bss/internal/services"
	"bss/internal/pkg/resp"
)

// ReportHandler 报表中心接口（M2-3）。登录即可访问（数据范围由 ScopeOwner 控制）。
type ReportHandler struct {
	svc *services.ReportService
}

func NewReportHandler(svc *services.ReportService) *ReportHandler {
	return &ReportHandler{svc: svc}
}

// SignTrend GET /reports/sign-trend?months=12
func (h *ReportHandler) SignTrend(w http.ResponseWriter, r *http.Request) {
	months := atoiDefault(r.URL.Query().Get("months"), 12)
	res, err := h.svc.GetSignTrend(r.Context(), months)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, res)
}

// PaymentTrend GET /reports/payment-trend?months=12
func (h *ReportHandler) PaymentTrend(w http.ResponseWriter, r *http.Request) {
	months := atoiDefault(r.URL.Query().Get("months"), 12)
	res, err := h.svc.GetPaymentTrend(r.Context(), months)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, res)
}

// SalesRank GET /reports/sales-rank
func (h *ReportHandler) SalesRank(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.GetSalesRank(r.Context())
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, res)
}

// Funnel GET /reports/funnel
func (h *ReportHandler) Funnel(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.GetFunnel(r.Context())
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, res)
}

// Export GET /reports/export?type=sign_trend|payment_trend|sales_rank|funnel
// 以附件形式返回 CSV（UTF-8 BOM）。
func (h *ReportHandler) Export(w http.ResponseWriter, r *http.Request) {
	typ := r.URL.Query().Get("type")
	content, filename, err := h.svc.ExportCSV(r.Context(), typ)
	if err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(content))
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	return n
}
