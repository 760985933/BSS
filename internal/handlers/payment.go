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

// PaymentHandler 回款计划/记录/汇总
type PaymentHandler struct {
	db  *gorm.DB
	svc *services.PaymentService
}

func NewPaymentHandler(db *gorm.DB) *PaymentHandler {
	return &PaymentHandler{db: db, svc: services.NewPaymentService(db)}
}

// canAccessContract 行级范围校验（与合同同 owner）
func (h *PaymentHandler) canAccessContract(w http.ResponseWriter, r *http.Request, contractID uint) bool {
	var ownerID uint
	if err := h.db.Model(&models.Contract{}).Where("id = ?", contractID).Select("owner_id").Scan(&ownerID).Error; err != nil || ownerID == 0 {
		resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, "合同不存在")
		return false
	}
	allowed, err := services.CanAccessOwner(h.db, r.Context(), ownerID)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "数据范围校验失败")
		return false
	}
	if !allowed {
		resp.Fail(w, http.StatusForbidden, resp.CodeForbidden, "无权访问该合同回款（不在你的数据范围内）")
		return false
	}
	return true
}

func parseUintParam(w http.ResponseWriter, r *http.Request, name string) (uint, bool) {
	n, err := strconv.ParseUint(chi.URLParam(r, name), 10, 64)
	if err != nil || n == 0 {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "参数非法")
		return 0, false
	}
	return uint(n), true
}

// ListPlans GET /api/v1/contracts/:id/plans
func (h *PaymentHandler) ListPlans(w http.ResponseWriter, r *http.Request) {
	contractID, ok := parseID(w, r)
	if !ok || !h.canAccessContract(w, r, contractID) {
		return
	}
	list, err := h.svc.ListPlans(r.Context(), contractID)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "查询回款计划失败")
		return
	}
	resp.OK(w, list)
}

// CreatePlan POST /api/v1/contracts/:id/plans
func (h *PaymentHandler) CreatePlan(w http.ResponseWriter, r *http.Request) {
	contractID, ok := parseID(w, r)
	if !ok || !h.canAccessContract(w, r, contractID) {
		return
	}
	var in services.PlanInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求格式错误")
		return
	}
	in.ContractID = contractID
	p, err := h.svc.CreatePlan(r.Context(), in)
	if err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, p)
}

// UpdatePlan PUT /api/v1/contracts/:id/plans/:pid
func (h *PaymentHandler) UpdatePlan(w http.ResponseWriter, r *http.Request) {
	contractID, ok := parseID(w, r)
	if !ok || !h.canAccessContract(w, r, contractID) {
		return
	}
	planID, ok := parseUintParam(w, r, "pid")
	if !ok {
		return
	}
	var in services.PlanInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求格式错误")
		return
	}
	in.ContractID = contractID
	if err := h.svc.UpdatePlan(r.Context(), planID, contractID, in); err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, map[string]string{"message": "已保存"})
}

// DeletePlan DELETE /api/v1/contracts/:id/plans/:pid
func (h *PaymentHandler) DeletePlan(w http.ResponseWriter, r *http.Request) {
	contractID, ok := parseID(w, r)
	if !ok || !h.canAccessContract(w, r, contractID) {
		return
	}
	planID, ok := parseUintParam(w, r, "pid")
	if !ok {
		return
	}
	if err := h.svc.DeletePlan(r.Context(), planID, contractID); err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, map[string]string{"message": "已删除"})
}

// ListRecords GET /api/v1/contracts/:id/records
func (h *PaymentHandler) ListRecords(w http.ResponseWriter, r *http.Request) {
	contractID, ok := parseID(w, r)
	if !ok || !h.canAccessContract(w, r, contractID) {
		return
	}
	list, err := h.svc.ListRecords(r.Context(), contractID)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "查询回款记录失败")
		return
	}
	resp.OK(w, list)
}

// CreateRecords POST /api/v1/contracts/:id/records {records:[...]}
func (h *PaymentHandler) CreateRecords(w http.ResponseWriter, r *http.Request) {
	contractID, ok := parseID(w, r)
	if !ok || !h.canAccessContract(w, r, contractID) {
		return
	}
	var req struct {
		Records []services.RecordInput `json:"records"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求格式错误")
		return
	}
	c := middleware.UserFrom(r.Context())
	if err := h.svc.CreateRecords(r.Context(), contractID, req.Records, c.UserID); err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, map[string]string{"message": "已录入回款"})
}

// DeleteRecord DELETE /api/v1/contracts/:id/records/:rid
func (h *PaymentHandler) DeleteRecord(w http.ResponseWriter, r *http.Request) {
	contractID, ok := parseID(w, r)
	if !ok || !h.canAccessContract(w, r, contractID) {
		return
	}
	recordID, ok := parseUintParam(w, r, "rid")
	if !ok {
		return
	}
	c := middleware.UserFrom(r.Context())
	if err := h.svc.DeleteRecord(r.Context(), recordID, contractID, c.UserID); err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, map[string]string{"message": "已删除"})
}

// Summary GET /api/v1/contracts/:id/payment-summary
func (h *PaymentHandler) Summary(w http.ResponseWriter, r *http.Request) {
	contractID, ok := parseID(w, r)
	if !ok || !h.canAccessContract(w, r, contractID) {
		return
	}
	sum, err := h.svc.Summary(r.Context(), contractID)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "查询回款汇总失败")
		return
	}
	resp.OK(w, sum)
}

func (h *PaymentHandler) failSvc(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrPlanAmountExceed):
		resp.Fail(w, http.StatusUnprocessableEntity, resp.CodePlanAmountExceed, err.Error())
	case errors.Is(err, services.ErrPlanLocked):
		resp.Fail(w, http.StatusUnprocessableEntity, resp.CodePlanLocked, err.Error())
	case errors.Is(err, services.ErrPlanMissing), errors.Is(err, services.ErrRecordMissing), errors.Is(err, services.ErrContractMissing):
		resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
	case errors.Is(err, services.ErrRecordPlanMismatch), errors.Is(err, services.ErrRecordInvalid), errors.Is(err, services.ErrPlanInvalid):
		resp.Fail(w, http.StatusUnprocessableEntity, resp.CodeBadRequest, err.Error())
	default:
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
	}
}
