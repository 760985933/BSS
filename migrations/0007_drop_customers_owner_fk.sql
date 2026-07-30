-- +goose Up
-- +goose NO TRANSACTION
-- M3-1 修复：客户公海池语义要求 owner_id = 0 表示无主客户（公海），
-- 但 0001 中 customers.owner_id 带有 REFERENCES employees(id) 外键，置 0 会触发
-- FOREIGN KEY constraint failed。通过新建临时表（去掉 owner 外键）复制数据后换名，
-- 避免重建子表导致的外键级联。子表外键按表名 "customers" 自然解析到新表，无需重建。
-- 说明：DROP 原 customers 需关闭外键检查，故本迁移以 NO TRANSACTION 运行。
PRAGMA foreign_keys=OFF;

CREATE TABLE customers2 (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT NOT NULL UNIQUE,                 -- KH-2026-0001
  name TEXT NOT NULL UNIQUE,
  industry TEXT DEFAULT '', source TEXT DEFAULT '', level TEXT DEFAULT '',
  owner_id INTEGER NOT NULL,
  remark TEXT DEFAULT '',
  last_followed_at DATETIME,
  claimed_at DATETIME,
  pool_reason TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, deleted_at DATETIME
);

INSERT INTO customers2 (id, code, name, industry, source, level, owner_id, remark, last_followed_at, claimed_at, pool_reason, created_at, updated_at, deleted_at)
  SELECT id, code, name, industry, source, level, owner_id, remark, last_followed_at, claimed_at, pool_reason, created_at, updated_at, deleted_at
  FROM customers;

DROP TABLE customers;
ALTER TABLE customers2 RENAME TO customers;
UPDATE sqlite_sequence SET name='customers' WHERE name='customers2';

CREATE INDEX idx_customers_owner ON customers(owner_id);
CREATE INDEX idx_customers_pool ON customers(owner_id, last_followed_at);

PRAGMA foreign_keys=ON;

-- +goose Down
-- +goose NO TRANSACTION
PRAGMA foreign_keys=OFF;
CREATE TABLE customers2 (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT NOT NULL UNIQUE,                 -- KH-2026-0001
  name TEXT NOT NULL UNIQUE,
  industry TEXT DEFAULT '', source TEXT DEFAULT '', level TEXT DEFAULT '',
  owner_id INTEGER NOT NULL REFERENCES employees(id),
  remark TEXT DEFAULT '',
  last_followed_at DATETIME,
  claimed_at DATETIME,
  pool_reason TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, deleted_at DATETIME
);
INSERT INTO customers2 (id, code, name, industry, source, level, owner_id, remark, last_followed_at, claimed_at, pool_reason, created_at, updated_at, deleted_at)
  SELECT id, code, name, industry, source, level, owner_id, remark, last_followed_at, claimed_at, pool_reason, created_at, updated_at, deleted_at
  FROM customers;
DROP TABLE customers;
ALTER TABLE customers2 RENAME TO customers;
UPDATE sqlite_sequence SET name='customers' WHERE name='customers2';
CREATE INDEX idx_customers_owner ON customers(owner_id);
CREATE INDEX idx_customers_pool ON customers(owner_id, last_followed_at);
PRAGMA foreign_keys=ON;
