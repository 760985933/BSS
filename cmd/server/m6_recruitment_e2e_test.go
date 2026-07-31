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

// TestM6RecruitmentE2E 端到端冒烟：复用生产路由 buildRouter，
// 覆盖 招聘职位创建 → 候选人添加 → 阶段单步/跳级(force)流转 → 终态锁定 → 漏斗 → 删除职位解除关联。
func TestM6RecruitmentE2E(t *testing.T) {
	dir := t.TempDir()
	gdb, err := db.Open(dir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	authSvc := services.NewAuthService(gdb)
	if err := authSvc.InitAdmin(context.Background()); err != nil {
		t.Fatalf("InitAdmin: %v", err)
	}
	cfg := &config.Config{Addr: "127.0.0.1:0", DataDir: dir, JWTSecret: "test-secret-m6"}
	h := buildRouter(cfg, gdb, authSvc)
	srv := httptest.NewServer(h)
	defer srv.Close()

	adminTok := m3LoginAs(t, srv, "admin@bss.local", "admin123")

	// 1) 新建职位（应自动生成 JP- 单号）
	code, body := m3DoReq(srv, "POST", "/job-posts", adminTok, m3MustJSON(t, map[string]any{
		"title": "后端工程师", "dept": "技术部", "headcount": 2, "status": "open",
	}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("新建职位失败: code=%d body=%s", code, body)
	}
	if !strings.Contains(string(body), `"code"`) {
		t.Fatalf("职位应返回 code: %s", body)
	}
	jpID := m6NestedID(t, body, "job_post")

	// 2) 添加候选人（关联职位，默认阶段 apply）
	code, body = m3DoReq(srv, "POST", "/candidates", adminTok, m3MustJSON(t, map[string]any{
		"name": "张三", "phone": "13800000000", "job_post_id": jpID, "source": "BOSS直聘",
	}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("添加候选人失败: code=%d body=%s", code, body)
	}
	candID := m6NestedID(t, body, "candidate")

	// 3) 单步前进 apply->screen
	code, _ = m3DoReq(srv, "POST", "/candidates/"+candID+"/advance", adminTok, m3MustJSON(t, map[string]any{
		"stage": "screen", "force": false,
	}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("阶段前进失败: code=%d", code)
	}

	// 4) 跳级 screen->offer 无 force 应 422（软校验）
	code, _ = m3DoReq(srv, "POST", "/candidates/"+candID+"/advance", adminTok, m3MustJSON(t, map[string]any{
		"stage": "offer", "force": false,
	}), "application/json")
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("跳级无 force 应 422, got %d", code)
	}
	// 带 force 通过
	code, _ = m3DoReq(srv, "POST", "/candidates/"+candID+"/advance", adminTok, m3MustJSON(t, map[string]any{
		"stage": "offer", "force": true,
	}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("跳级带 force 应 200, got %d", code)
	}

	// 5) offer->hired（单步，force 可省略）
	code, _ = m3DoReq(srv, "POST", "/candidates/"+candID+"/advance", adminTok, m3MustJSON(t, map[string]any{
		"stage": "hired", "force": true,
	}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("入职失败: code=%d", code)
	}
	// 终态锁定：hired->apply 应 409
	code, _ = m3DoReq(srv, "POST", "/candidates/"+candID+"/advance", adminTok, m3MustJSON(t, map[string]any{
		"stage": "apply", "force": true,
	}), "application/json")
	if code != http.StatusConflict {
		t.Fatalf("终态应 409, got %d", code)
	}

	// 6) 漏斗计数：hired=1
	code, body = m3DoReq(srv, "GET", "/candidates/funnel", adminTok, nil, "")
	if code != http.StatusOK {
		t.Fatalf("漏斗查询失败: code=%d", code)
	}
	if !strings.Contains(string(body), `"stage":"hired"`) {
		t.Fatalf("漏斗应含 hired: %s", body)
	}

	// 7) 删除职位：候选人解除关联但仍存在
	code, _ = m3DoReq(srv, "DELETE", "/job-posts/"+jpID, adminTok, nil, "")
	if code != http.StatusOK {
		t.Fatalf("删除职位失败: code=%d", code)
	}
	code, body = m3DoReq(srv, "GET", "/candidates/"+candID, adminTok, nil, "")
	if code != http.StatusOK {
		t.Fatalf("候选人应仍在: code=%d", code)
	}
	if !strings.Contains(string(body), `"name":"张三"`) {
		t.Fatalf("候选人应仍在且名为张三: %s", body)
	}
}

// m6NestedID 从响应 data.<key>.id 取出嵌套 ID（创建类接口 data 内含对象）
func m6NestedID(t *testing.T, body []byte, key string) string {
	t.Helper()
	var r struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("解析响应失败: %v body=%s", err, body)
	}
	raw, ok := r.Data[key]
	if !ok {
		t.Fatalf("data 无 %s: %s", key, body)
	}
	var obj struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("解析 %s 失败: %v", key, err)
	}
	return obj.ID
}
