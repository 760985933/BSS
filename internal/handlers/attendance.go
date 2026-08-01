package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"bss/internal/middleware"
	"bss/internal/pkg/code"
	"bss/internal/pkg/resp"
	"bss/internal/services"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// AttendanceHandler 承载 M6-S3 考勤排班 + 请假 + 考勤记录 的 HTTP 处理
type AttendanceHandler struct {
	db *gorm.DB
	gen *code.Generator
}

func NewAttendanceHandler(db *gorm.DB) *AttendanceHandler {
	return &AttendanceHandler{db: db, gen: code.NewGenerator(db)}
}

func (h *AttendanceHandler) ownerID(r *http.Request) uint {
	return middleware.UserFrom(r.Context()).UserID
}

// ---------------- 排班 ----------------

func (h *AttendanceHandler) ListSchedules(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	list, err := services.ListSchedules(r.Context(), h.db, q.Get("employee_id"))
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, list)
}

func (h *AttendanceHandler) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	var in services.ScheduleInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求体解析失败")
		return
	}
	s, err := services.CreateSchedule(r.Context(), h.db, in)
	if err != nil {
		if errors.Is(err, services.ErrEmployeeMissing) {
			resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
			return
		}
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, s)
}

func (h *AttendanceHandler) GetSchedule(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	s, err := services.GetSchedule(r.Context(), h.db, uint(id))
	if err != nil {
		if errors.Is(err, services.ErrScheduleMissing) {
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
			return
		}
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, s)
}

func (h *AttendanceHandler) UpdateSchedule(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	var in services.ScheduleInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求体解析失败")
		return
	}
	s, err := services.UpdateSchedule(r.Context(), h.db, uint(id), in)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrScheduleMissing):
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
		case errors.Is(err, services.ErrEmployeeMissing), errors.Is(err, services.ErrScheduleWeekdayInvalid):
			resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
		default:
			resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		}
		return
	}
	resp.OK(w, s)
}

func (h *AttendanceHandler) DeleteSchedule(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err := services.DeleteSchedule(r.Context(), h.db, uint(id)); err != nil {
		if errors.Is(err, services.ErrScheduleMissing) {
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
			return
		}
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, nil)
}

// ---------------- 请假 ----------------

func (h *AttendanceHandler) ListLeaveRequests(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	list, err := services.ListLeaveRequests(r.Context(), h.db, q.Get("employee_id"), q.Get("status"), q.Get("type"))
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, list)
}

func (h *AttendanceHandler) CreateLeaveRequest(w http.ResponseWriter, r *http.Request) {
	var in services.LeaveRequestInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求体解析失败")
		return
	}
	lr, err := services.CreateLeaveRequest(r.Context(), h.db, h.gen, in)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrEmployeeMissing), errors.Is(err, services.ErrLeaveTypeInvalid), errors.Is(err, services.ErrLeaveDateInvalid):
			resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
		default:
			resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		}
		return
	}
	resp.OK(w, lr)
}

func (h *AttendanceHandler) GetLeaveRequest(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	lr, err := services.GetLeaveRequest(r.Context(), h.db, uint(id))
	if err != nil {
		if errors.Is(err, services.ErrLeaveMissing) {
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
			return
		}
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, lr)
}

func (h *AttendanceHandler) DeleteLeaveRequest(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err := services.DeleteLeaveRequest(r.Context(), h.db, uint(id)); err != nil {
		if errors.Is(err, services.ErrLeaveMissing) {
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
			return
		}
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, nil)
}

type leaveDecideReq struct {
	Approve bool   `json:"approve"`
	Reason  string `json:"reason"`
}

// DecideLeaveRequest 审批：approve=true 通过 / false 驳回
func (h *AttendanceHandler) DecideLeaveRequest(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	var req leaveDecideReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求体解析失败")
		return
	}
	if !req.Approve && strings.TrimSpace(req.Reason) == "" {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "驳回必须填写原因")
		return
	}
	lr, err := services.DecideLeaveRequest(r.Context(), h.db, uint(id), req.Approve, h.ownerID(r), req.Reason)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrLeaveMissing):
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
		case errors.Is(err, services.ErrLeaveAlreadyDecided):
			resp.Fail(w, http.StatusConflict, resp.CodeInvalidStateTransit, err.Error())
		default:
			resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		}
		return
	}
	resp.OK(w, lr)
}

// ---------------- 考勤记录 ----------------

func (h *AttendanceHandler) ListAttendances(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	list, err := services.ListAttendances(r.Context(), h.db, q.Get("employee_id"), q.Get("date"), q.Get("status"), q.Get("from"), q.Get("to"))
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, list)
}

func (h *AttendanceHandler) UpsertAttendance(w http.ResponseWriter, r *http.Request) {
	var in services.AttendanceInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求体解析失败")
		return
	}
	a, err := services.UpsertAttendance(r.Context(), h.db, in)
	if err != nil {
		if errors.Is(err, services.ErrEmployeeMissing) {
			resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
			return
		}
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, a)
}

func (h *AttendanceHandler) GenerateAttendance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Date string `json:"date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求体解析失败")
		return
	}
	n, err := services.GenerateAttendance(r.Context(), h.db, req.Date)
	if err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
		return
	}
	resp.OK(w, map[string]any{"created": n})
}

func (h *AttendanceHandler) DeleteAttendance(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err := services.DeleteAttendance(r.Context(), h.db, uint(id)); err != nil {
		if errors.Is(err, services.ErrAttendanceMissing) {
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
			return
		}
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, nil)
}
