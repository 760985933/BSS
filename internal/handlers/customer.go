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

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// CustomerHandler 客户 + 联系人
type CustomerHandler struct {
	db  *gorm.DB
	svc *services.CustomerService
}

func NewCustomerHandler(db *gorm.DB) *CustomerHandler {
	return &CustomerHandler{db: db, svc: services.NewCustomerService(db)}
}

// List GET /api/v1/customers —— ScopeOwner 数据范围 + 筛选 + 真分页
func (h *CustomerHandler) List(w http.ResponseWriter, r *http.Request) {
	page, size := pageParams(r)
	q := r.URL.Query()
	base := h.db.WithContext(r.Context()).Model(&models.Customer{}).Preload("Owner")
	base = services.ScopeOwner(base, r.Context())
	if kw := q.Get("keyword"); kw != "" {
		base = base.Where("name LIKE ?", "%"+kw+"%")
	}
	for field, col := range map[string]string{"industry": "industry", "source": "source", "level": "level"} {
		if v := q.Get(field); v != "" {
			base = base.Where(col+" = ?", v)
		}
	}
	if v := q.Get("owner_id"); v != "" {
		if id, err := strconv.ParseUint(v, 10, 64); err == nil {
			base = base.Where("owner_id = ?", id)
		}
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "查询客户失败")
		return
	}
	var list []models.Customer
	if err := base.Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error; err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "查询客户失败")
		return
	}
	resp.OKPage(w, list, total, page, size)
}

// Create POST /api/v1/customers（admin/sales/sales_lead；负责人=当前用户）
func (h *CustomerHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in services.CustomerInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求格式错误")
		return
	}
	c := middleware.UserFrom(r.Context())
	cust, err := h.svc.Create(r.Context(), in, c.UserID)
	if err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, cust)
}

// Get GET /api/v1/customers/:id —— 行级数据范围校验
func (h *CustomerHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok || !h.canAccess(w, r, id) {
		return
	}
	cust, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, cust)
}

// Update PUT /api/v1/customers/:id（财务除外 + 行级范围）
func (h *CustomerHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok || !h.canAccess(w, r, id) {
		return
	}
	var in services.CustomerInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求格式错误")
		return
	}
	if err := h.svc.Update(r.Context(), id, in); err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, map[string]string{"message": "已保存"})
}

// Delete DELETE /api/v1/customers/:id —— 有下游数据禁删
func (h *CustomerHandler) Delete(w http.ResponseWriter, r *http.Request) {
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

// Transfer POST /api/v1/customers/:id/transfer {owner_id}
func (h *CustomerHandler) Transfer(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok || !h.canAccess(w, r, id) {
		return
	}
	var req struct {
		OwnerID uint `json:"owner_id,string"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OwnerID == 0 {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请选择目标负责人")
		return
	}
	if err := h.svc.Transfer(r.Context(), id, req.OwnerID); err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, map[string]string{"message": "已转移"})
}

// ---------- 联系人 ----------

func (h *CustomerHandler) ListContacts(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok || !h.canAccess(w, r, id) {
		return
	}
	list, err := h.svc.ListContacts(r.Context(), id)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "查询联系人失败")
		return
	}
	resp.OK(w, list)
}

func (h *CustomerHandler) CreateContact(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok || !h.canAccess(w, r, id) {
		return
	}
	var in services.ContactInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求格式错误")
		return
	}
	ct, err := h.svc.CreateContact(r.Context(), id, in)
	if err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, ct)
}

func (h *CustomerHandler) UpdateContact(w http.ResponseWriter, r *http.Request) {
	cid, err := strconv.ParseUint(chi.URLParam(r, "cid"), 10, 64)
	if err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "非法的联系人 ID")
		return
	}
	var in services.ContactInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求格式错误")
		return
	}
	if err := h.svc.UpdateContact(r.Context(), uint(cid), in); err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, map[string]string{"message": "已保存"})
}

func (h *CustomerHandler) DeleteContact(w http.ResponseWriter, r *http.Request) {
	cid, err := strconv.ParseUint(chi.URLParam(r, "cid"), 10, 64)
	if err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "非法的联系人 ID")
		return
	}
	if err := h.svc.DeleteContact(r.Context(), uint(cid)); err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, map[string]string{"message": "已删除"})
}

// ---------- 辅助 ----------

// canAccess 行级数据范围校验（取客户 owner 后按角色判定）
func (h *CustomerHandler) canAccess(w http.ResponseWriter, r *http.Request, id uint) bool {
	ownerID, err := h.svc.OwnerOf(r.Context(), id)
	if err != nil {
		resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, "客户不存在")
		return false
	}
	if !services.CanAccessOwner(h.db, r.Context(), ownerID) {
		resp.Fail(w, http.StatusForbidden, resp.CodeForbidden, "无权访问该客户（不在你的数据范围内）")
		return false
	}
	return true
}

func (h *CustomerHandler) failSvc(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrCustomerNameExists):
		resp.Fail(w, http.StatusConflict, resp.CodeConflict, err.Error())
	case errors.Is(err, services.ErrCustomerHasChildren):
		resp.Fail(w, http.StatusUnprocessableEntity, resp.CodeCustomerHasChildren, err.Error())
	case errors.Is(err, services.ErrCustomerMissing):
		resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
	default:
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
	}
}

func pageParams(r *http.Request) (page, size int) {
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	size, _ = strconv.Atoi(r.URL.Query().Get("size"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	return page, size
}
