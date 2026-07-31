-- +goose Up
-- M3-3 项目/交付管理：项目 + 成员(人天) + 任务/里程碑

CREATE TABLE projects (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT NOT NULL UNIQUE,                -- XM-YYYY-####
  name TEXT NOT NULL,
  customer_id INTEGER REFERENCES customers(id), -- 关联客户（可选）
  owner_id INTEGER NOT NULL REFERENCES employees(id), -- 项目经理
  status TEXT NOT NULL DEFAULT 'planning',  -- planning/in_progress/on_hold/completed/cancelled
  start_date TEXT,
  end_date TEXT,                            -- 预计结束日期
  description TEXT DEFAULT '',
  created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, deleted_at DATETIME
);
CREATE INDEX idx_projects_owner ON projects(owner_id);
CREATE INDEX idx_projects_customer ON projects(customer_id);

-- 项目成员 + 人天（计划/实际）
CREATE TABLE project_members (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id INTEGER NOT NULL REFERENCES projects(id),
  employee_id INTEGER NOT NULL REFERENCES employees(id),
  role TEXT DEFAULT '',                     -- 项目内角色（如 负责/开发/测试）
  planned_days REAL NOT NULL DEFAULT 0,     -- 计划人天（非金额，允许小数）
  actual_days REAL NOT NULL DEFAULT 0,      -- 实际人天
  created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, deleted_at DATETIME,
  UNIQUE(project_id, employee_id)
);
CREATE INDEX idx_pm_project ON project_members(project_id);
CREATE INDEX idx_pm_employee ON project_members(employee_id);

-- 任务 / 里程碑（kind 区分；状态 todo/doing/done）
CREATE TABLE project_tasks (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id INTEGER NOT NULL REFERENCES projects(id),
  kind TEXT NOT NULL DEFAULT 'task',        -- task | milestone
  title TEXT NOT NULL,
  assignee_id INTEGER REFERENCES employees(id), -- 负责人（可选）
  due_date TEXT,
  status TEXT NOT NULL DEFAULT 'todo',      -- todo/doing/done
  estimate_days REAL NOT NULL DEFAULT 0,    -- 预估人天
  sort INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, deleted_at DATETIME
);
CREATE INDEX idx_tasks_project ON project_tasks(project_id, sort);

-- +goose Down
DROP TABLE IF EXISTS project_tasks;
DROP TABLE IF EXISTS project_members;
DROP TABLE IF EXISTS projects;
