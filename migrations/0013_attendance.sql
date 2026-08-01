-- +goose Up
-- 考勤排班：按员工/星期/班别配置上下班时间
CREATE TABLE attendance_schedules (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  employee_id INTEGER NOT NULL,
  weekday INTEGER NOT NULL,          -- 1=周一 .. 7=周日
  start_time TEXT NOT NULL,          -- "09:00"
  end_time TEXT NOT NULL,            -- "18:00"
  shift_type TEXT NOT NULL DEFAULT 'regular',  -- regular/night/shift
  created_at DATETIME,
  updated_at DATETIME,
  deleted_at DATETIME,
  FOREIGN KEY (employee_id) REFERENCES employees(id)
);
CREATE INDEX idx_schedules_employee ON attendance_schedules(employee_id);
CREATE INDEX idx_schedules_employee_weekday ON attendance_schedules(employee_id, weekday);

-- 请假申请：员工提交，HR/主管审批
CREATE TABLE leave_requests (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT UNIQUE NOT NULL,
  employee_id INTEGER NOT NULL,
  type TEXT NOT NULL,               -- annual/sick/personal/marriage/maternity/bereavement
  start_date DATETIME NOT NULL,
  end_date DATETIME NOT NULL,
  reason TEXT,
  status TEXT NOT NULL DEFAULT 'pending',  -- pending/approved/rejected
  approver_id INTEGER,
  approved_at DATETIME,
  reject_reason TEXT,
  created_at DATETIME,
  updated_at DATETIME,
  deleted_at DATETIME,
  FOREIGN KEY (employee_id) REFERENCES employees(id),
  FOREIGN KEY (approver_id) REFERENCES employees(id)
);
CREATE INDEX idx_leave_employee ON leave_requests(employee_id);
CREATE INDEX idx_leave_status ON leave_requests(status);

-- 考勤记录：按员工+日期标记出勤状态（normal/late/early/absent/leave）
CREATE TABLE attendances (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  employee_id INTEGER NOT NULL,
  date TEXT NOT NULL,               -- YYYY-MM-DD
  schedule_id INTEGER,
  status TEXT NOT NULL DEFAULT 'normal',  -- normal/late/early/absent/leave
  leave_type TEXT,
  remark TEXT,
  created_at DATETIME,
  updated_at DATETIME,
  deleted_at DATETIME,
  FOREIGN KEY (employee_id) REFERENCES employees(id),
  FOREIGN KEY (schedule_id) REFERENCES attendance_schedules(id)
);
CREATE INDEX idx_attendance_employee ON attendances(employee_id);
CREATE INDEX idx_attendance_date ON attendances(date);
CREATE INDEX idx_attendance_emp_date ON attendances(employee_id, date);

-- +goose Down
DROP TABLE IF EXISTS attendances;
DROP TABLE IF EXISTS leave_requests;
DROP TABLE IF EXISTS attendance_schedules;
