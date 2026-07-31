-- +goose Up
-- M4-3 银企对账：银行流水表
CREATE TABLE bank_statements (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  trans_date TEXT NOT NULL,
  counterparty TEXT DEFAULT '',
  amount_cent INTEGER NOT NULL DEFAULT 0,
  direction TEXT NOT NULL DEFAULT 'income', -- income 收 / expend 付
  summary TEXT DEFAULT '',
  payment_record_id INTEGER,
  created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, deleted_at DATETIME,
  FOREIGN KEY (payment_record_id) REFERENCES payment_records(id)
);
CREATE INDEX idx_bs_payment ON bank_statements(payment_record_id);
CREATE INDEX idx_bs_date ON bank_statements(trans_date);

-- +goose Down
DROP TABLE IF EXISTS bank_statements;
