-- +goose Up
-- M2-2 开票管理
CREATE TABLE invoices (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT NOT NULL,
  contract_id INTEGER NOT NULL,
  payment_record_id INTEGER,
  amount_cent INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'draft',
  issued_at TEXT DEFAULT '',
  remark TEXT DEFAULT '',
  created_by INTEGER NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  deleted_at DATETIME
);
CREATE INDEX idx_invoices_contract ON invoices(contract_id);
CREATE UNIQUE INDEX uq_invoices_code ON invoices(code);

-- +goose Down
DROP TABLE IF EXISTS invoices;
