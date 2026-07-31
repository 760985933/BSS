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

// TestM6HRE2E 端到端冒烟：复用生产路由 buildRouter，
// 覆盖 员工创建 → 劳动合同创建/状态机(解除需原因、终态锁定) → 入职步骤推进 → 提醒扫描接入。
func TestM6HRE2E(t *testing.T) {
	dir := t.TempDir()
	gdb, err := db.Open(dir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	authSvc := services.NewAuthService(gdb)
	if err := authSvc.InitAdmin(context.Background()); err != nil {
		t.Fatalf("InitAdmin: %v", err)
	}
	cfg := &config.Config{Addr: "127.0.0.1:0", DataDir: dir, JWTSecret: "test-secret-m6-hr"}
	h := buildRouter(cfg, gdb, authSvc)
	srv := httptest.NewServer(h)
	defer srv.Close()

	adminTok := m3LoginAs(t, srv, "admin@bss.local", "admin123")

	// 1) 创建员工（用于合同/入职关联）
	code, body := m3DoReq(srv, "POST", "/employees", adminTok, m3MustJSON(t, map[string]any{
		"name": "合同员工A", "phone": "13900000000", "dept": "技术部",
		"position": "工程师", "role": "hr", "email": "hr_e2e_emp@x.com",
	}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("创建员工失败: code=%d body=%s", code, body)
	}
	empID := m6NestedID(t, body, "employee")

	// 2) 新建劳动合同（默认 draft，应生成 LC- 单号）
	code, body = m3DoReq(srv, "POST", "/labor-contracts", adminTok, m3MustJSON(t, map[string]any{
		"employee_id": empID, "type": "fixed", "probation_months": 3,
		"start_date": "2026-01-01", "end_date": "2026-12-31",
	}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("新建合同失败: code=%d body=%s", code, body)
	}
	if !strings.Contains(string(body), `"code"`) {
		t.Fatalf("合同应返回 code: %s", body)
	}
	lcID := m6DataID(t, body)

	// 3) draft -> active 直接过
	code, _ = m3DoReq(srv, "POST", "/labor-contracts/"+lcID+"/transition", adminTok,
		m3MustJSON(t, map[string]any{"to": "active", "reason": "", "force": false}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("draft->active 应 200, got %d", code)
	}

	// 4) active -> draft（回退）无 force：422 软校验
	code, _ = m3DoReq(srv, "POST", "/labor-contracts/"+lcID+"/transition", adminTok,
		m3MustJSON(t, map[string]any{"to": "draft", "reason": "", "force": false}), "application/json")
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("回退无 force 应 422, got %d", code)
	}
	// 带 force 回退成功
	code, _ = m3DoReq(srv, "POST", "/labor-contracts/"+lcID+"/transition", adminTok,
		m3MustJSON(t, map[string]any{"to": "draft", "reason": "", "force": true}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("带 force 回退应 200, got %d", code)
	}
	// 重新生效
	code, _ = m3DoReq(srv, "POST", "/labor-contracts/"+lcID+"/transition", adminTok,
		m3MustJSON(t, map[string]any{"to": "active", "reason": "", "force": false}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("draft->active(重) 应 200, got %d", code)
	}

	// 5) active -> terminated 无原因：400 必填校验
	code, _ = m3DoReq(srv, "POST", "/labor-contracts/"+lcID+"/transition", adminTok,
		m3MustJSON(t, map[string]any{"to": "terminated", "reason": "", "force": true}), "application/json")
	if code != http.StatusBadRequest {
		t.Fatalf("解除无原因应 400, got %d", code)
	}

	// 6) active -> terminated 带原因：200
	code, _ = m3DoReq(srv, "POST", "/labor-contracts/"+lcID+"/transition", adminTok,
		m3MustJSON(t, map[string]any{"to": "terminated", "reason": "协商一致", "force": true}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("带原因解除应 200, got %d", code)
	}

	// 7) 终态再流转：409
	code, _ = m3DoReq(srv, "POST", "/labor-contracts/"+lcID+"/transition", adminTok,
		m3MustJSON(t, map[string]any{"to": "active", "reason": "", "force": true}), "application/json")
	if code != http.StatusConflict {
		t.Fatalf("终态应 409, got %d", code)
	}

	// 7) 新建入职（默认 in_progress）
	code, body = m3DoReq(srv, "POST", "/onboardings", adminTok, m3MustJSON(t, map[string]any{
		"employee_id": empID, "step_profile": "pending", "step_equip": "pending",
		"step_training": "pending", "step_probation": "pending",
	}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("新建入职失败: code=%d body=%s", code, body)
	}
	obID := m6DataID(t, body)

	// 8) 四步全 done -> completed
	code, body = m3DoReq(srv, "PUT", "/onboardings/"+obID, adminTok, m3MustJSON(t, map[string]any{
		"employee_id": empID, "step_profile": "done", "step_equip": "done",
		"step_training": "done", "step_probation": "done",
	}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("更新入职失败: code=%d body=%s", code, body)
	}
	if !strings.Contains(string(body), `"status":"completed"`) {
		t.Fatalf("四步完成应 completed: %s", body)
	}

	// 9) 提醒扫描接入冒烟（扫描端点可用，不报错）
	code, _ = m3DoReq(srv, "POST", "/admin/scan-reminders", adminTok, nil, "")
	if code != http.StatusOK {
		t.Fatalf("scan-reminders 应 200, got %d", code)
	}
}

// m6DataID 从响应 data.id 取出顶层对象 ID（创建类接口 data 为对象本身）
func m6DataID(t *testing.T, body []byte) string {
	t.Helper()
	var r struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("解析响应失败: %v body=%s", err, body)
	}
	if r.Data.ID == "" {
		t.Fatalf("data.id 为空: %s", body)
	}
	return r.Data.ID
}
