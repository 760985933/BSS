package handlers

import (
	"net/http"
	"strconv"

	"bss/internal/pkg/resp"
	"bss/internal/services"
)

// AuditHandler 审计查询接口（M2-4）。仅管理/监督角色可访问（路由层 RequireRole 把关）。
type AuditHandler struct {
	svc *services.AuditQueryService
}

func NewAuditHandler(svc *services.AuditQueryService) *AuditHandler {
	return &AuditHandler{svc: svc}
}

// List GET /audit-logs?entity_type=&entity_id=&action=&operator_id=&start=&end=&page=&size=
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	page, size := pageParams(r)
	q := services.AuditQuery{
		EntityType: r.URL.Query().Get("entity_type"),
		Action:     r.URL.Query().Get("action"),
		Start:      r.URL.Query().Get("start"),
		End:        r.URL.Query().Get("end"),
		Page:       page,
		Size:       size,
	}
	if v := r.URL.Query().Get("entity_id"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			q.EntityID = uint(n)
		}
	}
	if v := r.URL.Query().Get("operator_id"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			q.OperatorID = uint(n)
		}
	}
	list, total, err := h.svc.List(r.Context(), q)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OKPage(w, list, total, page, size)
}
