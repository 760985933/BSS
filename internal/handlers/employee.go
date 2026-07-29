package handlers

import (
	"net/http"

	"bss/internal/middleware"
	"bss/internal/models"
	"bss/internal/pkg/resp"

	"gorm.io/gorm"
)

// EmployeeHandler 员工查询（M0：只读列表，验证 RBAC 数据范围；CRUD 见 Sprint 1）
type EmployeeHandler struct {
	db *gorm.DB
}

func NewEmployeeHandler(db *gorm.DB) *EmployeeHandler {
	return &EmployeeHandler{db: db}
}

// List GET /api/v1/employees —— 数据范围：sales_lead 仅本部门，其余角色全量只读
func (h *EmployeeHandler) List(w http.ResponseWriter, r *http.Request) {
	c := middleware.UserFrom(r.Context())
	var list []models.Employee
	q := h.db.WithContext(r.Context()).Order("id")
	if c.Role == models.RoleSalesLead {
		q = q.Where("dept = ?", c.Dept)
	}
	if err := q.Find(&list).Error; err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "查询员工列表失败")
		return
	}
	resp.OKPage(w, list, int64(len(list)), 1, len(list))
}
