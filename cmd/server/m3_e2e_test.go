package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bss/internal/config"
	"bss/internal/db"
	"bss/internal/services"

	"github.com/xuri/excelize/v2"
)

// TestM3ImportAndProjects 端到端冒烟：复用生产路由 buildRouter，
// 覆盖 Excel 导入（模板下载 + 上传落库）与项目/交付管理（增删成员/任务）。
func TestM3ImportAndProjects(t *testing.T) {
	dir := t.TempDir()
	gdb, err := db.Open(dir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	authSvc := services.NewAuthService(gdb)
	if err := authSvc.InitAdmin(context.Background()); err != nil {
		t.Fatalf("InitAdmin: %v", err)
	}
	cfg := &config.Config{Addr: "127.0.0.1:0", DataDir: dir, JWTSecret: "test-secret-m3"}
	h := buildRouter(cfg, gdb, authSvc)
	srv := httptest.NewServer(h)
	defer srv.Close()

	adminTok := m3LoginAs(t, srv, "admin@bss.local", "admin123")

	// 1) 模板下载
	code, body := m3DoReq(srv, "GET", "/imports/customers/template", adminTok, nil, "")
	if code != http.StatusOK || len(body) == 0 {
		t.Fatalf("模板下载失败: code=%d len=%d", code, len(body))
	}

	// 2) 上传导入：构造 xlsx 并 multipart 提交
	xlsx := m3MakeImportXLSX(t)
	code, body = m3PostMultipart(srv, "/imports/customers", adminTok, "file", "customers.xlsx", xlsx)
	if code != http.StatusOK {
		t.Fatalf("导入上传失败: code=%d body=%s", code, body)
	}
	var imp struct {
		Data struct {
			Total            int `json:"total"`
			CreatedCustomers int `json:"created_customers"`
		} `json:"data"`
	}
	_ = json.Unmarshal(body, &imp)
	if imp.Data.CreatedCustomers < 1 {
		t.Fatalf("导入未创建客户: %s", body)
	}

	// 3) 新建项目
	code, body = m3DoReq(srv, "POST", "/projects", adminTok, m3MustJSON(t, map[string]any{
		"name": "烟雾测试项目", "owner_id": "1", "status": "in_progress",
	}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("新建项目失败: code=%d body=%s", code, body)
	}
	projID := m3JSONField(t, body, "id")

	// 4) 添加成员（人天）
	code, _ = m3DoReq(srv, "POST", "/projects/"+projID+"/members", adminTok, m3MustJSON(t, map[string]any{
		"employee_id": "1", "role": "负责", "planned_days": 10, "actual_days": 4,
	}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("添加成员失败: code=%d", code)
	}

	// 5) 添加任务
	code, _ = m3DoReq(srv, "POST", "/projects/"+projID+"/tasks", adminTok, m3MustJSON(t, map[string]any{
		"kind": "task", "title": "需求调研", "status": "doing", "estimate_days": 3,
	}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("添加任务失败: code=%d", code)
	}

	// 6) 详情应包含成员与任务
	code, body = m3DoReq(srv, "GET", "/projects/"+projID, adminTok, nil, "")
	if code != http.StatusOK {
		t.Fatalf("获取项目详情失败: code=%d", code)
	}
	if !strings.Contains(string(body), `"members"`) || !strings.Contains(string(body), `"tasks"`) {
		t.Fatalf("项目详情未含成员/任务: %s", body)
	}

	// 7) 删除项目
	code, _ = m3DoReq(srv, "DELETE", "/projects/"+projID, adminTok, nil, "")
	if code != http.StatusOK {
		t.Fatalf("删除项目失败: code=%d", code)
	}
}

// ---------- helpers ----------

func m3LoginAs(t *testing.T, srv *httptest.Server, email, pwd string) string {
	t.Helper()
	code, body := m3DoReq(srv, "POST", "/auth/login", "", m3MustJSON(t, map[string]string{
		"email": email, "password": pwd,
	}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("登录失败: code=%d body=%s", code, body)
	}
	var r struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(body, &r)
	return r.Data.Token
}

func m3DoReq(srv *httptest.Server, method, apiPath, token string, body []byte, ct string) (int, []byte) {
	req, _ := http.NewRequest(method, srv.URL+"/api/v1"+apiPath, bytes.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func m3PostMultipart(srv *httptest.Server, apiPath, token, field, filename string, content []byte) (int, []byte) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile(field, filename)
	part.Write(content)
	mw.Close()
	return m3DoReq(srv, "POST", apiPath, token, buf.Bytes(), mw.FormDataContentType())
}

func m3MakeImportXLSX(t *testing.T) []byte {
	t.Helper()
	f := excelize.NewFile()
	const sheet = "客户导入"
	f.SetSheetName("Sheet1", sheet)
	headers := []string{"客户名称", "行业", "来源", "等级", "负责人邮箱", "备注",
		"联系人姓名", "联系人手机", "联系人邮箱", "联系人职位", "是否首要联系人"}
	for i, h := range headers {
		c, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, c, h)
	}
	row := []string{"导入测试客户M3", "科技", "转介绍", "A", "", "冒烟", "", "", "", "", ""}
	for i, v := range row {
		c, _ := excelize.CoordinatesToCellName(i+1, 2)
		_ = f.SetCellValue(sheet, c, v)
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func m3MustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func m3JSONField(t *testing.T, body []byte, key string) string {
	t.Helper()
	var r map[string]json.RawMessage
	_ = json.Unmarshal(body, &r)
	raw, ok := r["data"]
	if !ok {
		t.Fatalf("响应无 data: %s", body)
	}
	var data map[string]json.RawMessage
	_ = json.Unmarshal(raw, &data)
	field, ok := data[key]
	if !ok {
		t.Fatalf("data 无字段 %s: %s", key, body)
	}
	s := string(field)
	return strings.Trim(s, `"`)
}
