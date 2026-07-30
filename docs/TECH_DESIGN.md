# BSS 业务管理系统 · 技术开发方案

> 版本：v1.2　|　日期：2026-07-29　|　配套文档：docs/PRD.md
>
> 变更记录：v1.1 按评审意见调整——商单↔合同改 N:N（deal_contracts 中间表）；商单加 probability 并对齐 SF 阶段；合同状态机对齐 SF CLM；部署补 Docker 形态。
> v1.2 查漏补缺——状态机补回退边；N:N 关联补同客户校验；补终态锁定实现规则。

---

## 1. 技术选型

| 层 | 选型 | 理由 |
|---|---|---|
| 前端 | React 18 + TypeScript + Vite | 标配 |
| UI 组件 | Ant Design 5 | 中文管理后台生态最完整，表格/表单/日期组件开箱即用 |
| 数据请求 | TanStack Query | 缓存/loading/错误处理统一 |
| 后端 | Go 1.22+，路由 chi | chi 轻量、标准库风格，无框架锁定 |
| ORM | GORM | 自带软删除/Hook，匹配审计日志需求 |
| DB migration | goose（SQL 文件） | **禁用 GORM AutoMigrate 上生产**；从第一张表开始走 SQL migration |
| 数据库 | SQLite 3（WAL 模式） | 单文件、零运维；驱动用 modernc.org/sqlite（纯 Go，免 CGO，交叉编译友好） |
| 认证 | JWT（HMAC-SHA256）+ bcrypt | 单机场景足够 |
| 定时任务 | robfig/cron | 每日扫描生成提醒 |
| 打包 | Go embed 内嵌前端 dist | 交付 = 单个二进制 |

## 2. 整体架构

```
┌──────────────────────────────────────────────┐
│ 浏览器  React SPA (Ant Design)                │
└──────────────┬───────────────────────────────┘
               │ HTTP/JSON  (同源，JWT)
┌──────────────▼───────────────────────────────┐
│ Go 单二进制                                   │
│  ├─ embed: 前端静态文件                       │
│  ├─ middleware: 日志 → 鉴权JWT → RBAC数据范围 │
│  ├─ handlers → services → GORM               │
│  ├─ cron: 每日 09:00 扫描到期/逾期 → 通知     │
│  ├─ SQLite (data/bss.db, WAL)                │
│  └─ 附件目录 (data/uploads/)                  │
└──────────────────────────────────────────────┘
```

关键约束：SQLite 单写入者 → **写连接池固定为 1**，读连接池可多个；启动时执行 `PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON;`。

## 3. 项目结构

```
BSS/
├── cmd/server/main.go          # 入口：加载配置、初始化DB、启动HTTP+cron
├── internal/
│   ├── config/                 # 环境变量/配置文件
│   ├── db/                     # 连接、PRAGMA、goose migrate
│   ├── models/                 # GORM 模型
│   ├── handlers/               # HTTP 层（按模块分文件）
│   ├── services/               # 业务逻辑、状态机流转
│   ├── middleware/             # jwt.go rbac.go audit.go
│   └── pkg/
│       ├── money/              # 元 ↔ 分 转换（唯一入口）
│       ├── code/               # 业务单号生成
│       └── resp/               # 统一响应/错误码
├── migrations/                 # 0001_init.sql ...
├── web/                        # 前端
│   └── src/
│       ├── api/                # axios 封装 + 各模块接口
│       ├── pages/              # dashboard/customers/deals/contracts/payments/employees/system
│       ├── components/  router/  stores/
├── data/                       # 运行时生成：bss.db + uploads/（gitignore）
├── scripts/backup.sh           # DB + uploads 打包备份
└── docs/                       # PRD / 本方案 / 开发计划
```

## 4. 数据库设计（一期完整 DDL）

约定：全表 `id INTEGER PRIMARY KEY AUTOINCREMENT`、`created_at/updated_at/deleted_at TEXT(UTC ISO8601)`；金额统一 `*_cent INTEGER`（分）；所有外键建立索引。

