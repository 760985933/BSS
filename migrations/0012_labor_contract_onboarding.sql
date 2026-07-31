-- +goose Up
-- M6-S2 劳动合同 + 入职管理
CREATE TABLE labor_contracts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT NOT NULL UNIQUE,
  employee_id INTEGER NOT NULL DEFAULT 0,
  type TEXT NOT NULL DEFAULT 'fixed',          -- fixed/nonfixed/internship/parttime
  start_date DATETIME,
  end_date DATETIME,
  sign_date DATETIME,
  probation_months INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'draft',        -- draft/active/expired/renewed/terminated
  terminate_reason TEXT,
  owner_id INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, deleted_at DATETIME,
  FOREIGN KEY (employee_id) REFERENCES employees(id),
  FOREIGN KEY (owner_id) REFERENCES employees(id)
);
CREATE INDEX idx_lc_emp ON labor_contracts(employee_id);
CREATE INDEX idx_lc_status ON labor_contracts(status);
CREATE INDEX idx_lc_owner ON labor_contracts(owner_id);

CREATE TABLE onboardings (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT NOT NULL UNIQUE,
  employee_id INTEGER NOT NULL DEFAULT 0,
  candidate_id INTEGER DEFAULT NULL,
  step_profile TEXT NOT NULL DEFAULT 'pending',   -- pending/done
  step_equip TEXT NOT NULL DEFAULT 'pending',
  step_training TEXT NOT NULL DEFAULT 'pending',
  step_probation TEXT NOT NULL DEFAULT 'pending',
  status TEXT NOT NULL DEFAULT 'in_progress',     -- in_progress/completed
  owner_id INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, deleted_at DATETIME,
  FOREIGN KEY (employee_id) REFERENCES employees(id),
  FOREIGN KEY (candidate_id) REFERENCES candidates(id),
  FOREIGN KEY (owner_id) REFERENCES employees(id)
);
CREATE INDEX idx_ob_emp ON onboardings(employee_id);
CREATE INDEX idx_ob_cand ON onboardings(candidate_id);
CREATE INDEX idx_ob_status ON onboardings(status);
CREATE INDEX idx_ob_owner ON onboardings(owner_id);

-- +goose Down
DROP TABLE IF EXISTS onboardings;
DROP TABLE IF EXISTS labor_contracts;
