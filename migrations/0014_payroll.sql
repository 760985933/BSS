-- +goose Up
-- 劳动合同补录月薪（DEV_PLAN 数据模型已含 salary_cent，S2 实现时遗漏，此处补齐）
ALTER TABLE labor_contracts ADD COLUMN salary_cent INTEGER NOT NULL DEFAULT 0;

-- 薪资核算主表：按月按员工一条；金额一律整数分（*_cent）
CREATE TABLE payrolls (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at    DATETIME,
    updated_at    DATETIME,
    deleted_at    DATETIME,
    code          TEXT NOT NULL UNIQUE,
    employee_id   INTEGER NOT NULL,
    period        TEXT NOT NULL,                 -- YYYY-MM
    base_cent     INTEGER NOT NULL DEFAULT 0,    -- 基本工资
    bonus_cent    INTEGER NOT NULL DEFAULT 0,    -- 奖金
    deduction_cent INTEGER NOT NULL DEFAULT 0,   -- 扣款
    social_cent   INTEGER NOT NULL DEFAULT 0,    -- 社保个人部分
    tax_cent      INTEGER NOT NULL DEFAULT 0,    -- 个税
    net_cent      INTEGER NOT NULL DEFAULT 0,    -- 实发 = base+bonus-deduction-social-tax
    status        TEXT NOT NULL DEFAULT 'draft',-- draft/calced/paid
    paid_at       DATETIME,
    owner_id      INTEGER NOT NULL DEFAULT 0,
    remark        TEXT
);

CREATE INDEX idx_payrolls_employee ON payrolls(employee_id);
CREATE INDEX idx_payrolls_period   ON payrolls(period);
CREATE INDEX idx_payrolls_status   ON payrolls(status);
CREATE INDEX idx_payrolls_owner    ON payrolls(owner_id);

-- +goose Down
DROP TABLE payrolls;
ALTER TABLE labor_contracts DROP COLUMN salary_cent;
