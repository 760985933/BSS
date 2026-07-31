package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"bss/internal/middleware"
	"bss/internal/pkg/resp"
	"bss/internal/services"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// ProjectHandler 项目/交付管理（M3-3）
type ProjectHandler struct {
	db  *gorm.DB
	svc *services.ProjectService
}

func NewProjectHandler(db *gorm.DB) *ProjectHandler {
	return &ProjectHandler{db: db, svc: services.NewProjectService(db)}
}

// List GET /api/v1/projects
func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	page, size := pageParams(r)
	q := r.URL.Query()
	list, total, err := h.svc.List(r.Context(), page, size, q.Get("keyword"), q.Get("status"), q.Get("owner_id"))
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "查询项目失败")
		return
	}
	resp.OKPage(w, list, total, page, size)
}

// Create POST /api/v1/projects
func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in services.ProjectInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求格式错误")
		return
	}
	c := middleware.UserFrom(r.Context())
	p, err := h.svc.Create(r.Context(), in, c.UserID)
	if err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, p)
}

// Get GET /api/v1/projects/:id（含成员与任务）
func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	p, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, p)
}

// Update PUT /api/v1/projects/:id
func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok || !h.canMutate(w, r, id) {
		return
	}
	var in services.ProjectInput
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

// Delete DELETE /api/v1/projects/:id
func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok || !h.canMutate(w, r, id) {
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, map[string]string{"message": "已删除"})
}

// ---------- 成员（人天） ----------

func (h *ProjectHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	list, err := h.svc.ListMembers(r.Context(), id)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "查询成员失败")
		return
	}
	resp.OK(w, list)
}

func (h *ProjectHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok || !h.canMutate(w, r, id) {
		return
	}
	var in services.MemberInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求格式错误")
		return
	}
	m, err := h.svc.AddMember(r.Context(), id, in)
	if err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, m)
}

func (h *ProjectHandler) UpdateMember(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok || !h.canMutate(w, r, id) {
		return
	}
	mid, ok := parseParamID(w, r, "mid")
	if !ok {
		return
	}
	var in services.MemberInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求格式错误")
		return
	}
	if err := h.svc.UpdateMember(r.Context(), mid, in); err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, map[string]string{"message": "已保存"})
}

func (h *ProjectHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok || !h.canMutate(w, r, id) {
		return
	}
	mid, ok := parseParamID(w, r, "mid")
	if !ok {
		return
	}
	if err := h.svc.RemoveMember(r.Context(), mid); err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, map[string]string{"message": "已移除"})
}

// ---------- 任务 / 里程碑 ----------

func (h *ProjectHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	list, err := h.svc.ListTasks(r.Context(), id)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "查询任务失败")
		return
	}
	resp.OK(w, list)
}

func (h *ProjectHandler) AddTask(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok || !h.canMutate(w, r, id) {
		return
	}
	var in services.TaskInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求格式错误")
		return
	}
	t, err := h.svc.AddTask(r.Context(), id, in)
	if err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, t)
}

func (h *ProjectHandler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok || !h.canMutate(w, r, id) {
		return
	}
	tid, ok := parseParamID(w, r, "tid")
	if !ok {
		return
	}
	var in services.TaskInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求格式错误")
		return
	}
	if err := h.svc.UpdateTask(r.Context(), tid, in); err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, map[string]string{"message": "已保存"})
}

func (h *ProjectHandler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok || !h.canMutate(w, r, id) {
		return
	}
	tid, ok := parseParamID(w, r, "tid")
	if !ok {
		return
	}
	if err := h.svc.DeleteTask(r.Context(), tid); err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, map[string]string{"message": "已删除"})
}

// ---------- 辅助 ----------

// canMutate 项目写操作鉴权（owner/成员 或 管理/监督角色）
func (h *ProjectHandler) canMutate(w http.ResponseWriter, r *http.Request, projectID uint) bool {
	ok, err := h.svc.AccessOK(r.Context(), projectID)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "权限校验失败")
		return false
	}
	if !ok {
		resp.Fail(w, http.StatusForbidden, resp.CodeForbidden, services.ErrNoProjectAccess.Error())
		return false
	}
	return true
}

func parseParamID(w http.ResponseWriter, r *http.Request, key string) (uint, bool) {
	v, err := strconv.ParseUint(chi.URLParam(r, key), 10, 64)
	if err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "非法的 ID")
		return 0, false
	}
	return uint(v), true
}

func (h *ProjectHandler) failSvc(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrProjectMissing):
		resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
	case errors.Is(err, services.ErrProjectNameEmpty):
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
	case errors.Is(err, services.ErrMemberMissing), errors.Is(err, services.ErrTaskMissing):
		resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
	case errors.Is(err, services.ErrEmployeeMissing):
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
	case errors.Is(err, services.ErrNoProjectAccess):
		resp.Fail(w, http.StatusForbidden, resp.CodeForbidden, err.Error())
	default:
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
	}
}
