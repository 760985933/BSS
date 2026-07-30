-- +goose Up
-- M2-1 审批流：approvals 表 + deals 增加折扣金额列
CREATE TABLE approvals (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT NOT NULL,
  entity_type TEXT NOT NULL,            -- contract | deal
  entity_id INTEGER NOT NULL,
  kind TEXT NOT NULL,                  -- contract_sign | deal_discount
  status TEXT NOT NULL DEFAULT 'pending',
  applicant_id INTEGER NOT NULL,
  approver_id INTEGER,
  amount_cent INTEGER NOT NULL DEFAULT 0,
  note TEXT DEFAULT '',
  reject_reason TEXT DEFAULT '',
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  deleted_at DATETIME
);
CREATE INDEX idx_approvals_entity ON approvals(entity_type, entity_id);
CREATE UNIQUE INDEX uq_approvals_code ON approvals(code);

-- 商单折扣审批通过后落定的折扣金额（分），0 表示无折扣
ALTER TABLE deals ADD COLUMN discount_amount_cent INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE deals DROP COLUMN discount_amount_cent;
DROP TABLE IF EXISTS approvals;
