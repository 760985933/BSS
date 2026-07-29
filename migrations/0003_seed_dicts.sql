-- +goose Up
-- 预置业务字典（PRD §5 数据字典）：行业 / 客户来源 / 客户等级 / 回款方式
INSERT INTO dicts (type, value, sort, created_at, updated_at) VALUES
  ('industry', 'IT服务', 1, datetime('now'), datetime('now')),
  ('industry', '制造业', 2, datetime('now'), datetime('now')),
  ('industry', '贸易零售', 3, datetime('now'), datetime('now')),
  ('industry', '金融', 4, datetime('now'), datetime('now')),
  ('industry', '教育培训', 5, datetime('now'), datetime('now')),
  ('industry', '医疗健康', 6, datetime('now'), datetime('now')),
  ('industry', '其他', 99, datetime('now'), datetime('now')),
  ('source', '转介绍', 1, datetime('now'), datetime('now')),
  ('source', '官网咨询', 2, datetime('now'), datetime('now')),
  ('source', '电话开发', 3, datetime('now'), datetime('now')),
  ('source', '展会活动', 4, datetime('now'), datetime('now')),
  ('source', '其他', 99, datetime('now'), datetime('now')),
  ('level', 'A 重点', 1, datetime('now'), datetime('now')),
  ('level', 'B 普通', 2, datetime('now'), datetime('now')),
  ('level', 'C 观察', 3, datetime('now'), datetime('now')),
  ('pay_method', '银行转账', 1, datetime('now'), datetime('now')),
  ('pay_method', '承兑汇票', 2, datetime('now'), datetime('now')),
  ('pay_method', '现金', 3, datetime('now'), datetime('now')),
  ('pay_method', '其他', 99, datetime('now'), datetime('now'));

-- +goose Down
DELETE FROM dicts WHERE type IN ('industry', 'source', 'level', 'pay_method');
