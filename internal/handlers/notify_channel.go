package handlers

import (
	"encoding/json"
	"net/http"

	"bss/internal/middleware"
	"bss/internal/pkg/resp"
	"bss/internal/services"

	"gorm.io/gorm"
)

// NotifyChannelHandler 通知渠道配置与外发日志（M3-4，仅 admin）。
type NotifyChannelHandler struct {
	svc *services.NotifyService
}

func NewNotifyChannelHandler(db *gorm.DB) *NotifyChannelHandler {
	return &NotifyChannelHandler{svc: services.NewNotifyService(db)}
}

// GetSettings GET /notify-settings —— 密码字段以掩码返回
func (h *NotifyChannelHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.Settings(r.Context())
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, services.Masked(st))
}

// UpdateSettings PUT /notify-settings
func (h *NotifyChannelHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var in services.NotifySettingsInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求体格式错误")
		return
	}
	st, err := h.svc.UpdateSettings(r.Context(), in)
	if err != nil {
		switch err {
		case services.ErrSMTPIncomplete, services.ErrWecomWebhookBad:
			resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
		default:
			resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		}
		return
	}
	resp.OK(w, services.Masked(st))
}

type testChannelReq struct {
	Channel string `json:"channel"` // email | wecom
	To      string `json:"to"`      // 邮件渠道可选，缺省发给当前登录人
}

// Test POST /notify-settings/test
func (h *NotifyChannelHandler) Test(w http.ResponseWriter, r *http.Request) {
	c := middleware.UserFrom(r.Context())
	if c == nil {
		resp.Fail(w, http.StatusUnauthorized, resp.CodeUnauthorized, "未登录")
		return
	}
	var in testChannelReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求体格式错误")
		return
	}
	if err := h.svc.SendTest(r.Context(), in.Channel, in.To, c.UserID); err != nil {
		switch err {
		case services.ErrChannelUnknown, services.ErrChannelDisabled,
			services.ErrNoRecipientEmail, services.ErrWecomWebhookBad, services.ErrSMTPIncomplete:
			resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
		default:
			// 网络/认证类失败：返回原始错误，方便管理员排查
			resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "发送失败："+err.Error())
		}
		return
	}
	resp.OK(w, map[string]string{"message": "已发送，请查收"})
}

// Logs GET /notify-logs?channel=&status=&page=&size=
func (h *NotifyChannelHandler) Logs(w http.ResponseWriter, r *http.Request) {
	page, size := pageParams(r)
	list, total, err := h.svc.Logs(r.Context(),
		r.URL.Query().Get("channel"), r.URL.Query().Get("status"), page, size)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, err.Error())
		return
	}
	resp.OK(w, resp.PageData{List: list, Total: total, Page: page, Size: size})
}