```sql
-- 员工（用户）
CREATE TABLE employees (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  email TEXT NOT NULL UNIQUE,
  phone TEXT DEFAULT '',
  dept TEXT DEFAULT '',
  position TEXT DEFAULT '',
  role TEXT NOT NULL DEFAULT 'sales',      -- admin/sales/sales_lead/finance/hr
  password_hash TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',   -- active/disabled
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL, deleted_at TEXT
);

-- 客户
CREATE TABLE customers (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT NOT NULL UNIQUE,               -- KH-2026-0001
  name TEXT NOT NULL UNIQUE,
  industry TEXT DEFAULT '', source TEXT DEFAULT '', level TEXT DEFAULT '',
  owner_id INTEGER NOT NULL,               -- 0 = 无主（公海客户）；非 0 时指向 employees(id)
  remark TEXT DEFAULT '',
  -- M3-1 公海池
  last_followed_at DATETIME,               -- 最后跟进时间（编辑客户/加联系人/建商单/推进阶段时刷新）
  claimed_at DATETIME,                     -- 领取时间，用于保护期判定
  pool_reason TEXT NOT NULL DEFAULT '',    -- 进入公海的原因
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL, deleted_at TEXT
);
CREATE INDEX idx_customers_owner ON customers(owner_id);
CREATE INDEX idx_customers_pool ON customers(owner_id, last_followed_at);

-- 公海流水（M3-1）：业务可查的领取/释放/回收轨迹，区别于通用 audit_logs
CREATE TABLE customer_pool_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  customer_id INTEGER NOT NULL,
  action TEXT NOT NULL,                    -- claim/release/recycle/assign
  from_owner_id INTEGER NOT NULL DEFAULT 0,
  to_owner_id INTEGER NOT NULL DEFAULT 0,
  operator_id INTEGER NOT NULL DEFAULT 0,  -- 0 = 系统（定时回收）
  reason TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL
);

-- 公海规则（M3-1，单行配置）
CREATE TABLE pool_settings (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  enabled INTEGER NOT NULL DEFAULT 0,              -- 是否启用自动回收
  max_claim_per_sales INTEGER NOT NULL DEFAULT 50, -- 单人持有上限（0=不限）
  idle_days_no_follow INTEGER NOT NULL DEFAULT 30, -- 超 N 天无跟进回收（0=停用该条）
  idle_days_no_deal INTEGER NOT NULL DEFAULT 60,   -- 领取后超 N 天未建商单回收（0=停用该条）
  protect_days INTEGER NOT NULL DEFAULT 7,         -- 领取保护期
  updated_at DATETIME NOT NULL
);

-- 联系人
CREATE TABLE contacts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  customer_id INTEGER NOT NULL REFERENCES customers(id),
  name TEXT NOT NULL, phone TEXT DEFAULT '', email TEXT DEFAULT '',
  position TEXT DEFAULT '', is_primary INTEGER NOT NULL DEFAULT 0,
  remark TEXT DEFAULT '',
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL, deleted_at TEXT
);
CREATE INDEX idx_contacts_customer ON contacts(customer_id);

-- 商单
CREATE TABLE deals (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT NOT NULL UNIQUE,               -- SD-2026-0001
  customer_id INTEGER NOT NULL REFERENCES customers(id),
  title TEXT NOT NULL,
  amount_cent INTEGER NOT NULL DEFAULT 0,
  probability INTEGER NOT NULL DEFAULT 10, -- 赢单概率%，按阶段带出可手调（加权预测用）
  expected_sign_date TEXT,
  status TEXT NOT NULL DEFAULT 'prospecting', -- prospecting/qualifying/proposal/negotiating/won/lost
  lost_reason TEXT DEFAULT '',             -- no_purchase/competitor/budget/qualified_out/other
  owner_id INTEGER NOT NULL REFERENCES employees(id),
  remark TEXT DEFAULT '',
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL, deleted_at TEXT
);
CREATE INDEX idx_deals_customer ON deals(customer_id);
CREATE INDEX idx_deals_owner ON deals(owner_id);
CREATE INDEX idx_deals_status ON deals(status);

-- 合同
CREATE TABLE contracts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT NOT NULL UNIQUE,               -- HT-2026-0001
  customer_id INTEGER NOT NULL REFERENCES customers(id),
  title TEXT NOT NULL,
  amount_cent INTEGER NOT NULL DEFAULT 0,  -- 与商单金额为独立口径，不强制勾稽
  sign_date TEXT, start_date TEXT, expire_date TEXT,
  status TEXT NOT NULL DEFAULT 'draft',    -- draft/pending/signed/performing/completed/cancelled/terminated/expired（approving 预留）
  terminate_reason TEXT DEFAULT '',
  owner_id INTEGER NOT NULL REFERENCES employees(id),
  remark TEXT DEFAULT '',
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL, deleted_at TEXT
);
CREATE INDEX idx_contracts_customer ON contracts(customer_id);
CREATE INDEX idx_contracts_expire ON contracts(expire_date);

-- 商单 ↔ 合同 N:N 关联（评审确认：最开放方案）
CREATE TABLE deal_contracts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  deal_id INTEGER NOT NULL REFERENCES deals(id),
  contract_id INTEGER NOT NULL REFERENCES contracts(id),
  created_at TEXT NOT NULL,
  UNIQUE(deal_id, contract_id)
);
CREATE INDEX idx_dc_deal ON deal_contracts(deal_id);
CREATE INDEX idx_dc_contract ON deal_contracts(contract_id);

-- 回款计划
CREATE TABLE payment_plans (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  contract_id INTEGER NOT NULL REFERENCES contracts(id),
  period_no INTEGER NOT NULL,
  due_date TEXT NOT NULL,
  amount_cent INTEGER NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',  -- pending/partial/paid（逾期为派生）
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL, deleted_at TEXT
);
CREATE INDEX idx_plans_contract ON payment_plans(contract_id);
CREATE INDEX idx_plans_due ON payment_plans(due_date);

-- 回款记录
CREATE TABLE payment_records (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  contract_id INTEGER NOT NULL REFERENCES contracts(id),
  plan_id INTEGER REFERENCES payment_plans(id),   -- 可空=不核销计划
  amount_cent INTEGER NOT NULL,
  paid_at TEXT NOT NULL,
  method TEXT DEFAULT '', remark TEXT DEFAULT '',
  created_by INTEGER NOT NULL REFERENCES employees(id),
  created_at TEXT NOT NULL, updated_at TEXT NOT NULL, deleted_at TEXT
);
CREATE INDEX idx_records_contract ON payment_records(contract_id);

-- 站内通知
CREATE TABLE notifications (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL REFERENCES employees(id),
  type TEXT NOT NULL,                      -- contract_expiring/payment_overdue
  title TEXT NOT NULL, content TEXT DEFAULT '',
  entity_type TEXT NOT NULL, entity_id INTEGER NOT NULL,
  dedup_key TEXT NOT NULL UNIQUE,          -- type+entity+date 防重复
  is_read INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL
);
CREATE INDEX idx_notif_user ON notifications(user_id, is_read);

-- 附件
CREATE TABLE attachments (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  entity_type TEXT NOT NULL,               -- contract（一期仅合同）
  entity_id INTEGER NOT NULL,
  file_name TEXT NOT NULL, file_path TEXT NOT NULL,
  file_size INTEGER NOT NULL, mime TEXT DEFAULT '',
  uploaded_by INTEGER NOT NULL REFERENCES employees(id),
  created_at TEXT NOT NULL, deleted_at TEXT
);

-- 审计日志（无 updated/deleted，只追加）
CREATE TABLE audit_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  entity_type TEXT NOT NULL, entity_id INTEGER NOT NULL,
  action TEXT NOT NULL,                    -- create/update/delete/transfer/status_change
  operator_id INTEGER NOT NULL REFERENCES employees(id),
  before_json TEXT DEFAULT '', after_json TEXT DEFAULT '',
  created_at TEXT NOT NULL
);
CREATE INDEX idx_audit_entity ON audit_logs(entity_type, entity_id);

-- 单号计数器（年度递增）
CREATE TABLE code_counters (
  prefix TEXT NOT NULL,                    -- KH/SD/HT
  year INTEGER NOT NULL,
  seq INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (prefix, year)
);
```

