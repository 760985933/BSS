package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bss/internal/config"
	"bss/internal/db"
	"bss/internal/middleware"
	"bss/internal/models"
	"bss/internal/services"
)

// 权限矩阵回归测试
//
// 目的：防止路由层 RBAC（RequireRole）被误改导致权限逃逸或误伤。
// 做法：复用与生产完全一致的 buildRouter，为 5 个角色签发 JWT，对每一个
// RequireRole 保护的端点逐角色发请求，断言：
//   - 角色在白名单内 → 不得返回 403（可能因资源不存在返回 404/422，但权限层必须通过）
//   - 角色不在白名单 → 必须返回 403
//   - 匿名（无 token）→ 必须返回 401
//
// 注意：本表是路由权限的 single source of truth，若调整 cmd/server/main.go 的
// RequireRole 角色或新增受保护端点，必须同步更新下方 cases，否则此测试会失败报警。

var allRoles = []string{
	models.RoleAdmin,
	models.RoleSales,
	models.RoleSalesLead,
	models.RoleFinance,
	models.RoleHR,
}

// permCase 描述一个受 RequireRole 保护的端点及其允许角色
type permCase struct {
	method  string
	path    string
	allowed []string
}

func p(method, path string, allowed ...string) permCase {
	return permCase{method: method, path: path, allowed: allowed}
}

// setupMatrix 启动与生产一致的路由，并为各角色签发 JWT（不落库，纯内存 token）
func setupMatrix(t *testing.T) (http.Handler, map[string]string) {
	t.Helper()
	dir := t.TempDir()
	gdb, err := db.Open(dir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	// InitAdmin 模拟生产初始化（不影响 token，仅确保基础数据就绪）
	authSvc := services.NewAuthService(gdb)
	if err := authSvc.InitAdmin(context.Background()); err != nil {
		t.Fatalf("InitAdmin: %v", err)
	}

	secret := "test-secret-permission-matrix"
	cfg := &config.Config{Addr: "127.0.0.1:0", DataDir: dir, JWTSecret: secret}

	// 为各角色构造内存 Employee 并签发 token（uid 任意，RequireRole 只看 Role）
	tokens := map[string]string{}
	seed := []struct {
		role, dept string
	}{
		{models.RoleAdmin, "MGT"},
		{models.RoleSales, "BD"},
		{models.RoleSalesLead, "BD"},
		{models.RoleFinance, "FIN"},
		{models.RoleHR, "HR"},
	}
	for _, s := range seed {
		emp := &models.Employee{Name: s.role, Role: s.role, Dept: s.dept}
		tok, err := middleware.GenerateToken(secret, emp)
		if err != nil {
			t.Fatalf("签发 %s token 失败: %v", s.role, err)
		}
		tokens[s.role] = tok
	}

	router := buildRouter(cfg, gdb, authSvc)
	return router, tokens
}

// doReq 发送一次请求并返回状态码
func doReq(r http.Handler, token, method, path string) int {
	var body io.Reader
	if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
		body = strings.NewReader("{}")
	}
	req := httptest.NewRequest(method, "/api/v1"+path, body)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec.Code
}

