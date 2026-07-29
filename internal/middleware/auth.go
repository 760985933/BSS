// Package middleware JWT 鉴权与 RBAC（TECH_DESIGN §6.5）
package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"bss/internal/actor"
	"bss/internal/models"
	"bss/internal/pkg/resp"

	"github.com/golang-jwt/jwt/v5"
)

type ctxKey string

const ctxClaims ctxKey = "claims"

// Claims 登录态载体
type Claims struct {
	UserID uint   `json:"uid"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	Dept   string `json:"dept"`
	jwt.RegisteredClaims
}

// GenerateToken 签发 12h 有效期的 JWT
func GenerateToken(secret string, emp *models.Employee) (string, error) {
	claims := Claims{
		UserID: emp.ID,
		Name:   emp.Name,
		Role:   emp.Role,
		Dept:   emp.Dept,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(12 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// AuthRequired 解析 Bearer Token，注入 Claims 与审计 actor
func AuthRequired(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if tokenStr == "" || tokenStr == r.Header.Get("Authorization") {
				resp.Fail(w, http.StatusUnauthorized, resp.CodeUnauthorized, "未登录或凭证缺失")
				return
			}
			claims := &Claims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
				return []byte(secret), nil
			})
			if err != nil || !token.Valid {
				resp.Fail(w, http.StatusUnauthorized, resp.CodeUnauthorized, "登录已过期，请重新登录")
				return
			}
			ctx := WithClaims(r.Context(), claims)
			ctx = actor.WithActor(ctx, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole 功能权限：角色不在白名单直接 403
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := UserFrom(r.Context())
			if c == nil || !allowed[c.Role] {
				resp.Fail(w, http.StatusForbidden, resp.CodeForbidden, "当前角色无权执行此操作")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// WithClaims 注入登录态（AuthRequired 内使用；测试代码也可直接注入，绕过 HTTP 层）
func WithClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, ctxClaims, c)
}

// UserFrom 从 context 取当前登录用户
func UserFrom(ctx context.Context) *Claims {
	if c, ok := ctx.Value(ctxClaims).(*Claims); ok {
		return c
	}
	return nil
}