## 5. API 设计规范

- 风格：REST，`/api/v1/...`；列表统一 `?page=&size=&keyword=&status=&owner_id=`，返回 `{list, total, page, size}`。
- 统一响应：成功 `{code:0, data:...}`；失败 `{code:<业务码>, message:"用户可读信息"}`。HTTP 状态码语义化（401/403/404/422/500）。
- **ID 一律 JSON 字符串序列化**（防 JS Number 精度问题）；**金额一律 integer 分**，元↔分转换只发生在前端展示层（`pkg/money` 是唯一换算入口）。
- 时间：请求/响应均为 UTC ISO8601 字符串；纯日期字段 `YYYY-MM-DD`。
- 错误码段：1xxx 通用，2xxx 客户/商单，3xxx 合同/回款，4xxx 权限。

核心路由（节选）：

```
POST   /api/v1/auth/login
GET    /api/v1/employees              POST /api/v1/employees
GET    /api/v1/customers              POST /api/v1/customers
GET    /api/v1/customers/:id          PUT  /api/v1/customers/:id
POST   /api/v1/customers/:id/transfer        # 转移负责人（审计）
GET    /api/v1/customers/:id/contacts POST ...
GET    /api/v1/deals                  POST /api/v1/deals
POST   /api/v1/deals/:id/status       {to:"won"|"lost"|...}  # 状态机唯一入口
GET    /api/v1/contracts              POST /api/v1/contracts   # body 含 deal_ids:[]（0..N）
PUT    /api/v1/contracts/:id/deals           # 调整关联商单（仅 won 状态可选，审计）
POST   /api/v1/contracts/:id/status
POST   /api/v1/contracts/:id/attachments     # multipart
GET    /api/v1/contracts/:id/payments        # 计划+记录+汇总
POST   /api/v1/payment-plans   POST /api/v1/payment-records
GET    /api/v1/notifications   POST /api/v1/notifications/:id/read
GET    /api/v1/dashboard/summary
```

