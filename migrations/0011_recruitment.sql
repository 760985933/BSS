-- +goose Up
-- M6-S1 招聘漏斗：招聘职位 + 候选人
CREATE TABLE job_posts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT NOT NULL UNIQUE,
  title TEXT NOT NULL DEFAULT '',
  dept TEXT DEFAULT '',
  headcount INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'open',       -- open 招聘中 / closed 已关闭
  description TEXT DEFAULT '',
  owner_id INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, deleted_at DATETIME,
  FOREIGN KEY (owner_id) REFERENCES employees(id)
);
CREATE INDEX idx_jp_owner ON job_posts(owner_id);

CREATE TABLE candidates (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  job_post_id INTEGER DEFAULT NULL,
  name TEXT NOT NULL DEFAULT '',
  phone TEXT DEFAULT '',
  email TEXT DEFAULT '',
  stage TEXT NOT NULL DEFAULT 'apply',        -- apply/screen/interview/offer/hired/rejected
  source TEXT DEFAULT '',
  resume_url TEXT DEFAULT '',
  owner_id INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, deleted_at DATETIME,
  FOREIGN KEY (job_post_id) REFERENCES job_posts(id),
  FOREIGN KEY (owner_id) REFERENCES employees(id)
);
CREATE INDEX idx_cand_job ON candidates(job_post_id);
CREATE INDEX idx_cand_stage ON candidates(stage);
CREATE INDEX idx_cand_owner ON candidates(owner_id);

-- +goose Down
DROP TABLE IF EXISTS candidates;
DROP TABLE IF EXISTS job_posts;
