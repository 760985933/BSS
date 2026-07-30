package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"bss/internal/middleware"
	"bss/internal/models"
	"bss/internal/pkg/resp"
	"bss/internal/services"

	"gorm.io/gorm"
)

// PoolHandler 客户公海池（M3-1）
type PoolHandler struct {
	db  *gorm.DB
	svc *services.PoolService
}

func NewPoolHandler(db *gorm.DB) *PoolHandler {
	return &PoolHandler{db: db, svc: services.NewPoolService(db)}
}

// List GET /api/v1/customer-pool —— 公海客户列表（登录即可见，无 ScopeOwner）
func (h *PoolHandler) List(w http.ResponseWriter, r *http.Request) {
	page, size := pageParams(r)
	q := r.URL.Query()
	list, total, err := h.svc.List(r.Context(), services.PoolFilter{
		Keyword:  q.Get("keyword"),
		Industry: q.Get("industry"),
		Source:   q.Get("source"),
		Level:    q.Get("level"),
		Page:     page, Size: size,
	})
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "查询公海客户失败")
		return
	}
	resp.OKPage(w, list, total, page, size)
}

// Claim POST /api/v1/customers/:id/claim —— 销售领取公海客户
func (h *PoolHandler) Claim(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	c := middleware.UserFrom(r.Context())
	if err := h.svc.Claim(r.Context(), id, c.UserID); err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, map[string]string{"message": "领取成功，该客户已归入你的名下"})
}

// Release POST /api/v1/customers/:id/release {reason} —— 释放到公海（需行级权限）
func (h *PoolHandler) Release(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if !h.canAccessCustomer(w, r, id) {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req) // reason 可选
	c := middleware.UserFrom(r.Context())
	if err := h.svc.Release(r.Context(), id, c.UserID, req.Reason); err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, map[string]string{"message": "已释放到公海"})
}

// Assign POST /api/v1/customer-pool/:id/assign {owner_id} —— 管理员/主管指派
func (h *PoolHandler) Assign(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req struct {
		OwnerID uint `json:"owner_id,string"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OwnerID == 0 {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请选择接收人")
		return
	}
	c := middleware.UserFrom(r.Context())
	if err := h.svc.Assign(r.Context(), id, req.OwnerID, c.UserID); err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, map[string]string{"message": "已指派"})
}

// Recycle POST /api/v1/customer-pool/recycle?dry_run=1 —— 手动触发回收（admin/sales_lead）
func (h *PoolHandler) Recycle(w http.ResponseWriter, r *http.Request) {
	dry := r.URL.Query().Get("dry_run") == "1"
	res, err := h.svc.Recycle(r.Context(), time.Now(), dry)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "回收执行失败")
		return
	}
	resp.OK(w, res)
}

// Logs GET /api/v1/customers/:id/pool-logs —— 公海流水
func (h *PoolHandler) Logs(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	list, err := h.svc.Logs(r.Context(), id)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "查询公海流水失败")
		return
	}
	resp.OK(w, list)
}

// GetSettings GET /api/v1/customer-pool/settings
func (h *PoolHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.Settings(r.Context())
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "查询公海规则失败")
		return
	}
	resp.OK(w, st)
}

// UpdateSettings PUT /api/v1/customer-pool/settings（admin）
func (h *PoolHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var in services.PoolSettingsInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求格式错误")
		return
	}
	st, err := h.svc.UpdateSettings(r.Context(), in)
	if err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, st)
}

// ---------- 辅助 ----------

// canAccessCustomer 释放操作的行级权限：本人 / 本部门主管 / 管理员
func (h *PoolHandler) canAccessCustomer(w http.ResponseWriter, r *http.Request, id uint) bool {
	var cust models.Customer
	if err := h.db.WithContext(r.Context()).Select("owner_id").Take(&cust, id).Error; err != nil {
		resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, "客户不存在")
		return false
	}
	allowed, err := services.CanAccessOwner(h.db, r.Context(), cust.OwnerID)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "数据范围校验失败")
		return false
	}
	if !allowed {
		resp.Fail(w, http.StatusForbidden, resp.CodeForbidden, "无权操作该客户（不在你的数据范围内）")
		return false
	}
	return true
}

func (h *PoolHandler) failSvc(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrCustomerMissing):
		resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
	case errors.Is(err, services.ErrNotInPool), errors.Is(err, services.ErrAlreadyInPool),
		errors.Is(err, services.ErrClaimLimit), errors.Is(err, services.ErrReleaseHasDeal),
		errors.Is(err, services.ErrReleaseHasContra):
		resp.Fail(w, http.StatusUnprocessableEntity, resp.CodeBadRequest, err.Error())
	default:
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
	}
}
