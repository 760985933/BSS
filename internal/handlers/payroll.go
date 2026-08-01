package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"bss/internal/middleware"
	"bss/internal/pkg/code"
	"bss/internal/pkg/resp"
	"bss/internal/services"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// PayrollHandler 承载 M6-S4 薪酬核算 的 HTTP 处理（限 admin/finance/hr）
type PayrollHandler struct {
	db *gorm.DB
	gen *code.Generator
}

func NewPayrollHandler(db *gorm.DB) *PayrollHandler {
	return &PayrollHandler{db: db, gen: code.NewGenerator(db)}
}

func (h *PayrollHandler) ownerID(r *http.Request) uint {
	return middleware.UserFrom(r.Context()).UserID
}

func (h *PayrollHandler) ListPayrolls(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	list, err := services.ListPayrolls(r.Context(), h.db, q.Get("period"), q.Get("employee_id"), q.Get("status"))
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, list)
}

func (h *PayrollHandler) CreatePayroll(w http.ResponseWriter, r *http.Request) {
	var in services.PayrollInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求体解析失败")
		return
	}
	p, err := services.CreatePayroll(r.Context(), h.db, h.gen, in, h.ownerID(r))
	if err != nil {
		if errors.Is(err, services.ErrPayrollEmployeeMissing) || errors.Is(err, services.ErrPayrollPeriodInvalid) {
			resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
			return
		}
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, p)
}

func (h *PayrollHandler) GeneratePayrolls(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Period string `json:"period"`
	}
	// 允许空请求体（无 period → 默认当前月份）；仅当 JSON 结构非法时返回 400
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求体解析失败")
		return
	}
	if req.Period == "" {
		// 缺省为当前月份
		req.Period = services.CurrentPeriod()
	}
	n, err := services.GeneratePayrolls(r.Context(), h.db, h.gen, req.Period, h.ownerID(r))
	if err != nil {
		if errors.Is(err, services.ErrPayrollPeriodInvalid) {
			resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
			return
		}
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, map[string]any{"period": req.Period, "created": n})
}

func (h *PayrollHandler) GetPayroll(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	p, err := services.GetPayroll(r.Context(), h.db, uint(id))
	if err != nil {
		if errors.Is(err, services.ErrPayrollMissing) {
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
			return
		}
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, p)
}

func (h *PayrollHandler) UpdatePayroll(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	var in services.PayrollInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求体解析失败")
		return
	}
	p, err := services.UpdatePayroll(r.Context(), h.db, uint(id), in)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrPayrollMissing):
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
		case errors.Is(err, services.ErrPayrollNotDraft):
			resp.Fail(w, http.StatusConflict, resp.CodeFieldLocked, err.Error())
		case errors.Is(err, services.ErrPayrollPeriodInvalid), errors.Is(err, services.ErrPayrollEmployeeMissing):
			resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
		default:
			resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		}
		return
	}
	resp.OK(w, p)
}

func (h *PayrollHandler) CalcPayroll(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	p, err := services.CalcPayroll(r.Context(), h.db, uint(id))
	if err != nil {
		switch {
		case errors.Is(err, services.ErrPayrollMissing):
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
		case errors.Is(err, services.ErrPayrollCalcState):
			resp.Fail(w, http.StatusConflict, resp.CodeInvalidStateTransit, err.Error())
		case errors.Is(err, services.ErrPayrollNetNegative):
			resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
		default:
			resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		}
		return
	}
	resp.OK(w, p)
}

func (h *PayrollHandler) PayPayroll(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	p, err := services.MarkPayrollPaid(r.Context(), h.db, uint(id))
	if err != nil {
		switch {
		case errors.Is(err, services.ErrPayrollMissing):
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
		case errors.Is(err, services.ErrPayrollPayState):
			resp.Fail(w, http.StatusConflict, resp.CodeInvalidStateTransit, err.Error())
		default:
			resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		}
		return
	}
	resp.OK(w, p)
}

func (h *PayrollHandler) DeletePayroll(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err := services.DeletePayroll(r.Context(), h.db, uint(id)); err != nil {
		switch {
		case errors.Is(err, services.ErrPayrollMissing):
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
		case errors.Is(err, services.ErrPayrollPaid):
			resp.Fail(w, http.StatusConflict, resp.CodeFieldLocked, err.Error())
		default:
			resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		}
		return
	}
	resp.OK(w, nil)
}

func (h *PayrollHandler) ExportPayrolls(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = services.CurrentPeriod()
	}
	csvStr, err := services.ExportPayrollsCSV(r.Context(), h.db, period)
	if err != nil {
		if errors.Is(err, services.ErrPayrollPeriodInvalid) {
			resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
			return
		}
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, map[string]any{"period": period, "csv": csvStr})
}