## 6. 关键技术决策（对应 PRD §7）

1. **金额**：DB 只存 `INTEGER` 分；Go 结构体用 `int64` 字段 + `json:",string"` 不需要（分不会超 2^53）；前端表单输入元，提交前 ×100 取整。
2. **状态机**：集中在 `services/*_state.go`，用 map 定义合法流转；handler 不直接改 status 字段。
   - 商单（对齐 SF Opportunity 裁剪 6 态，含回退边）：
     `dealFlow = {prospecting:[qualifying,lost], qualifying:[proposal,prospecting,lost], proposal:[negotiating,qualifying,lost], negotiating:[won,proposal,lost]}`；won/lost 为终态。
   - 合同（对齐 SF CLM 裁剪）：主线 `draft→pending→signed→performing→completed`；回退 `pending→draft`（Re-draft）；旁路 `draft/pending→cancelled`，`signed/performing→terminated/expired`；`approving` 枚举预留（二期审批流）。
   - 商单阶段变更时自动带出默认 probability（10/25/60/80/100/0），允许手动覆盖；回退时概率同样按目标阶段重置再允许手调。
   - 商单退出标准一期软校验：不满足时 422+`warning:true` 由前端弹确认后重发（带 `force:true`），两次请求均记审计；配置项预留二期切硬校验。
3. **逾期派生**：`due_date < date('now') AND status != 'paid'` 在 SQL 查询时计算，列表返回 `is_overdue` 布尔字段。
4. **单号生成**：`code_counters` 行内 `UPDATE ... SET seq=seq+1 RETURNING seq`（SQLite 支持），在写事务内完成，防并发重号。
5. **RBAC 数据范围**：middleware 把 `(role, userID, dept)` 注入 context；service 层统一 `scope(query, ctx)`：admin/finance 不加条件，sales 加 `owner_id=?`，sales_lead 加 `owner_id IN (SELECT id FROM employees WHERE dept=?)`。
6. **审计日志**：GORM Hook（AfterCreate/AfterUpdate/AfterDelete）序列化前后值 JSON 写 `audit_logs`，异步无感。
7. **附件**：落盘 `data/uploads/YYYY/MM/<uuid>.<ext>`，DB 存相对路径；下载走鉴权接口 `GET /api/v1/attachments/:id/download`，不暴露静态目录。
8. **通知去重**：`dedup_key = type:entity_id:YYYY-MM-DD` 唯一索引，INSERT OR IGNORE。
9. **备份**：`scripts/backup.sh` 用 SQLite 在线备份 `sqlite3 bss.db ".backup ..."`（或 VACUUM INTO）+ tar uploads，cron 每日执行，保留 30 份。
10. **商单↔合同 N:N**：关联校验在 service 层——仅 won 状态商单可被关联，**且所选商单的 customer_id 必须与合同 customer_id 一致**（合同签约主体唯一）；合同详情/商单详情互相展示对方列表都走 `deal_contracts` join；解除关联写审计日志，不物理删合同。
11. **终态锁定**：商单 won/lost 后、合同 signed 及以后状态，`amount_cent/customer_id/关联商单` 字段在 update 接口白名单中剔除（service 层 `editableFields(status)` 统一判定）；补充协议等变更引导新建单据并关联原单。已核销的回款计划禁改金额/禁删（校验 `EXISTS(SELECT 1 FROM payment_records WHERE plan_id=?)`）。

## 7. 部署方案（评审确认：Docker 或裸服务器，非 SaaS）

- 构建：`cd web && npm run build` → Go `embed.FS` 内嵌 `dist/` → 单二进制 `bss-server`。
- **裸服务器**：`./bss-server -addr 127.0.0.1:8080 -data ./data`，前面挂 Caddy/Nginx 反代 + HTTPS；交付物 = 二进制 + systemd unit + backup.sh + 运维 README（含初始化 admin 说明）。
- **Docker**：单阶段 Dockerfile（node 构建前端 → golang 构建后端 → alpine 运行时）；`docker run -v bss-data:/data -p 8080:8080 bss:latest`；`data/`（SQLite + uploads）挂 named volume 持久化；备份 = 对 volume 跑 backup.sh 或 `docker run --rm -v bss-data:/data alpine tar`。
- 单租户单实例，不做编排/多副本（SQLite 约束，与部署形态决策一致）。

## 8. 测试策略

- 后端：service 层单测（状态机、金额、单号并发）+ handler 集成测试（httptest + 内存 SQLite）。
- 前端：Vitest 覆盖 money 转换与表单校验；核心链路人工验收清单（按 PRD §4 逐条）。
- 里程碑出口标准见 docs/DEV_PLAN.md。
