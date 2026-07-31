-- +goose Up
-- M3-4 通知渠道扩展：站内信之外增加「邮件（SMTP）」与「企业微信群机器人 webhook」
-- 单行配置表（id 恒为 1），默认两个渠道均关闭，不配置就完全不影响现有行为。
CREATE TABLE notify_settings (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  -- 邮件（SMTP）
  email_enabled INTEGER NOT NULL DEFAULT 0,
  smtp_host TEXT NOT NULL DEFAULT '',
  smtp_port INTEGER NOT NULL DEFAULT 465,
  smtp_username TEXT NOT NULL DEFAULT '',
  smtp_password TEXT NOT NULL DEFAULT '',
  smtp_from TEXT NOT NULL DEFAULT '',
  smtp_tls INTEGER NOT NULL DEFAULT 1,       -- 1=隐式 TLS(465)，0=明文/STARTTLS(25/587)
  -- 企业微信群机器人
  wecom_enabled INTEGER NOT NULL DEFAULT 0,
  wecom_webhook TEXT NOT NULL DEFAULT '',
  -- 渠道触发的通知类型白名单（逗号分隔；空=全部类型）
  types TEXT NOT NULL DEFAULT '',
  updated_at DATETIME NOT NULL
);

INSERT INTO notify_settings (id, updated_at) VALUES (1, CURRENT_TIMESTAMP);

-- 外发日志：每条外发（成功/失败）都留痕，便于排查「为什么没收到」
CREATE TABLE notify_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  channel TEXT NOT NULL,                     -- email | wecom
  notification_id INTEGER NOT NULL DEFAULT 0,-- 0 表示测试发送
  user_id INTEGER NOT NULL DEFAULT 0,
  target TEXT NOT NULL DEFAULT '',           -- 邮箱地址 / webhook 主机
  title TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,                      -- success | failed
  error TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL
);
CREATE INDEX idx_notify_logs_created ON notify_logs(created_at DESC, id DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_notify_logs_created;
DROP TABLE IF EXISTS notify_logs;
DROP TABLE IF EXISTS notify_settings;
