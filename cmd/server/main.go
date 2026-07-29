package main

import (
	"context"
	"io/fs"
	"log"
	"net/http"
	"strings"

	bss "bss"
	"bss/internal/config"
	"bss/internal/db"
	"bss/internal/handlers"
	"bss/internal/middleware"
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
			r.Get("/employees", empH.List) // 全角色可读，数据范围在 handler 内控制
		})

		// /api 下未匹配路径返回 JSON 404（而不是 SPA 页面）
		r.NotFound(func(w http.ResponseWriter, req *http.Request) {
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, "接口不存在")
		})
	})

	// SPA 托管：静态资源直出，其余 GET 回退 index.html
	r.Get("/*", spaHandler())

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
