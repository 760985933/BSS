-- +goose Up
-- M3-1 客户公海池
-- owner_id = 0 视为无主客户（公海）；以下字段支撑领取保护期与超时回收判定。
ALTER TABLE customers ADD COLUMN last_followed_at DATETIME;
ALTER TABLE customers ADD COLUMN claimed_at DATETIME;
ALTER TABLE customers ADD COLUMN pool_reason TEXT NOT NULL DEFAULT '';

CREATE INDEX idx_customers_pool ON customers(owner_id, last_followed_at);

-- 公海流水：领取/释放/回收/指派，业务可查（区别于通用 audit_logs）
CREATE TABLE customer_pool_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  customer_id INTEGER NOT NULL,
  action TEXT NOT NULL,
  from_owner_id INTEGER NOT NULL DEFAULT 0,
  to_owner_id INTEGER NOT NULL DEFAULT 0,
  operator_id INTEGER NOT NULL DEFAULT 0,
  reason TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL
);
CREATE INDEX idx_pool_logs_customer ON customer_pool_logs(customer_id, id);

-- 公海规则（单行配置）
CREATE TABLE pool_settings (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  enabled INTEGER NOT NULL DEFAULT 0,
  max_claim_per_sales INTEGER NOT NULL DEFAULT 50,
  idle_days_no_follow INTEGER NOT NULL DEFAULT 30,
  idle_days_no_deal INTEGER NOT NULL DEFAULT 60,
  protect_days INTEGER NOT NULL DEFAULT 7,
  updated_at DATETIME NOT NULL
);

INSERT INTO pool_settings
  (id, enabled, max_claim_per_sales, idle_days_no_follow, idle_days_no_deal, protect_days, updated_at)
VALUES
  (1, 0, 50, 30, 60, 7, CURRENT_TIMESTAMP);

-- 存量客户：以创建时间作为初始跟进时间，避免规则一开启就被全量回收
UPDATE customers SET last_followed_at = created_at, claimed_at = created_at WHERE owner_id <> 0;

-- +goose Down
DROP TABLE IF EXISTS pool_settings;
DROP TABLE IF EXISTS customer_pool_logs;
DROP INDEX IF EXISTS idx_customers_pool;
