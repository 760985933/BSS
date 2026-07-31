package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"bss/internal/pkg/resp"
	"bss/internal/services"
)

// ReconciliationHandler 银企对账（限 admin/finance）
type ReconciliationHandler struct {
	db *gorm.DB
}

// NewReconciliationHandler 构造
func NewReconciliationHandler(db *gorm.DB) *ReconciliationHandler {
	return &ReconciliationHandler{db: db}
}

// Create 批量录入银行流水
func (h *ReconciliationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var items []services.BankStatementInput
	if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求参数错误")
		return
	}
	if len(items) == 0 {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "至少录入一条流水")
		return
	}
	n, err := services.CreateBankStatements(r.Context(), h.db, items)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "录入失败")
		return
	}
	resp.OK(w, map[string]any{"created": n})
}

// List 查询流水列表（可按 reconciled=true/false 过滤）
func (h *ReconciliationHandler) List(w http.ResponseWriter, r *http.Request) {
	var rec *bool
	if v := r.URL.Query().Get("reconciled"); v != "" {
		b := v == "true"
		rec = &b
	}
	rows, err := services.ListBankStatements(r.Context(), h.db, rec)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "查询失败")
		return
	}
	resp.OK(w, rows)
}

// Reconcile 勾对：将流水关联到一条回款记录
func (h *ReconciliationHandler) Reconcile(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "流水 ID 非法")
		return
	}
	var body struct {
		PaymentRecordID uint `json:"payment_record_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PaymentRecordID == 0 {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "回款记录 ID 必填")
		return
	}
	if err := services.Reconcile(r.Context(), h.db, uint(id), body.PaymentRecordID); err != nil {
		switch {
		case errors.Is(err, services.ErrStatementMissing), errors.Is(err, services.ErrPaymentMissing):
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
		case errors.Is(err, services.ErrAlreadyReconciled):
			resp.Fail(w, http.StatusConflict, resp.CodeBadRequest, err.Error())
		default:
			resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "勾对失败")
		}
		return
	}
	resp.OK(w, map[string]string{"message": "勾对成功"})
}

// Unreconcile 取消勾对
func (h *ReconciliationHandler) Unreconcile(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "流水 ID 非法")
		return
	}
	if err := services.Unreconcile(r.Context(), h.db, uint(id)); err != nil {
		if errors.Is(err, services.ErrStatementMissing) {
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
			return
		}
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "取消勾对失败")
		return
	}
	resp.OK(w, map[string]string{"message": "已取消勾对"})
}

// Summary 未达账项汇总
func (h *ReconciliationHandler) Summary(w http.ResponseWriter, r *http.Request) {
	res, err := services.ReconciliationSummary(r.Context(), h.db)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "查询未达账项失败")
		return
	}
	resp.OK(w, res)
}