// TestPermissionMatrix 全网权限矩阵回归
func TestPermissionMatrix(t *testing.T) {
	router, tokens := setupMatrix(t)

	cases := []permCase{
		// 员工：增改/状态/离职预览/离职仅 admin/hr；重置密码仅 admin
		p("POST", "/employees", models.RoleAdmin, models.RoleHR),
		p("PUT", "/employees/1", models.RoleAdmin, models.RoleHR),
		p("POST", "/employees/1/status", models.RoleAdmin, models.RoleHR),
		p("GET", "/employees/1/offboard-preview", models.RoleAdmin, models.RoleHR),
		p("POST", "/employees/1/offboard", models.RoleAdmin, models.RoleHR),
		p("POST", "/employees/1/reset-password", models.RoleAdmin),

		// 数据字典：仅 admin 维护
		p("POST", "/dicts", models.RoleAdmin),
		p("DELETE", "/dicts/1", models.RoleAdmin),

		// 客户写操作：排除财务、HR（admin/sales/sales_lead）
		p("POST", "/customers", models.RoleAdmin, models.RoleSales, models.RoleSalesLead),
		p("PUT", "/customers/1", models.RoleAdmin, models.RoleSales, models.RoleSalesLead),
		p("DELETE", "/customers/1", models.RoleAdmin, models.RoleSales, models.RoleSalesLead),
		p("POST", "/customers/1/transfer", models.RoleAdmin, models.RoleSales, models.RoleSalesLead),
		p("POST", "/customers/1/contacts", models.RoleAdmin, models.RoleSales, models.RoleSalesLead),
		p("PUT", "/customers/1/contacts/1", models.RoleAdmin, models.RoleSales, models.RoleSalesLead),
		p("DELETE", "/customers/1/contacts/1", models.RoleAdmin, models.RoleSales, models.RoleSalesLead),

		// 客户公海池：领取/释放 admin/sales/sales_lead；指派/回收 admin/sales_lead；规则仅 admin
		p("POST", "/customers/1/claim", models.RoleAdmin, models.RoleSales, models.RoleSalesLead),
		p("POST", "/customers/1/release", models.RoleAdmin, models.RoleSales, models.RoleSalesLead),
		p("POST", "/customer-pool/1/assign", models.RoleAdmin, models.RoleSalesLead),
		p("POST", "/customer-pool/recycle", models.RoleAdmin, models.RoleSalesLead),
		p("PUT", "/customer-pool/settings", models.RoleAdmin),

		// 商单写操作：排除财务、HR
		p("POST", "/deals", models.RoleAdmin, models.RoleSales, models.RoleSalesLead),
		p("PUT", "/deals/1", models.RoleAdmin, models.RoleSales, models.RoleSalesLead),
		p("POST", "/deals/1/status", models.RoleAdmin, models.RoleSales, models.RoleSalesLead),
		p("DELETE", "/deals/1", models.RoleAdmin, models.RoleSales, models.RoleSalesLead),

		// 合同写操作：排除财务、HR
		p("POST", "/contracts", models.RoleAdmin, models.RoleSales, models.RoleSalesLead),
		p("PUT", "/contracts/1", models.RoleAdmin, models.RoleSales, models.RoleSalesLead),
		p("POST", "/contracts/1/status", models.RoleAdmin, models.RoleSales, models.RoleSalesLead),
		p("PUT", "/contracts/1/deals", models.RoleAdmin, models.RoleSales, models.RoleSalesLead),
		p("DELETE", "/contracts/1", models.RoleAdmin, models.RoleSales, models.RoleSalesLead),
		p("POST", "/contracts/1/attachments", models.RoleAdmin, models.RoleSales, models.RoleSalesLead),
		p("DELETE", "/attachments/1", models.RoleAdmin, models.RoleSales, models.RoleSalesLead),

		// 通知后台扫描：仅 admin
		p("POST", "/admin/scan-reminders", models.RoleAdmin),

		// 审批流：提交 admin/sales/sales_lead；通过/驳回 admin/finance/sales_lead
		p("POST", "/approvals", models.RoleAdmin, models.RoleSales, models.RoleSalesLead),
		p("POST", "/approvals/1/approve", models.RoleAdmin, models.RoleFinance, models.RoleSalesLead),
		p("POST", "/approvals/1/reject", models.RoleAdmin, models.RoleFinance, models.RoleSalesLead),

		// 开票管理：admin/finance/sales_lead
		p("POST", "/invoices", models.RoleAdmin, models.RoleFinance, models.RoleSalesLead),
		p("POST", "/invoices/1/issue", models.RoleAdmin, models.RoleFinance, models.RoleSalesLead),
		p("POST", "/invoices/1/void", models.RoleAdmin, models.RoleFinance, models.RoleSalesLead),
		p("PUT", "/invoices/1", models.RoleAdmin, models.RoleFinance, models.RoleSalesLead),
		p("DELETE", "/invoices/1", models.RoleAdmin, models.RoleFinance, models.RoleSalesLead),

		// 审计查询：仅 admin/hr/finance（GET 也受 RequireRole 保护）
		p("GET", "/audit-logs", models.RoleAdmin, models.RoleHR, models.RoleFinance),

		// 回款：计划 CRUD 排除财务、HR；回款记录录入/删除仅 admin/finance
		p("POST", "/contracts/1/plans", models.RoleAdmin, models.RoleSales, models.RoleSalesLead),
		p("PUT", "/contracts/1/plans/1", models.RoleAdmin, models.RoleSales, models.RoleSalesLead),
		p("DELETE", "/contracts/1/plans/1", models.RoleAdmin, models.RoleSales, models.RoleSalesLead),
		p("POST", "/contracts/1/records", models.RoleAdmin, models.RoleFinance),
		p("DELETE", "/contracts/1/records/1", models.RoleAdmin, models.RoleFinance),
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			for _, role := range allRoles {
				code := doReq(router, tokens[role], tc.method, tc.path)
				allowed := false
				for _, a := range tc.allowed {
					if a == role {
						allowed = true
						break
					}
				}
				if allowed {
					if code == http.StatusForbidden {
						t.Errorf("角色 %s 应在白名单内(被允许),但 %s %s 返回 403", role, tc.method, tc.path)
					}
				} else {
					if code != http.StatusForbidden {
						t.Errorf("角色 %s 不在白名单(应被拒绝),但 %s %s 返回 %d(期望 403)", role, tc.method, tc.path, code)
					}
				}
			}
		})
	}
}

// TestPermissionMatrixAnonymous 匿名请求必须 401
func TestPermissionMatrixAnonymous(t *testing.T) {
	router, _ := setupMatrix(t)
	cases := []permCase{
		p("POST", "/employees"),
		p("GET", "/customers"),
		p("POST", "/deals"),
		p("POST", "/contracts"),
		p("GET", "/audit-logs"),
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			code := doReq(router, "", tc.method, tc.path)
			if code != http.StatusUnauthorized {
				t.Errorf("匿名请求 %s %s 应返回 401,却返回 %d", tc.method, tc.path, code)
			}
		})
	}
}
