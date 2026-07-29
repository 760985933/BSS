package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"bss/internal/middleware"
	"bss/internal/pkg/resp"
	"bss/internal/services"
)

// AuthHandler 登录/会话相关接口
type AuthHandler struct {
	auth  *services.AuthService
	token string
}

func NewAuthHandler(auth *services.AuthService, jwtSecret string) *AuthHandler {
	return &AuthHandler{auth: auth, token: jwtSecret}
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login POST /api/v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Password == "" {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请输入邮箱和密码")
		return
	}
	emp, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		resp.Fail(w, http.StatusUnauthorized, resp.CodeUnauthorized, err.Error())
		return
	}
	token, err := middleware.GenerateToken(h.token, emp)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "签发会话失败")
		return
	}
	resp.OK(w, map[string]any{"token": token, "user": emp})
}

// Me GET /api/v1/auth/me —— 返回当前登录用户信息（前端路由守卫用）
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	c := middleware.UserFrom(r.Context())
	if c == nil {
		resp.Fail(w, http.StatusUnauthorized, resp.CodeUnauthorized, "未登录")
		return
	}
	resp.OK(w, map[string]any{
		"id":   strconv.FormatUint(uint64(c.UserID), 10),
		"name": c.Name,
		"role": c.Role,
		"dept": c.Dept,
	})
}

type changePwdReq struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ChangePassword POST /api/v1/auth/change-password
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	c := middleware.UserFrom(r.Context())
	var req changePwdReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OldPassword == "" || req.NewPassword == "" {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请输入原密码和新密码")
		return
	}
	if err := h.auth.ChangePassword(r.Context(), c.UserID, req.OldPassword, req.NewPassword); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
		return
	}
	resp.OK(w, map[string]string{"message": "密码已更新，请重新登录"})
}
