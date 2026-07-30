package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"bss/internal/middleware"
	"bss/internal/pkg/resp"
	"bss/internal/services"
)

// InvoiceHandler 开票管理接口（M2-2）
type InvoiceHandler struct {
	svc *services.InvoiceService
}

func NewInvoiceHandler(svc *services.InvoiceService) *InvoiceHandler {
	return &InvoiceHandler{svc: svc}
}

// List GET /invoices
func (h *InvoiceHandler) List(w http.ResponseWriter, r *http.Request) {
	page, size := pageParams(r)
	contractID := r.URL.Query().Get("contract_id")
	status := r.URL.Query().Get("status")
	list, total, err := h.svc.List(r.Context(), page, size, contractID, status)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, map[string]any{"list": list, "total": total})
}

// Get GET /invoices/:id
func (h *InvoiceHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	inv, err := h.svc.Get(r.Context(), id)
	if err != nil {
		failInvoice(w, err)
		return
	}
	resp.OK(w, inv)
}

// Create POST /invoices —— 新建待开发票（管理员/财务/销售主管）
func (h *InvoiceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in services.InvoiceInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求体解析失败")
		return
	}
	claims := middleware.UserFrom(r.Context())
	inv, err := h.svc.Create(r.Context(), in, claims.UserID)
	if err != nil {
		failInvoice(w, err)
		return
	}
	resp.OK(w, inv)
}

// Issue POST /invoices/:id/issue —— 开票
func (h *InvoiceHandler) Issue(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.svc.Issue(r.Context(), id); err != nil {
		failInvoice(w, err)
		return
	}
	resp.OK(w, map[string]string{"status": "issued"})
}

// Void POST /invoices/:id/void —— 作废
func (h *InvoiceHandler) Void(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.svc.Void(r.Context(), id); err != nil {
		failInvoice(w, err)
		return
	}
	resp.OK(w, map[string]string{"status": "voided"})
}

// Update PUT /invoices/:id —— 编辑待开发票
func (h *InvoiceHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var in services.InvoiceInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求体解析失败")
		return
	}
	if err := h.svc.Update(r.Context(), id, in); err != nil {
		failInvoice(w, err)
		return
	}
	resp.OK(w, map[string]string{"message": "ok"})
}

// Delete DELETE /invoices/:id —— 删除待开发票
func (h *InvoiceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		failInvoice(w, err)
		return
	}
	resp.OK(w, map[string]string{"message": "ok"})
}

func failInvoice(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrInvoiceMissing):
		resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
	case errors.Is(err, services.ErrInvoiceAmountExceed):
		resp.Fail(w, http.StatusUnprocessableEntity, resp.CodeInvoiceAmountExceed, err.Error())
	case errors.Is(err, services.ErrInvoiceInvalidState),
		errors.Is(err, services.ErrInvoiceNotDraft),
		errors.Is(err, services.ErrInvoiceNegativeAmount):
		resp.Fail(w, http.StatusUnprocessableEntity, resp.CodeInvoiceInvalidState, err.Error())
	default:
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
	}
}
