-- +goose Up
-- BSS 一期全量 DDL（对应 docs/TECH_DESIGN.md v1.2 §4）
-- 约定：金额统一 *_cent INTEGER（分）；时间由 GORM 存 UTC ISO8601 TEXT；软删除 deleted_at

CREATE TABLE employees (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  email TEXT NOT NULL UNIQUE,
  phone TEXT DEFAULT '',
  dept TEXT DEFAULT '',
  position TEXT DEFAULT '',
  role TEXT NOT NULL DEFAULT 'sales',        -- admin/sales/sales_lead/finance/hr
  password_hash TEXT NOT NULL,
  must_change_pwd INTEGER NOT NULL DEFAULT 0, -- 首启 admin 为 1，改密后清 0
  status TEXT NOT NULL DEFAULT 'active',     -- active/disabled
  created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, deleted_at DATETIME
);

CREATE TABLE customers (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT NOT NULL UNIQUE,                 -- KH-2026-0001
  name TEXT NOT NULL UNIQUE,
  industry TEXT DEFAULT '', source TEXT DEFAULT '', level TEXT DEFAULT '',
  owner_id INTEGER NOT NULL,
  remark TEXT DEFAULT '',
  created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, deleted_at DATETIME
);
CREATE INDEX idx_customers_owner ON customers(owner_id);

CREATE TABLE contacts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  customer_id INTEGER NOT NULL REFERENCES customers(id),
  name TEXT NOT NULL, phone TEXT DEFAULT '', email TEXT DEFAULT '',
  position TEXT DEFAULT '', is_primary INTEGER NOT NULL DEFAULT 0,
  remark TEXT DEFAULT '',
  created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, deleted_at DATETIME
);
CREATE INDEX idx_contacts_customer ON contacts(customer_id);

CREATE TABLE deals (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT NOT NULL UNIQUE,                 -- SD-2026-0001
  customer_id INTEGER NOT NULL REFERENCES customers(id),
  title TEXT NOT NULL,
  amount_cent INTEGER NOT NULL DEFAULT 0,
  probability INTEGER NOT NULL DEFAULT 10,   -- 赢单概率%，按阶段带出可手调
  expected_sign_date TEXT,
  status TEXT NOT NULL DEFAULT 'prospecting',-- prospecting/qualifying/proposal/negotiating/won/lost
  lost_reason TEXT DEFAULT '',               -- no_purchase/competitor/budget/qualified_out/other
  owner_id INTEGER NOT NULL REFERENCES employees(id),
  remark TEXT DEFAULT '',
  created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, deleted_at DATETIME
);
CREATE INDEX idx_deals_customer ON deals(customer_id);
CREATE INDEX idx_deals_owner ON deals(owner_id);
CREATE INDEX idx_deals_status ON deals(status);

CREATE TABLE contracts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT NOT NULL UNIQUE,                 -- HT-2026-0001
  customer_id INTEGER NOT NULL REFERENCES customers(id),
  title TEXT NOT NULL,
  amount_cent INTEGER NOT NULL DEFAULT 0,    -- 与商单金额独立口径，不强制勾稽
  sign_date TEXT, start_date TEXT, expire_date TEXT,
  status TEXT NOT NULL DEFAULT 'draft',      -- draft/pending/signed/performing/completed/cancelled/terminated/expired（approving 预留）
  terminate_reason TEXT DEFAULT '',
  owner_id INTEGER NOT NULL REFERENCES employees(id),
  remark TEXT DEFAULT '',
  created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, deleted_at DATETIME
);
CREATE INDEX idx_contracts_customer ON contracts(customer_id);
CREATE INDEX idx_contracts_expire ON contracts(expire_date);

-- 商单 ↔ 合同 N:N 关联（评审确认：最开放方案）
CREATE TABLE deal_contracts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  deal_id INTEGER NOT NULL REFERENCES deals(id),
  contract_id INTEGER NOT NULL REFERENCES contracts(id),
  created_at DATETIME NOT NULL,
  UNIQUE(deal_id, contract_id)
);
CREATE INDEX idx_dc_deal ON deal_contracts(deal_id);
CREATE INDEX idx_dc_contract ON deal_contracts(contract_id);

CREATE TABLE payment_plans (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  contract_id INTEGER NOT NULL REFERENCES contracts(id),
  period_no INTEGER NOT NULL,
  due_date TEXT NOT NULL,
  amount_cent INTEGER NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',    -- pending/partial/paid（逾期为派生）
  created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, deleted_at DATETIME
);
CREATE INDEX idx_plans_contract ON payment_plans(contract_id);
CREATE INDEX idx_plans_due ON payment_plans(due_date);

CREATE TABLE payment_records (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  contract_id INTEGER NOT NULL REFERENCES contracts(id),
  plan_id INTEGER REFERENCES payment_plans(id), -- 可空=不核销计划
  amount_cent INTEGER NOT NULL,
  paid_at TEXT NOT NULL,
  method TEXT DEFAULT '', remark TEXT DEFAULT '',
  created_by INTEGER NOT NULL REFERENCES employees(id),
  created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, deleted_at DATETIME
);
CREATE INDEX idx_records_contract ON payment_records(contract_id);

CREATE TABLE notifications (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL REFERENCES employees(id),
  type TEXT NOT NULL,                        -- contract_expiring/payment_overdue
  title TEXT NOT NULL, content TEXT DEFAULT '',
  entity_type TEXT NOT NULL, entity_id INTEGER NOT NULL,
  dedup_key TEXT NOT NULL UNIQUE,            -- type+entity+date 防重复
  is_read INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL
);
CREATE INDEX idx_notif_user ON notifications(user_id, is_read);

CREATE TABLE attachments (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  entity_type TEXT NOT NULL,                 -- contract（一期仅合同）
  entity_id INTEGER NOT NULL,
  file_name TEXT NOT NULL, file_path TEXT NOT NULL,
  file_size INTEGER NOT NULL, mime TEXT DEFAULT '',
  uploaded_by INTEGER NOT NULL REFERENCES employees(id),
  created_at DATETIME NOT NULL, deleted_at DATETIME
);

-- 审计日志（只追加，无 updated/deleted）
CREATE TABLE audit_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  entity_type TEXT NOT NULL, entity_id INTEGER NOT NULL,
  action TEXT NOT NULL,                      -- create/update/delete/transfer/status_change
  operator_id INTEGER NOT NULL DEFAULT 0,    -- 0=系统
  before_json TEXT DEFAULT '', after_json TEXT DEFAULT '',
  created_at DATETIME NOT NULL
);
CREATE INDEX idx_audit_entity ON audit_logs(entity_type, entity_id);

-- 业务单号计数器（年度递增）
CREATE TABLE code_counters (
  prefix TEXT NOT NULL,                      -- KH/SD/HT
  year INTEGER NOT NULL,
  seq INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (prefix, year)
);

-- +goose Down
DROP TABLE IF EXISTS code_counters;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS attachments;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS payment_records;
DROP TABLE IF EXISTS payment_plans;
DROP TABLE IF EXISTS deal_contracts;
DROP TABLE IF EXISTS contracts;
DROP TABLE IF EXISTS deals;
DROP TABLE IF EXISTS contacts;
DROP TABLE IF EXISTS customers;
DROP TABLE IF EXISTS employees;
