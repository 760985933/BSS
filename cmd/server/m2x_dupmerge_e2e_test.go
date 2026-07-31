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
	"gorm.io/gorm"
)

// TestM2xDuplicateMerge 端到端：复用生产路由 buildRouter，
// 覆盖客户查重查询 + 合并（联系人/商单/合同迁移 + 软删从客户）。
func TestM2xDuplicateMerge(t *testing.T) {
	dir := t.TempDir()
	gdb, err := db.Open(dir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	authSvc := services.NewAuthService(gdb)
	if err := authSvc.InitAdmin(context.Background()); err != nil {
		t.Fatalf("InitAdmin: %v", err)
	}
	cfg := &config.Config{Addr: "127.0.0.1:0", DataDir: dir, JWTSecret: "test-secret-dm"}
	h := buildRouter(cfg, gdb, authSvc)
	srv := httptest.NewServer(h)
	defer srv.Close()

	adminTok := m3LoginAs(t, srv, "admin@bss.local", "admin123")

	// seed：两个客户共享同一联系人手机
	a := models.Customer{Name: "主客户D", Code: "KH-D1"}
	b := models.Customer{Name: "从客户D", Code: "KH-D2"}
	if err := gdb.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Create(&b).Error; err != nil {
		t.Fatal(err)
	}
	gdb.Create(&models.Contact{CustomerID: a.ID, Name: "ca", Phone: "13811112222"})
	gdb.Create(&models.Contact{CustomerID: b.ID, Name: "cb", Phone: "13811112222"})
	// 从客户带一个商单与一个合同，验证迁移
	emp := models.Employee{Name: "owner-d", Email: "owner-d@x.com", Role: models.RoleAdmin}
	gdb.Create(&emp)
	gdb.Create(&models.Deal{CustomerID: b.ID, Code: "SD-D1", Title: "d", Status: "won", OwnerID: emp.ID})
	gdb.Create(&models.Contract{CustomerID: b.ID, Code: "HT-D1", Title: "c", Status: "active", OwnerID: emp.ID})

	// 1) 查重：应返回 1 组
	code, body := m3DoReq(srv, "GET", "/customers/duplicates", adminTok, nil, "")
	if code != http.StatusOK {
		t.Fatalf("查重失败: code=%d body=%s", code, body)
	}
	var q struct {
		Data []struct {
			Field     string `json:"field"`
			Value     string `json:"value"`
			Customers []struct {
				ID string `json:"id"`
			} `json:"customers"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &q); err != nil {
		t.Fatalf("解析查重响应: %v", err)
	}
	if len(q.Data) != 1 {
		t.Fatalf("查重组数 = %d, want 1", len(q.Data))
	}

	// 2) 合并
	code, body = m3DoReq(srv, "POST", "/customers/merge",
			adminTok, dmMustJSON(t, map[string]any{"primary_id": a.ID, "secondary_id": b.ID}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("合并失败: code=%d body=%s", code, body)
	}

	// 3) 合并后查重应不再有该组
	code, body = m3DoReq(srv, "GET", "/customers/duplicates", adminTok, nil, "")
	if code != http.StatusOK {
		t.Fatalf("复查重失败: code=%d", code)
	}
	var q2 struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &q2); err != nil {
		t.Fatalf("解析复查重响应: %v", err)
	}
	if len(q2.Data) != 0 {
		t.Fatalf("合并后查重组数 = %d, want 0", len(q2.Data))
	}

	// 4) 从客户软删；联系人/商单/合同已迁移到主客户
	var gone models.Customer
	if err := gdb.Where("id = ?", b.ID).First(&gone).Error; !errorsIsNotFound(err) {
		t.Fatalf("从客户应已软删，got err=%v", err)
	}
	var contacts []models.Contact
	gdb.Where("customer_id = ?", a.ID).Find(&contacts)
	if len(contacts) != 2 {
		t.Errorf("主客户联系人数 = %d, want 2（含从客户迁移来的）", len(contacts))
	}
	var deals []models.Deal
	gdb.Where("customer_id = ?", a.ID).Find(&deals)
	if len(deals) != 1 {
		t.Errorf("主客户商单数 = %d, want 1", len(deals))
	}
	var contracts []models.Contract
	gdb.Where("customer_id = ?", a.ID).Find(&contracts)
	if len(contracts) != 1 {
		t.Errorf("主客户合同数 = %d, want 1", len(contracts))
	}
}

func errorsIsNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}

func dmMustJSON(t *testing.T, v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
