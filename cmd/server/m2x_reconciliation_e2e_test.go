package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"bss/internal/config"
	"bss/internal/db"
	"bss/internal/models"
	"bss/internal/services"
)

// TestM2xReconciliation 端到端：复用生产路由 buildRouter，
// 覆盖银行流水录入 → 勾对回款 → 未达账项汇总。
func TestM2xReconciliation(t *testing.T) {
	dir := t.TempDir()
	gdb, err := db.Open(dir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	authSvc := services.NewAuthService(gdb)
	if err := authSvc.InitAdmin(context.Background()); err != nil {
		t.Fatalf("InitAdmin: %v", err)
	}
	cfg := &config.Config{Addr: "127.0.0.1:0", DataDir: dir, JWTSecret: "test-secret-rec"}
	h := buildRouter(cfg, gdb, authSvc)
	srv := httptest.NewServer(h)
	defer srv.Close()
	adminTok := m3LoginAs(t, srv, "admin@bss.local", "admin123")

	// seed：员工 + 客户 + 合同 + 回款记录
	emp := models.Employee{Name: "rec-owner", Email: "rec-owner@x.com", Role: models.RoleAdmin}
	gdb.Create(&emp)
	cust := models.Customer{Name: "rec-cust", Code: "KH-RECA"}
	gdb.Create(&cust)
	contr := models.Contract{CustomerID: cust.ID, Code: "HT-RECA", Title: "c", Status: "active", OwnerID: emp.ID}
	gdb.Create(&contr)
	pr := models.PaymentRecord{ContractID: contr.ID, AmountCent: 1000, PaidAt: "2026-07-01", Method: "bank", CreatedBy: emp.ID}
	gdb.Create(&pr)

	// 1) 录入流水
	code, body := m3DoReq(srv, "POST", "/bank-statements", adminTok,
		dmMustJSON(t, []map[string]any{{"trans_date": "2026-07-01", "counterparty": "甲方", "amount_cent": 1000, "direction": "income"}}),
		"application/json")
	if code != http.StatusOK {
		t.Fatalf("录入流水失败: code=%d body=%s", code, body)
	}

	// 2) 列表取流水 id
	code, body = m3DoReq(srv, "GET", "/bank-statements", adminTok, nil, "")
	if code != http.StatusOK {
		t.Fatalf("流水列表失败: code=%d", code)
	}
	var lst struct {
		Data []struct {
			ID uint `json:"id,string"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &lst); err != nil {
		t.Fatalf("解析流水列表: %v", err)
	}
	if len(lst.Data) != 1 {
		t.Fatalf("流水数=%d want 1", len(lst.Data))
	}
	stmtID := strconv.Itoa(int(lst.Data[0].ID))

	// 3) 勾对
	code, body = m3DoReq(srv, "POST", "/bank-statements/"+stmtID+"/reconcile", adminTok,
		dmMustJSON(t, map[string]any{"payment_record_id": pr.ID}), "application/json")
	if code != http.StatusOK {
		t.Fatalf("勾对失败: code=%d body=%s", code, body)
	}

	// 4) 未达账项：回款已勾对，两条都应空
	code, body = m3DoReq(srv, "GET", "/reconciliation", adminTok, nil, "")
	if code != http.StatusOK {
		t.Fatalf("未达账项失败: code=%d", code)
	}
	var sum struct {
		Data struct {
			BankOnly    []json.RawMessage `json:"bank_only"`
			CompanyOnly []json.RawMessage `json:"company_only"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &sum); err != nil {
		t.Fatalf("解析未达账项: %v", err)
	}
	if len(sum.Data.BankOnly) != 0 || len(sum.Data.CompanyOnly) != 0 {
		t.Errorf("勾对后未达账项应全空，got bank=%d company=%d", len(sum.Data.BankOnly), len(sum.Data.CompanyOnly))
	}
}
