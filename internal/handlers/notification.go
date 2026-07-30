package handlers

import (
	"net/http"
	"strconv"
	"time"

	"bss/internal/middleware"
	"bss/internal/pkg/resp"
	"bss/internal/services"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// NotificationHandler 站内通知 + 仪表盘。
type NotificationHandler struct {
	db *gorm.DB
}

func NewNotificationHandler(db *gorm.DB) *NotificationHandler {
	return &NotificationHandler{db: db}
}

// List GET /notifications?is_read=&type=&page=&size=
func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	c := middleware.UserFrom(r.Context())
	if c == nil {
		resp.Fail(w, http.StatusUnauthorized, resp.CodeUnauthorized, "未登录")
		return
	}
	var isRead *bool
	if v := r.URL.Query().Get("is_read"); v != "" {
		b := v == "1" || v == "true"
		isRead = &b
	}
	typ := r.URL.Query().Get("type")
	page, size := pageParams(r)
	list, total, err := services.ListNotifications(r.Context(), h.db, c.UserID, isRead, typ, page, size)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, map[string]interface{}{
		"items": list,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// UnreadCount GET /notifications/unread-count
func (h *NotificationHandler) UnreadCount(w http.ResponseWriter, r *http.Request) {
	c := middleware.UserFrom(r.Context())
	if c == nil {
		resp.Fail(w, http.StatusUnauthorized, resp.CodeUnauthorized, "未登录")
		return
	}
	n, err := services.UnreadCount(r.Context(), h.db, c.UserID)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, map[string]interface{}{"count": n})
}

// MarkRead POST /notifications/:id/read
func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	c := middleware.UserFrom(r.Context())
	if c == nil {
		resp.Fail(w, http.StatusUnauthorized, resp.CodeUnauthorized, "未登录")
		return
	}
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "通知 ID 非法")
		return
	}
	if err := services.MarkRead(r.Context(), h.db, c.UserID, uint(id)); err != nil {
		switch err {
		case services.ErrNotificationNotFound:
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, "通知不存在")
		case services.ErrNotificationForbidden:
			resp.Fail(w, http.StatusForbidden, resp.CodeForbidden, "无权操作该通知")
		default:
			resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		}
		return
	}
	resp.OK(w, map[string]string{"message": "ok"})
}

// MarkAllRead POST /notifications/read-all
func (h *NotificationHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	c := middleware.UserFrom(r.Context())
	if c == nil {
		resp.Fail(w, http.StatusUnauthorized, resp.CodeUnauthorized, "未登录")
		return
	}
	if err := services.MarkAllRead(r.Context(), h.db, c.UserID); err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, map[string]string{"message": "ok"})
}

// Dashboard GET /dashboard
func (h *NotificationHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	d, err := services.GetDashboard(r.Context(), h.db, time.Now())
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, d)
}

// TriggerScan POST /admin/scan-reminders —— 手动触发提醒扫描（admin 维护用）
func (h *NotificationHandler) TriggerScan(w http.ResponseWriter, r *http.Request) {
	n, err := services.ScanReminders(r.Context(), h.db, time.Now())
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, map[string]interface{}{"created": n})
}
