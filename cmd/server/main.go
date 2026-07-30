package main

import (
	"context"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	bss "bss"
	"bss/internal/config"
	"bss/internal/cron"
	"bss/internal/db"
	"bss/internal/handlers"
	"bss/internal/middleware"
	"bss/internal/models"
	"bss/internal/pkg/resp"
	"bss/internal/services"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

func main() {
	cfg := config.Load()

	gdb, err := db.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}

	ctx := context.Background()
	authSvc := services.NewAuthService(gdb)
	if err := authSvc.InitAdmin(ctx); err != nil {
		log.Fatalf("admin 初始化失败: %v", err)
	}

	authH := handlers.NewAuthHandler(authSvc, cfg.JWTSecret)
	empH := handlers.NewEmployeeHandler(gdb)
	custH := handlers.NewCustomerHandler(gdb)
	dealH := handlers.NewDealHandler(gdb)
	contrH := handlers.NewContractHandler(gdb, filepath.Join(cfg.DataDir, "uploads"))
	payH := handlers.NewPaymentHandler(gdb)
	notifH := handlers.NewNotificationHandler(gdb)
	apprH := handlers.NewApprovalHandler(services.NewApprovalService(gdb))
	invH := handlers.NewInvoiceHandler(services.NewInvoiceService(gdb))

	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(chimw.RequestID)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/login", authH.Login)

		// 受保护接口：JWT + RBAC
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthRequired(cfg.JWTSecret))
			r.Get("/auth/me", authH.Me)
			r.Post("/auth/change-password", authH.ChangePassword)

			// 员工：列表全角色可读（范围在 handler 控制）；增改/状态管理仅 admin/hr；重置密码仅 admin
			r.Get("/employees", empH.List)
			r.With(middleware.RequireRole(models.RoleAdmin, models.RoleHR)).Group(func(r chi.Router) {
				r.Post("/employees", empH.Create)
				r.Put("/employees/{id}", empH.Update)
				r.Post("/employees/{id}/status", empH.SetStatus)
			})
			r.With(middleware.RequireRole(models.RoleAdmin)).Post("/employees/{id}/reset-password", empH.ResetPassword)

			// 数据字典：全员可读，admin 维护
			r.Get("/dicts", empH.ListDicts)
			r.With(middleware.RequireRole(models.RoleAdmin)).Group(func(r chi.Router) {
				r.Post("/dicts", empH.AddDict)
				r.Delete("/dicts/{id}", empH.RemoveDict)
			})

			// 客户：查看全角色（ScopeOwner 行级过滤）；增删改/转移排除财务（PRD §6）
			r.Get("/customers", custH.List)
			r.Get("/customers/{id}", custH.Get)
			r.Get("/customers/{id}/contacts", custH.ListContacts)
			r.With(middleware.RequireRole(models.RoleAdmin, models.RoleSales, models.RoleSalesLead)).Group(func(r chi.Router) {
				r.Post("/customers", custH.Create)
				r.Put("/customers/{id}", custH.Update)
				r.Delete("/customers/{id}", custH.Delete)
				r.Post("/customers/{id}/transfer", custH.Transfer)
				r.Post("/customers/{id}/contacts", custH.CreateContact)
				r.Put("/customers/{id}/contacts/{cid}", custH.UpdateContact)
				r.Delete("/customers/{id}/contacts/{cid}", custH.DeleteContact)
			})

			// 商单：查看全角色（ScopeOwner）；增删改/状态流转排除财务（PRD §6）
			r.Get("/deals", dealH.List)
			r.Get("/deals/forecast", dealH.Forecast)
			r.Get("/deals/{id}", dealH.Get)
			r.With(middleware.RequireRole(models.RoleAdmin, models.RoleSales, models.RoleSalesLead)).Group(func(r chi.Router) {
				r.Post("/deals", dealH.Create)
				r.Put("/deals/{id}", dealH.Update)
			r.Post("/deals/{id}/status", dealH.ChangeStatus)
			r.Delete("/deals/{id}", dealH.Delete)
		})

		// 合同：查看全角色（ScopeOwner）；增删改/状态流转/关联商单/附件排除财务（PRD §6）
		r.Get("/contracts", contrH.List)
		r.Get("/contracts/{id}", contrH.Get)
		r.Get("/contracts/{id}/attachments", contrH.ListAttachments)
		r.With(middleware.RequireRole(models.RoleAdmin, models.RoleSales, models.RoleSalesLead)).Group(func(r chi.Router) {
			r.Post("/contracts", contrH.Create)
			r.Put("/contracts/{id}", contrH.Update)
			r.Post("/contracts/{id}/status", contrH.ChangeStatus)
			r.Put("/contracts/{id}/deals", contrH.ReplaceDeals)
			r.Delete("/contracts/{id}", contrH.Delete)
			r.Post("/contracts/{id}/attachments", contrH.UploadAttachment)
			r.Delete("/attachments/{id}", contrH.DeleteAttachment)
		})
		// 附件下载：鉴权组内，非登录不可下载
		r.Get("/attachments/{id}/download", contrH.DownloadAttachment)

		// 提醒 + 仪表盘：登录即可访问自己的通知；仪表盘按 ScopeOwner 过滤
		r.Get("/notifications", notifH.List)
		r.Get("/notifications/unread-count", notifH.UnreadCount)
		r.Post("/notifications/{id}/read", notifH.MarkRead)
		r.Post("/notifications/read-all", notifH.MarkAllRead)
		r.Get("/dashboard", notifH.Dashboard)
		r.With(middleware.RequireRole(models.RoleAdmin)).Post("/admin/scan-reminders", notifH.TriggerScan)

		// 审批流（M2-1）：登录即可查看；提交审批限销售/销售主管/管理员；审批通过/驳回限管理员/财务/销售主管
		r.Get("/approvals", apprH.List)
		r.Get("/approvals/{id}", apprH.Get)
		r.With(middleware.RequireRole(models.RoleAdmin, models.RoleSales, models.RoleSalesLead)).Post("/approvals", apprH.Create)
		r.With(middleware.RequireRole(models.RoleAdmin, models.RoleFinance, models.RoleSalesLead)).Group(func(r chi.Router) {
			r.Post("/approvals/{id}/approve", apprH.Approve)
			r.Post("/approvals/{id}/reject", apprH.Reject)
		})

		// 开票管理（M2-2）：查看全角色（ScopeOwner）；新建/开票/作废/编辑/删除限管理员/财务/销售主管
		r.Get("/invoices", invH.List)
		r.Get("/invoices/{id}", invH.Get)
		r.With(middleware.RequireRole(models.RoleAdmin, models.RoleFinance, models.RoleSalesLead)).Group(func(r chi.Router) {
			r.Post("/invoices", invH.Create)
			r.Post("/invoices/{id}/issue", invH.Issue)
			r.Post("/invoices/{id}/void", invH.Void)
			r.Put("/invoices/{id}", invH.Update)
			r.Delete("/invoices/{id}", invH.Delete)
		})

		// 回款：查看全角色（ScopeOwner）；计划 CRUD 排除财务；回款记录录入/删除仅 admin/finance
		r.Get("/contracts/{id}/plans", payH.ListPlans)
		r.Get("/contracts/{id}/records", payH.ListRecords)
		r.Get("/contracts/{id}/payment-summary", payH.Summary)
		r.With(middleware.RequireRole(models.RoleAdmin, models.RoleSales, models.RoleSalesLead)).Group(func(r chi.Router) {
			r.Post("/contracts/{id}/plans", payH.CreatePlan)
			r.Put("/contracts/{id}/plans/{pid}", payH.UpdatePlan)
			r.Delete("/contracts/{id}/plans/{pid}", payH.DeletePlan)
		})
		r.With(middleware.RequireRole(models.RoleAdmin, models.RoleFinance)).Group(func(r chi.Router) {
			r.Post("/contracts/{id}/records", payH.CreateRecords)
			r.Delete("/contracts/{id}/records/{rid}", payH.DeleteRecord)
		})
	})

		// /api 下未匹配路径返回 JSON 404（而不是 SPA 页面）
		r.NotFound(func(w http.ResponseWriter, req *http.Request) {
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, "接口不存在")
		})
	})

	// SPA 托管：静态资源直出，其余 GET 回退 index.html
	r.Get("/*", spaHandler())

	// 后台调度：每日 09:00 扫描到期/逾期生成提醒（随主进程生命周期）
	cron.Start(context.Background(), gdb)

	log.Printf("BSS 服务已启动: http://%s （数据目录: %s）", cfg.Addr, cfg.DataDir)
	if err := http.ListenAndServe(cfg.Addr, r); err != nil {
		log.Fatal(err)
	}
}

// spaHandler 内嵌前端单页应用托管
func spaHandler() http.HandlerFunc {
	dist, err := fs.Sub(bss.WebDistFS, "web/dist")
	if err != nil {
		log.Fatalf("内嵌前端资源失败: %v", err)
	}
	fileServer := http.FileServer(http.FS(dist))
	return func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p != "" {
			if f, err := dist.Open(p); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		// 前端路由回退
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	}
}
