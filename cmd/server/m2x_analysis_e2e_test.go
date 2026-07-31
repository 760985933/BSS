package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"bss/internal/config"
	"bss/internal/db"
	"bss/internal/models"
	"bss/internal/services"
)

// TestM2xLostAnalysis 端到端：复用生产路由 buildRouter，验证输单分析端点
// （仅 admin/主管可访问）返回正确的聚合数据。
func TestM2xLostAnalysis(t *testing.T) {
	dir := t.TempDir()
	gdb, err := db.Open(dir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	authSvc := services.NewAuthService(gdb)
	if err := authSvc.InitAdmin(context.Background()); err != nil {
		t.Fatalf("InitAdmin: %v", err)
	}
	cfg := &config.Config{Addr: "127.0.0.1:0", DataDir: dir, JWTSecret: "test-secret-la"}
	h := buildRouter(cfg, gdb, authSvc)
	srv := httptest.NewServer(h)
	defer srv.Close()
	adminTok := m3LoginAs(t, srv, "admin@bss.local", "admin123")

	// seed：员工 + 客户 + 2 个 lost 商单（不同原因）
	emp := models.Employee{Name: "la-owner", Email: "la-owner@x.com", Role: models.RoleAdmin}
	if err := gdb.Create(&emp).Error; err != nil {
		t.Fatal(err)
	}
	cust := models.Customer{Name: "la-cust", Code: "KH-LAX"}
	if err := gdb.Create(&cust).Error; err != nil {
		t.Fatal(err)
	}
	gdb.Create(&models.Deal{CustomerID: cust.ID, Code: "SD-LA1", Title: "a", Status: "lost", LostReason: "competitor", OwnerID: emp.ID})
	gdb.Create(&models.Deal{CustomerID: cust.ID, Code: "SD-LA2", Title: "b", Status: "lost", LostReason: "budget", OwnerID: emp.ID})

	code, body := m3DoReq(srv, "GET", "/reports/lost-analysis", adminTok, nil, "")
	if code != http.StatusOK {
		t.Fatalf("分析端点失败: code=%d body=%s", code, body)
	}
	var r struct {
		Data struct {
			Total int `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("解析响应: %v", err)
	}
	if r.Data.Total != 2 {
		t.Errorf("total=%d, want 2", r.Data.Total)
	}
}
