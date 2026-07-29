package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"bss/internal/middleware"
	"bss/internal/models"
	"bss/internal/pkg/resp"
	"bss/internal/services"

	"gorm.io/gorm"
)

// DealHandler 商单
type DealHandler struct {
	db  *gorm.DB
	svc *services.DealService
}

func NewDealHandler(db *gorm.DB) *DealHandler {
	return &DealHandler{db: db, svc: services.NewDealService(db)}
}

// List GET /api/v1/deals —— ScopeOwner + 筛选 + 分页
func (h *DealHandler) List(w http.ResponseWriter, r *http.Request) {
	page, size := pageParams(r)
	q := r.URL.Query()
	base := h.db.WithContext(r.Context()).Model(&models.Deal{}).Preload("Customer").Preload("Owner")
	base = services.ScopeOwner(base, r.Context())
	if kw := q.Get("keyword"); kw != "" {
		base = base.Where("title LIKE ?", "%"+kw+"%")
	}
	if v := q.Get("status"); v != "" {
		base = base.Where("status = ?", v)
	}
	if v := q.Get("customer_id"); v != "" {
		if id, err := strconv.ParseUint(v, 10, 64); err == nil {
			base = base.Where("customer_id = ?", id)
		}
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "查询商单失败")
		return
	}
	var list []models.Deal
	if err := base.Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "查询商单失败")
		return
	}
	resp.OKPage(w, list, total, page, size)
}

// Create POST /api/v1/deals
func (h *DealHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in services.DealInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求格式错误")
		return
	}
	c := middleware.UserFrom(r.Context())
	d, err := h.svc.Create(r.Context(), in, c.UserID)
	if err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, d)
}

// Get GET /api/v1/deals/:id
func (h *DealHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok || !h.canAccess(w, r, id) {
		return
	}
	d, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, d)
}

// Update PUT /api/v1/deals/:id
func (h *DealHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok || !h.canAccess(w, r, id) {
		return
	}
	var in services.DealInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求格式错误")
		return
	}
	// 终态锁定判定需要当前状态，Update 内部已处理；此处仅补充客户归属一致性
	if err := h.svc.Update(r.Context(), id, in); err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, map[string]string{"message": "已保存"})
}

// ChangeStatus POST /api/v1/deals/:id/status  {to, lost_reason, force}
func (h *DealHandler) ChangeStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok || !h.canAccess(w, r, id) {
		return
	}
	var req struct {
		To         string `json:"to"`
		LostReason string `json:"lost_reason"`
		Force      bool   `json:"force"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求格式错误")
		return
	}
	d, err := h.svc.ChangeStatus(r.Context(), id, req.To, req.LostReason, req.Force)
	if errors.Is(err, services.ErrExitWarning) {
		resp.FailWarning(w, resp.CodeExitCriteriaUnmet, "商单金额为 0，确认推进到该阶段？")
		return
	}
	if err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, d)
}

// Delete DELETE /api/v1/deals/:id
func (h *DealHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok || !h.canAccess(w, r, id) {
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, map[string]string{"message": "已删除"})
}

// Forecast GET /api/v1/deals/forecast —— 加权预测（ScopeOwner 范围内进行中商单）
func (h *DealHandler) Forecast(w http.ResponseWriter, r *http.Request) {
	scoped := services.ScopeOwner(h.db, r.Context())
	result, err := h.svc.Forecast(r.Context(), scoped)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "计算加权预测失败")
		return
	}
	resp.OK(w, result)
}

// ---------- 辅助 ----------

func (h *DealHandler) canAccess(w http.ResponseWriter, r *http.Request, id uint) bool {
	ownerID, err := h.svc.OwnerOf(r.Context(), id)
	if err != nil {
		resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, "商单不存在")
		return false
	}
	allowed, err := services.CanAccessOwner(h.db, r.Context(), ownerID)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "数据范围校验失败")
		return false
	}
	if !allowed {
		resp.Fail(w, http.StatusForbidden, resp.CodeForbidden, "无权访问该商单（不在你的数据范围内）")
		return false
	}
	return true
}

func (h *DealHandler) failSvc(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrInvalidTransition), errors.Is(err, services.ErrLostReasonRequired),
		errors.Is(err, services.ErrDealLocked), errors.Is(err, services.ErrDealHasContract),
		errors.Is(err, services.ErrDealClosed):
		resp.Fail(w, http.StatusUnprocessableEntity, resp.CodeInvalidStateTransit, err.Error())
	case errors.Is(err, services.ErrDealMissing):
		resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
	default:
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
	}
}
