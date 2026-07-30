package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"bss/internal/middleware"
	"bss/internal/pkg/resp"
	"bss/internal/services"
)

// ApprovalHandler 审批流接口（M2-1）
type ApprovalHandler struct {
	svc *services.ApprovalService
}

func NewApprovalHandler(svc *services.ApprovalService) *ApprovalHandler {
	return &ApprovalHandler{svc: svc}
}

// List GET /approvals
func (h *ApprovalHandler) List(w http.ResponseWriter, r *http.Request) {
	page, size := pageParams(r)
	status := r.URL.Query().Get("status")
	kind := r.URL.Query().Get("kind")
	entityType := r.URL.Query().Get("entity_type")
	list, total, err := h.svc.List(r.Context(), page, size, status, kind, entityType)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, map[string]any{"list": list, "total": total})
}

// Get GET /approvals/:id
func (h *ApprovalHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	ap, err := h.svc.Get(r.Context(), id)
	if err != nil {
		failApproval(w, err)
		return
	}
	resp.OK(w, ap)
}

// Create POST /approvals —— 提交审批（销售/销售主管/管理员）
func (h *ApprovalHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in services.ApprovalInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求体解析失败")
		return
	}
	claims := middleware.UserFrom(r.Context())
	ap, err := h.svc.Create(r.Context(), in, claims.UserID)
	if err != nil {
		failApproval(w, err)
		return
	}
	resp.OK(w, ap)
}

// Approve POST /approvals/:id/approve —— 审批通过（管理员/财务/销售主管）
func (h *ApprovalHandler) Approve(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	claims := middleware.UserFrom(r.Context())
	if err := h.svc.Approve(r.Context(), id, claims.UserID); err != nil {
		failApproval(w, err)
		return
	}
	resp.OK(w, map[string]string{"status": "approved"})
}

// Reject POST /approvals/:id/reject —— 审批驳回（管理员/财务/销售主管）
func (h *ApprovalHandler) Reject(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	claims := middleware.UserFrom(r.Context())
	if err := h.svc.Reject(r.Context(), id, claims.UserID, body.Reason); err != nil {
		failApproval(w, err)
		return
	}
	resp.OK(w, map[string]string{"status": "rejected"})
}

// failApproval 统一映射审批业务错误到 HTTP/业务码
func failApproval(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrApprovalMissing):
		resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
	case errors.Is(err, services.ErrApprovalNotPending):
		resp.Fail(w, http.StatusUnprocessableEntity, resp.CodeApprovalNotPending, err.Error())
	case errors.Is(err, services.ErrApprovalInvalidKind),
		errors.Is(err, services.ErrApprovalEntityState),
		errors.Is(err, services.ErrApprovalEntityMissing),
		errors.Is(err, services.ErrApprovalRejectReasonRequired):
		resp.Fail(w, http.StatusUnprocessableEntity, resp.CodeApprovalInvalid, err.Error())
	default:
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
	}
}
