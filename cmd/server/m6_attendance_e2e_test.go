package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bss/internal/config"
	"bss/internal/db"
	"bss/internal/services"
)

// TestM6AttendanceE2E 端到端冒烟：复用生产路由 buildRouter，
// 覆盖 排班创建 → 请假提交/审批 → 考勤登记/按排班生成。
func TestM6AttendanceE2E(t *testing.T) {
	dir := t.TempDir()
	gdb, err := db.Open(dir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	authSvc := services.NewAuthService(gdb)
	if err := authSvc.InitAdmin(context.Background()); err != nil {
		t.Fatalf("InitAdmin: %v", err)
	}
	cfg := &config.Config{Addr: "127.0.0.1:0", DataDir: dir, JWTSecret: "test-secret-m6-att"}
	h := buildRouter(cfg, gdb, authSvc)
	srv := httptest.NewServer(h)
	defer srv.Close()

	adminTok := m3LoginAs(t, srv, "admin@bss.local", "admin123")

	// 1) 创建员工
	code, body := m3DoReq(srv, "POST", "/employees", adminTok, m3MustJSON(t, map[string]any{
		"name": "考勤员工B", "phone": "13900000001", "dept": "技术部",
		"position": "工程师", "role": "hr", "email": "att_e2e_emp@x.com",
	}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("创建员工失败: code=%d body=%s", code, body)
	}
	empID := m6NestedID(t, body, "employee")

	// 2) 新建排班（周一）
	code, body = m3DoReq(srv, "POST", "/schedules", adminTok, m3MustJSON(t, map[string]any{
		"employee_id": empID, "weekday": 1, "start_time": "09:00", "end_time": "18:00", "shift_type": "regular",
	}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("新建排班失败: code=%d body=%s", code, body)
	}
	schID := m6DataID(t, body)

	// 3) 列表排班
	code, _ = m3DoReq(srv, "GET", "/schedules", adminTok, nil, "")
	if code != http.StatusOK {
		t.Fatalf("列表排班应 200, got %d", code)
	}

	// 4) 提交请假（2026-08-03 周一，与排班星期一致，便于生成联调）
	code, body = m3DoReq(srv, "POST", "/leave-requests", adminTok, m3MustJSON(t, map[string]any{
		"employee_id": empID, "type": "personal", "start_date": "2026-08-03", "end_date": "2026-08-03", "reason": "家事",
	}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("提交请假失败: code=%d body=%s", code, body)
	}
	if !strings.Contains(string(body), `"code"`) {
		t.Fatalf("请假应返回 code: %s", body)
	}
	leaveID := m6DataID(t, body)

	// 5) 审批通过
	code, _ = m3DoReq(srv, "POST", "/leave-requests/"+leaveID+"/decide", adminTok,
		m3MustJSON(t, map[string]any{"approve": true, "reason": ""}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("审批通过应 200, got %d", code)
	}

	// 6) 手动登记一条考勤
	code, body = m3DoReq(srv, "POST", "/attendances", adminTok, m3MustJSON(t, map[string]any{
		"employee_id": empID, "date": "2026-08-02", "status": "normal",
	}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("登记考勤失败: code=%d body=%s", code, body)
	}

	// 7) 按排班生成 2026-08-03 考勤（应识别该员工已批请假 → leave）
	code, body = m3DoReq(srv, "POST", "/attendances/generate", adminTok,
		m3MustJSON(t, map[string]any{"date": "2026-08-03"}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("生成考勤应 200, got %d body=%s", code, body)
	}
	if !strings.Contains(string(body), `"created":1`) {
		t.Fatalf("应生成 1 条考勤, got %s", body)
	}

	// 8) 列表考勤含 2 条
	code, body = m3DoReq(srv, "GET", "/attendances", adminTok, nil, "")
	if code != http.StatusOK {
		t.Fatalf("列表考勤应 200, got %d", code)
	}
	if !strings.Contains(string(body), `"status":"leave"`) {
		t.Fatalf("应含请假状态记录: %s", body)
	}

	// 9) 校验排班删除（软删 + 解除考勤引用不报错）
	code, _ = m3DoReq(srv, "DELETE", "/schedules/"+schID, adminTok, nil, "")
	if code != http.StatusOK {
		t.Fatalf("删除排班应 200, got %d", code)
	}
}
