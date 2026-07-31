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

// TestM34NotifyChannels 端到端冒烟：复用生产路由 buildRouter，
// 覆盖通知渠道配置的读写（密码掩码）、企业微信测试发送（打到本地 httptest webhook）与外发日志查询。
func TestM34NotifyChannels(t *testing.T) {
	dir := t.TempDir()
	gdb, err := db.Open(dir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	authSvc := services.NewAuthService(gdb)
	if err := authSvc.InitAdmin(context.Background()); err != nil {
		t.Fatalf("InitAdmin: %v", err)
	}
	cfg := &config.Config{Addr: "127.0.0.1:0", DataDir: dir, JWTSecret: "test-secret-m34"}
	srv := httptest.NewServer(buildRouter(cfg, gdb, authSvc))
	defer srv.Close()

	// 假 webhook：断言收到 markdown 消息
	hits := 0
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if payload["msgtype"] != "markdown" {
			t.Errorf("webhook 收到非 markdown 消息: %v", payload)
		}
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer hook.Close()

	tok := m3LoginAs(t, srv, "admin@bss.local", "admin123")

	// 1) 初始配置：两个渠道均关闭
	code, body := m3DoReq(srv, "GET", "/notify-settings", tok, nil, "")
	if code != http.StatusOK {
		t.Fatalf("读配置失败: code=%d body=%s", code, body)
	}
	if !strings.Contains(string(body), `"email_enabled":false`) || !strings.Contains(string(body), `"wecom_enabled":false`) {
		t.Fatalf("默认应两个渠道均关闭: %s", body)
	}

	// 2) 非法 webhook 应被拒
	code, _ = m3DoReq(srv, "PUT", "/notify-settings", tok, m3MustJSON(t, map[string]any{
		"wecom_enabled": true, "wecom_webhook": "不是地址",
	}), "application/json")
	if code != http.StatusBadRequest {
		t.Fatalf("非法 webhook 应 400，得到 %d", code)
	}

	// 3) 保存有效配置（含 SMTP 密码）
	code, body = m3DoReq(srv, "PUT", "/notify-settings", tok, m3MustJSON(t, map[string]any{
		"wecom_enabled": true, "wecom_webhook": hook.URL + "/cgi-bin/webhook/send?key=test",
		"email_enabled": true, "smtp_host": "smtp.example.com", "smtp_port": 465,
		"smtp_username": "u", "smtp_password": "p@ss", "smtp_from": "bss@example.com", "smtp_tls": true,
		"types": "contract_expiring,payment_overdue",
	}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("保存配置失败: code=%d body=%s", code, body)
	}
	if !strings.Contains(string(body), `"smtp_password":"********"`) {
		t.Fatalf("密码应以掩码返回: %s", body)
	}

	// 4) 企业微信测试发送 → 打到本地 webhook
	code, body = m3DoReq(srv, "POST", "/notify-settings/test", tok,
		m3MustJSON(t, map[string]any{"channel": "wecom"}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("测试发送失败: code=%d body=%s", code, body)
	}
	if hits != 1 {
		t.Fatalf("webhook 应被调用 1 次，实际 %d 次", hits)
	}

	// 5) 外发日志有一条成功记录
	code, body = m3DoReq(srv, "GET", "/notify-logs?channel=wecom", tok, nil, "")
	if code != http.StatusOK {
		t.Fatalf("查日志失败: code=%d body=%s", code, body)
	}
	var logs struct {
		Data struct {
			Total int `json:"total"`
			List  []struct {
				Channel string `json:"channel"`
				Status  string `json:"status"`
			} `json:"list"`
		} `json:"data"`
	}
	_ = json.Unmarshal(body, &logs)
	if logs.Data.Total != 1 || logs.Data.List[0].Status != "success" {
		t.Fatalf("应有 1 条成功的 wecom 日志: %s", body)
	}

	// 6) 未知渠道 → 400
	code, _ = m3DoReq(srv, "POST", "/notify-settings/test", tok,
		m3MustJSON(t, map[string]any{"channel": "sms"}), "application/json")
	if code != http.StatusBadRequest {
		t.Fatalf("未知渠道应 400，得到 %d", code)
	}
}
