package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"bss/internal/middleware"
	"bss/internal/pkg/code"
	"bss/internal/pkg/resp"
	"bss/internal/services"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// HRHandler 承载 M6-S2 劳动合同 + 入职管理 的 HTTP 处理
type HRHandler struct {
	db *gorm.DB
	gen *code.Generator
}

func NewHRHandler(db *gorm.DB) *HRHandler {
	return &HRHandler{db: db, gen: code.NewGenerator(db)}
}

func (h *HRHandler) ownerID(r *http.Request) uint {
	return middleware.UserFrom(r.Context()).UserID
}

// ---------------- 劳动合同 ----------------

func (h *HRHandler) ListLaborContracts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	list, err := services.ListLaborContracts(r.Context(), h.db, q.Get("keyword"), q.Get("status"), q.Get("employee_id"))
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, list)
}

func (h *HRHandler) CreateLaborContract(w http.ResponseWriter, r *http.Request) {
	var in services.LaborContractInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求体解析失败")
		return
	}
	lc, err := services.CreateLaborContract(r.Context(), h.db, h.gen, in, h.ownerID(r))
	if err != nil {
		if errors.Is(err, services.ErrEmployeeMissing) {
			resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
			return
		}
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, lc)
}

func (h *HRHandler) GetLaborContract(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	lc, err := services.GetLaborContract(r.Context(), h.db, uint(id))
	if err != nil {
		if errors.Is(err, services.ErrLaborContractMissing) {
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
			return
		}
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, lc)
}

func (h *HRHandler) UpdateLaborContract(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	var in services.LaborContractInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求体解析失败")
		return
	}
	lc, err := services.UpdateLaborContract(r.Context(), h.db, uint(id), in)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrLaborContractMissing):
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
		case errors.Is(err, services.ErrEmployeeMissing):
			resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
		case errors.Is(err, services.ErrLCStatusTerminal):
			resp.Fail(w, http.StatusConflict, resp.CodeInvalidStateTransit, err.Error())
		default:
			resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		}
		return
	}
	resp.OK(w, lc)
}

func (h *HRHandler) DeleteLaborContract(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err := services.DeleteLaborContract(r.Context(), h.db, uint(id)); err != nil {
		if errors.Is(err, services.ErrLaborContractMissing) {
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
			return
		}
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, nil)
}

type lcTransitionReq struct {
	To     string `json:"to"`
	Reason string `json:"reason"`
	Force  bool   `json:"force"`
}

// TransitionLaborContract 合同状态流转（解除需原因，回退需 force）
func (h *HRHandler) TransitionLaborContract(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	var req lcTransitionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求体解析失败")
		return
	}
	lc, err := services.TransitionLaborContract(r.Context(), h.db, uint(id), req.To, req.Reason, req.Force)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrLaborContractMissing):
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
		case errors.Is(err, services.ErrLCStatusTerminal), errors.Is(err, services.ErrLCInvalidTransition):
			resp.Fail(w, http.StatusConflict, resp.CodeInvalidStateTransit, err.Error())
		case errors.Is(err, services.ErrLCTransitionForceReq):
			resp.FailWarning(w, resp.CodeExitCriteriaUnmet, err.Error())
		case errors.Is(err, services.ErrLCTerminateReasonRequired):
			resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
		default:
			resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		}
		return
	}
	resp.OK(w, lc)
}

// ---------------- 入职管理 ----------------

func (h *HRHandler) ListOnboardings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	list, err := services.ListOnboardings(r.Context(), h.db, q.Get("keyword"), q.Get("status"), q.Get("employee_id"))
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, list)
}

func (h *HRHandler) CreateOnboarding(w http.ResponseWriter, r *http.Request) {
	var in services.OnboardingInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求体解析失败")
		return
	}
	ob, err := services.CreateOnboarding(r.Context(), h.db, h.gen, in, h.ownerID(r))
	if err != nil {
		switch {
		case errors.Is(err, services.ErrEmployeeMissing), errors.Is(err, services.ErrCandidateMissing):
			resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
		default:
			resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		}
		return
	}
	resp.OK(w, ob)
}

func (h *HRHandler) GetOnboarding(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	ob, err := services.GetOnboarding(r.Context(), h.db, uint(id))
	if err != nil {
		if errors.Is(err, services.ErrOnboardingMissing) {
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
			return
		}
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, ob)
}

func (h *HRHandler) UpdateOnboarding(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	var in services.OnboardingInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求体解析失败")
		return
	}
	ob, err := services.UpdateOnboarding(r.Context(), h.db, uint(id), in)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrOnboardingMissing):
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
		case errors.Is(err, services.ErrEmployeeMissing), errors.Is(err, services.ErrCandidateMissing):
			resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
		default:
			resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		}
		return
	}
	resp.OK(w, ob)
}

func (h *HRHandler) DeleteOnboarding(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err := services.DeleteOnboarding(r.Context(), h.db, uint(id)); err != nil {
		if errors.Is(err, services.ErrOnboardingMissing) {
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
			return
		}
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, nil)
}
