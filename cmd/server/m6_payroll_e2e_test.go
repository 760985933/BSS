package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bss/internal/config"
	"bss/internal/db"
	"bss/internal/services"
)

// TestM6PayrollE2E 端到端冒烟：复用生产路由 buildRouter，
// 覆盖 建员工 → 建生效合同(含月薪) → 生成当月薪资 → 核算 → 发放 → 导出。
func TestM6PayrollE2E(t *testing.T) {
	dir := t.TempDir()
	gdb, err := db.Open(dir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	authSvc := services.NewAuthService(gdb)
	if err := authSvc.InitAdmin(context.Background()); err != nil {
		t.Fatalf("InitAdmin: %v", err)
	}
	cfg := &config.Config{Addr: "127.0.0.1:0", DataDir: dir, JWTSecret: "test-secret-m6-pay"}
	h := buildRouter(cfg, gdb, authSvc)
	srv := httptest.NewServer(h)
	defer srv.Close()

	adminTok := m3LoginAs(t, srv, "admin@bss.local", "admin123")

	// 1) 创建在职员工
	code, body := m3DoReq(srv, "POST", "/employees", adminTok, m3MustJSON(t, map[string]any{
		"name": "薪资员工C", "phone": "13900000002", "dept": "技术部",
		"position": "工程师", "role": "hr", "email": "pay_e2e_emp@x.com",
	}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("创建员工失败: code=%d body=%s", code, body)
	}
	empID := m6NestedID(t, body, "employee")

	// 2) 新建劳动合同并写入月薪（2000 元 = 200000 分），转 active
	code, body = m3DoReq(srv, "POST", "/labor-contracts", adminTok, m3MustJSON(t, map[string]any{
		"employee_id": empID, "type": "fixed", "salary_cent": 200000,
		"start_date": "2026-01-01",
	}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("新建合同失败: code=%d body=%s", code, body)
	}
	lcID := m6DataID(t, body)
	code, _ = m3DoReq(srv, "POST", "/labor-contracts/"+lcID+"/transition", adminTok,
		m3MustJSON(t, map[string]any{"to": "active"}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("合同转 active 应 200, got %d", code)
	}

	// 3) 生成当月薪资
	period := "2026-08"
	code, body = m3DoReq(srv, "POST", "/payrolls/generate", adminTok,
		m3MustJSON(t, map[string]any{"period": period}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("生成薪资应 200, got %d body=%s", code, body)
	}
	if !strings.Contains(string(body), `"created":`) {
		t.Fatalf("生成薪资应返回 created, got %s", body)
	}

	// 4) 列表并定位该员工薪资，校验底薪取自合同
	code, body = m3DoReq(srv, "GET", "/payrolls?period="+period, adminTok, nil, "")
	if code != http.StatusOK {
		t.Fatalf("列表薪资应 200, got %d", code)
	}
	var listResp struct {
		Code int `json:"code"`
		Data []struct {
			ID       string `json:"id"`
			BaseCent int64  `json:"base_cent"`
			Status   string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &listResp); err != nil {
		t.Fatalf("解析列表失败: %v", err)
	}
	if len(listResp.Data) == 0 {
		t.Fatalf("薪资列表应为非空")
	}
	payID := ""
	for _, p := range listResp.Data {
		if p.BaseCent == 200000 {
			payID = p.ID
		}
	}
	if payID == "" {
		t.Fatalf("应存在底薪=200000 的薪资记录, got %s", body)
	}

	// 5) 核算 → 实发=200000，状态 calced
	code, body = m3DoReq(srv, "POST", "/payrolls/"+payID+"/calc", adminTok, nil, "")
	if code != http.StatusOK {
		t.Fatalf("核算应 200, got %d body=%s", code, body)
	}
	if !strings.Contains(string(body), `"net_cent":200000`) || !strings.Contains(string(body), `"status":"calced"`) {
		t.Fatalf("核算后 net_cent=200000/calced, got %s", body)
	}

	// 6) 发放 → paid
	code, body = m3DoReq(srv, "POST", "/payrolls/"+payID+"/pay", adminTok, nil, "")
	if code != http.StatusOK {
		t.Fatalf("发放应 200, got %d body=%s", code, body)
	}
	if !strings.Contains(string(body), `"status":"paid"`) {
		t.Fatalf("发放后 status=paid, got %s", body)
	}

	// 7) 导出工资条（应含员工名与金额）
	code, body = m3DoReq(srv, "GET", "/payrolls/export?period="+period, adminTok, nil, "")
	if code != http.StatusOK {
		t.Fatalf("导出应 200, got %d", code)
	}
	if !strings.Contains(string(body), "薪资员工C") {
		t.Fatalf("导出 CSV 应含员工名, got %s", body)
	}
}
