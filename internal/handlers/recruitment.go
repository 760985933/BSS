package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"bss/internal/middleware"
	"bss/internal/pkg/code"
	"bss/internal/pkg/resp"
	"bss/internal/services"
)

// RecruitmentHandler 招聘管理（M6-S1，限 admin/hr）
type RecruitmentHandler struct {
	db  *gorm.DB
	gen *code.Generator
}

// NewRecruitmentHandler 构造
func NewRecruitmentHandler(db *gorm.DB) *RecruitmentHandler {
	return &RecruitmentHandler{db: db, gen: code.NewGenerator(db)}
}

// ownerID 从登录态取当前用户 ID
func (h *RecruitmentHandler) ownerID(r *http.Request) uint {
	if c := middleware.UserFrom(r.Context()); c != nil {
		return c.UserID
	}
	return 0
}

// ---------- 招聘职位 ----------

// ListJobPosts 职位列表
func (h *RecruitmentHandler) ListJobPosts(w http.ResponseWriter, r *http.Request) {
	keyword := r.URL.Query().Get("keyword")
	status := r.URL.Query().Get("status")
	rows, err := services.ListJobPosts(r.Context(), h.db, keyword, status)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "查询失败")
		return
	}
	resp.OK(w, rows)
}

// CreateJobPost 创建职位（自动生成 JP- 单号）
func (h *RecruitmentHandler) CreateJobPost(w http.ResponseWriter, r *http.Request) {
	var in services.JobPostInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求参数错误")
		return
	}
	jp, err := services.CreateJobPost(r.Context(), h.db, h.gen, in, h.ownerID(r))
	if err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
		return
	}
	resp.OK(w, map[string]any{"job_post": jp, "message": "职位已创建"})
}

// GetJobPost 职位详情
func (h *RecruitmentHandler) GetJobPost(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "职位 ID 非法")
		return
	}
	jp, err := services.GetJobPost(r.Context(), h.db, uint(id))
	if err != nil {
		if errors.Is(err, services.ErrJobPostMissing) {
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
			return
		}
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "查询失败")
		return
	}
	resp.OK(w, jp)
}

// UpdateJobPost 更新职位
func (h *RecruitmentHandler) UpdateJobPost(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "职位 ID 非法")
		return
	}
	var in services.JobPostInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求参数错误")
		return
	}
	if err := services.UpdateJobPost(r.Context(), h.db, uint(id), in); err != nil {
		if errors.Is(err, services.ErrJobPostMissing) {
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
			return
		}
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "更新失败")
		return
	}
	resp.OK(w, map[string]string{"message": "职位已更新"})
}

// DeleteJobPost 删除职位（软删 + 名下候选人解除关联）
func (h *RecruitmentHandler) DeleteJobPost(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "职位 ID 非法")
		return
	}
	if err := services.DeleteJobPost(r.Context(), h.db, uint(id)); err != nil {
		if errors.Is(err, services.ErrJobPostMissing) {
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
			return
		}
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "删除失败")
		return
	}
	resp.OK(w, map[string]string{"message": "职位已删除"})
}

// ---------- 候选人 ----------

// ListCandidates 候选人列表
func (h *RecruitmentHandler) ListCandidates(w http.ResponseWriter, r *http.Request) {
	keyword := r.URL.Query().Get("keyword")
	stage := r.URL.Query().Get("stage")
	jobPostID, _ := strconv.ParseUint(r.URL.Query().Get("job_post_id"), 10, 64)
	rows, err := services.ListCandidates(r.Context(), h.db, keyword, uint(jobPostID), stage)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "查询失败")
		return
	}
	resp.OK(w, rows)
}

// Funnel 招聘漏斗各阶段计数
func (h *RecruitmentHandler) Funnel(w http.ResponseWriter, r *http.Request) {
	jobPostID, _ := strconv.ParseUint(r.URL.Query().Get("job_post_id"), 10, 64)
	stats, err := services.CandidateFunnelStats(r.Context(), h.db, uint(jobPostID))
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "查询失败")
		return
	}
	resp.OK(w, stats)
}

// CreateCandidate 添加候选人
func (h *RecruitmentHandler) CreateCandidate(w http.ResponseWriter, r *http.Request) {
	var in services.CandidateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求参数错误")
		return
	}
	c, err := services.CreateCandidate(r.Context(), h.db, in, h.ownerID(r))
	if err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
		return
	}
	resp.OK(w, map[string]any{"candidate": c, "message": "候选人已添加"})
}

// GetCandidate 候选人详情
func (h *RecruitmentHandler) GetCandidate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "候选人 ID 非法")
		return
	}
	c, err := services.GetCandidate(r.Context(), h.db, uint(id))
	if err != nil {
		if errors.Is(err, services.ErrCandidateMissing) {
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
			return
		}
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "查询失败")
		return
	}
	resp.OK(w, c)
}

// UpdateCandidate 更新候选人
func (h *RecruitmentHandler) UpdateCandidate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "候选人 ID 非法")
		return
	}
	var in services.CandidateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求参数错误")
		return
	}
	if err := services.UpdateCandidate(r.Context(), h.db, uint(id), in); err != nil {
		if errors.Is(err, services.ErrCandidateMissing) || errors.Is(err, services.ErrJobPostMissing) {
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
			return
		}
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "更新失败")
		return
	}
	resp.OK(w, map[string]string{"message": "候选人已更新"})
}

// DeleteCandidate 删除候选人（软删）
func (h *RecruitmentHandler) DeleteCandidate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "候选人 ID 非法")
		return
	}
	if err := services.DeleteCandidate(r.Context(), h.db, uint(id)); err != nil {
		if errors.Is(err, services.ErrCandidateMissing) {
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
			return
		}
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "删除失败")
		return
	}
	resp.OK(w, map[string]string{"message": "候选人已删除"})
}

// AdvanceCandidate 候选人阶段流转（回退/跳级需 force）
func (h *RecruitmentHandler) AdvanceCandidate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "候选人 ID 非法")
		return
	}
	var body struct {
		Stage string `json:"stage"`
		Force  bool   `json:"force"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Stage == "" {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "目标阶段必填")
		return
	}
	if err := services.AdvanceCandidate(r.Context(), h.db, uint(id), body.Stage, body.Force); err != nil {
		switch {
		case errors.Is(err, services.ErrCandidateMissing):
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
		case errors.Is(err, services.ErrStageTerminal):
			resp.Fail(w, http.StatusConflict, resp.CodeInvalidStateTransit, err.Error())
		case errors.Is(err, services.ErrStageForceRequired):
			resp.FailWarning(w, resp.CodeExitCriteriaUnmet, err.Error())
		default:
			resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "阶段流转失败")
		}
		return
	}
	resp.OK(w, map[string]string{"message": "阶段已更新"})
}
