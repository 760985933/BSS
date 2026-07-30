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

// EmployeeHandler 员工档案 + 数据字典
type EmployeeHandler struct {
	db  *gorm.DB
	svc *services.EmployeeService
}

func NewEmployeeHandler(db *gorm.DB) *EmployeeHandler {
	return &EmployeeHandler{db: db, svc: services.NewEmployeeService(db)}
}

// List GET /api/v1/employees —— 数据范围：sales_lead 仅本部门，其余角色全量只读（PRD §6）
func (h *EmployeeHandler) List(w http.ResponseWriter, r *http.Request) {
	c := middleware.UserFrom(r.Context())
	var list []models.Employee
	q := h.db.WithContext(r.Context()).Order("id")
	if c.Role == models.RoleSalesLead {
		q = q.Where("dept = ?", c.Dept)
	}
	if kw := r.URL.Query().Get("keyword"); kw != "" {
		q = q.Where("name LIKE ? OR email LIKE ?", "%"+kw+"%", "%"+kw+"%")
	}
	if err := q.Find(&list).Error; err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "查询员工列表失败")
		return
	}
	resp.OKPage(w, list, int64(len(list)), 1, len(list))
}

type createReq struct {
	services.EmployeeInput
	Email string `json:"email"`
}

// Create POST /api/v1/employees（admin/hr）
func (h *EmployeeHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求格式错误")
		return
	}
	emp, err := h.svc.Create(r.Context(), req.EmployeeInput, req.Email)
	if err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, map[string]any{
		"employee":         emp,
		"initial_password": services.InitialPassword,
		"message":          "员工已创建，请将初始密码告知本人（首次登录强制改密）",
	})
}

// Update PUT /api/v1/employees/:id（admin/hr）
func (h *EmployeeHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var in services.EmployeeInput
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

// SetStatus POST /api/v1/employees/:id/status  {active:true|false}（admin/hr）
func (h *EmployeeHandler) SetStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求格式错误")
		return
	}
	c := middleware.UserFrom(r.Context())
	if err := h.svc.SetStatus(r.Context(), id, c.UserID, req.Active); err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, map[string]string{"message": "状态已更新"})
}

// ResetPassword POST /api/v1/employees/:id/reset-password（仅 admin，PRD §4.1）
func (h *EmployeeHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.svc.ResetPassword(r.Context(), id); err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, map[string]any{
		"initial_password": services.InitialPassword,
		"message":          "密码已重置为初始密码，请告知本人（首次登录强制改密）",
	})
}

// OffboardPreview GET /api/v1/employees/:id/offboard-preview —— 交接前预览待转移数据量
func (h *EmployeeHandler) OffboardPreview(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	prev, err := h.svc.OffboardPreview(r.Context(), id)
	if err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, prev)
}

// Offboard POST /api/v1/employees/:id/offboard  {successor_id} —— 转移名下数据并停用
func (h *EmployeeHandler) Offboard(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req struct {
		SuccessorID uint `json:"successor_id,string"` // 前端以字符串传递，与 customer.Transfer 一致
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求格式错误")
		return
	}
	c := middleware.UserFrom(r.Context())
	res, err := h.svc.Offboard(r.Context(), id, req.SuccessorID, c.UserID)
	if err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, map[string]any{
		"result":  res,
		"message": "已将该员工名下数据转移给交接人并停用账号",
	})
}

// ---------- 数据字典 ----------

// ListDicts GET /api/v1/dicts?type=dept（全员可读）
func (h *EmployeeHandler) ListDicts(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.ListDict(r.Context(), r.URL.Query().Get("type"))
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "查询字典失败")
		return
	}
	resp.OK(w, list)
}

// AddDict POST /api/v1/dicts（admin）
func (h *EmployeeHandler) AddDict(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求格式错误")
		return
	}
	d, err := h.svc.AddDict(r.Context(), req.Type, req.Value)
	if err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, d)
}

// RemoveDict DELETE /api/v1/dicts/:id（admin）
func (h *EmployeeHandler) RemoveDict(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.svc.RemoveDict(r.Context(), id); err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, map[string]string{"message": "已删除"})
}

// ---------- 辅助 ----------

func parseID(w http.ResponseWriter, r *http.Request) (uint, bool) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id == 0 {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "非法的 ID")
		return 0, false
	}
	return uint(id), true
}

// failSvc 业务错误统一映射（用户可读信息直出）
func (h *EmployeeHandler) failSvc(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrEmailExists), errors.Is(err, services.ErrInvalidRole):
		resp.Fail(w, http.StatusConflict, resp.CodeConflict, err.Error())
	case errors.Is(err, services.ErrLastAdmin), errors.Is(err, services.ErrCannotSelfOp),
		errors.Is(err, services.ErrOffboardSameEmployee), errors.Is(err, services.ErrSuccessorMissing),
		errors.Is(err, services.ErrSuccessorNotActive):
		resp.Fail(w, http.StatusUnprocessableEntity, resp.CodeBadRequest, err.Error())
	case errors.Is(err, services.ErrEmployeeMissing):
		resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
	default:
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
	}
}
