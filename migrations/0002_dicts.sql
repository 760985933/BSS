-- +goose Up
-- 通用数据字典（PRD §5：行业/来源/等级/回款方式等枚举后端集中定义；§4.2 部门单层枚举配置页维护）
CREATE TABLE dicts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  type TEXT NOT NULL,                      -- dept/industry/source/level/pay_method
  value TEXT NOT NULL,
  sort INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, deleted_at DATETIME
);
CREATE INDEX idx_dicts_type ON dicts(type);
-- 软删除友好的唯一约束：同名值删除后允许重建
CREATE UNIQUE INDEX uq_dicts_type_value ON dicts(type, value) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS dicts;
