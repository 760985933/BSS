# BSS 业务管理系统 · 开发计划

> 版本：v1.2　|　日期：2026-07-29　|　配套：docs/PRD.md、docs/TECH_DESIGN.md
> 工作量按 1–2 名全栈开发估算，可按实际人力等比缩放。顺序不可乱：每个 Sprint 依赖前一个的数据模型。
>
> 变更记录：v1.1 按评审意见调整——商单↔合同 N:N（Sprint 3/4 任务同步更新）；商单状态机对齐 SF 6 态 + 概率；被砍功能统一进 PRD 附录 A Backlog；Excel 导入确认一期不做。
> v1.2 查漏补缺——Sprint 3 加权预测验收修正（原误挂在未开工的仪表盘上）；Sprint 4 补同客户校验与回退验收；Sprint 5 补已核销计划调整约束。

---

## 里程碑总览

| 里程碑 | 内容 | 出口标准 |
|---|---|---|
| M0 工程基座 | 脚手架 + 登录骨架 | 能登录、空布局跑起来、migration 可用 |
| M1 一期 MVP | 11 张核心表全链路可用 | 通过 PRD §4 逐条验收 + 模拟数据跑通"获客→收钱" |
| M2 二期增强 | 审批/开票/报表/审计查询 | 财务全流程无纸面操作 |
| M3 三期可选 | 交付管理/通知渠道扩展 | 视业务反馈排期 |

---

## M0 · 工程基座（约 3–5 天）—— ✅ 已完成（2026-07-29）

**目标**：所有后续开发的"地基"，地基不打后面全返工。

- [x] 仓库初始化：Go module + web (Vite+React+TS+AntD) + Makefile（dev/build/test）
- [x] SQLite 连接封装：WAL、busy_timeout=5000、foreign_keys=ON、写连接池=1
- [x] goose migration 接入 + `0001_init.sql`（§4 全部 DDL 一次建全，后续改动走增量 migration）
- [x] 统一响应/错误码 `pkg/resp`；金额 `pkg/money`；单号 `pkg/code`（含并发单测）
- [x] JWT 登录 + bcrypt + 首启初始化 admin + RBAC middleware（角色+数据范围注入 context）
- [x] 前端：axios 封装（401 跳登录）、路由 + 主布局（侧边栏按角色渲染菜单）
- [x] GORM 审计 Hook 骨架（AfterCreate/Update → audit_logs，软删除识别为 delete）

**验收**：admin 登录后看到空仪表盘 ✅；`make test` 通过 ✅；migration 可重复执行 ✅（二次启动 "no migrations to run"）。

**实施记录**：
- 端到端验证通过：错误密码 401 / 登录取 token / me / employees（数据范围）/ 无 token 拦截 / 改密 / 审计写入（create operator=0 系统、update operator=1）/ SPA 托管与前端路由回退 / API 404 JSON / UTC 时间戳
- 踩坑记录（详见 `.workbuddy/memory/MEMORY.md`）：GORM 回调内另开会话死锁 → 改 ConnPool 原生 SQL；glebarez 时间列须 DATETIME；UTC 需 NowFunc；npm 代理卡死改 pnpm + onlyBuiltDependencies: [esbuild]
- 启动方式：`make dev-server`（开发）/ `make run`（构建单二进制运行，embed 前端）

---

## M1 · 一期 MVP（约 5–6 周，6 个 Sprint）

### Sprint 1 · 员工档案 + 权限收尾（约 4 天）—— ✅ 已完成（2026-07-29）
- 员工 CRUD、停用/启用、角色分配、admin 重置密码；部门枚举配置页（dicts 通用字典表，软删除友好的部分唯一索引）
- 数据范围 `scope()` 落地 + 单测（ScopeOwner 5 用例 + CanAccessOwner 5 用例）
- **验收**：主管登录仅见本部门员工 ✅；销售全量只读（PRD §6 口径）✅；admin 可建号/停用、停用账号禁登录 ✅；销售建号 403 ✅；邮箱唯一 409 ✅；不能停用自己、保留最后一名 active admin ✅；重置密码后初始密码可登录 ✅；员工操作审计留痕 ✅

> 注：原计划验收语"销售只见本部门"与 PRD §6 矩阵（销售=全量只读列表）不一致，已按 PRD 实现并在此修正。

### Sprint 2 · 客户 + 联系人（约 5 天）—— ✅ 已完成（2026-07-29）
- 客户 CRUD/搜索/筛选/真分页、名称唯一 409、单号 KH-（seed 字典：行业/来源/等级/回款方式）
- 联系人子表、首要联系人唯一（事务内置换）
- 客户详情页（信息 + 联系人管理 + 商单/合同/回款 Tab 占位）
- 客户转移 + 审计（before/after 完整行）
- **验收**：销售仅见本人客户 ✅；改他人客户 403 ✅；转移后审计 owner 前后值 ✅；首要联系人唯一 ✅；有商单客户禁删（2001）✅；**1 万条数据：列表 16ms / 搜索 13ms / 第 400 页 34ms**（远低于 300ms）✅
- 审计模块三轮修复并留下回归测试（audit_test.go）：GORM `Clauses["WHERE"].Expression` 顶层是 `clause.Where` 包装（含 Expr/AndConditions/Eq/IN 多形态，IN 的列名在回调阶段是 `~~~py~~~` 占位符）；软删除走 Delete 回调链且主键条件在 gorm:delete 内部才添加（Before 提取不到，只能 After 快照）。

### Sprint 3 · 商单（约 4 天）—— ✅ 已完成（2026-07-30）
- 商单 CRUD、单号 SD-、金额元↔分全链路
- 状态机（对齐 SF 6 态）prospecting→qualifying→proposal→negotiating→won/lost，含回退边与退出标准软校验（422 warning + force 确认）
- probability 按阶段自动带出（10/25/60/80/100/0）可手调；lost 必填输单原因（5 类枚举）
- 加权预测金额 Σ(金额×概率) 的 service 层计算 + 单测（仪表盘呈现在 Sprint 6）
- 客户详情页商单区接入；商单列表页（阶段 Tag/赢率进度条/推进 Modal/输单登记）
- **验收**：跳级 422 ✅；回退合法且概率重置、审计留痕 ✅；金额 0 推进 warning + force 通过 ✅；lost 必填原因且终态不可逆 ✅；won 后金额/客户只读、仅 remark 可改 ✅；加权预测端到端精确（10 万×10%=1 万）✅；单测 5 项（流转/回退/输单/锁定/预测）✅

### Sprint 4 · 合同 + 附件（约 6 天）—— ✅ 已完成（2026-07-30）
- 合同 CRUD、单号 HT-；**deal_contracts N:N 关联（0..N 个 won 商单多选，且必须与合同同客户）+ PUT /contracts/:id/deals**
- 状态机 draft→pending→signed→performing→completed，回退 pending→draft，旁路 cancelled（签约前）/terminated（签约后必填原因）/expired（人工标记）
- signed 后金额/客户/关联商单只读（editableFields 判定）
- 附件上传/下载（鉴权、类型白名单、20MB）
- **验收**：仅 won 商单可被关联 ✅；跨客户商单关联被 422 拒绝 ✅；多商单合并签一份合同可建 ✅；pending 可退回 draft 并记审计 ✅；附件非登录不可下载（401）✅；terminated 必填原因（422）✅；signed 后金额/商单锁定（422）✅；E2E 18 项断言全绿 ✅

### Sprint 5 · 回款 ✅（约 5 天）
- 回款计划 CRUD（总额 ≤ 合同额校验）；**已核销计划禁改金额/禁删，只能新增调整期**
- 回款记录录入/核销、计划状态自动推进 pending→partial→paid
- 逾期派生查询（is_overdue）+ 合同维度汇总（应收/已收/余额/逾期额）
- **验收**：核销金额准确到分（边界用例：多退少补、跨计划核销）；已核销计划改删被 422 拒绝；财务角色才能录入
- 实现：后端 `internal/services/payment.go` + `internal/handlers/payment.go`（计划 CRUD 排除财务、回款录入仅 admin/finance、ScopeOwner 行级校验）；前端 `web/src/pages/Payments.tsx` + `web/src/components/PaymentCenter.tsx`（汇总卡片/计划表/记录表/录入表单，按角色门控）；`web/src/api/index.ts` 增类型与接口；单测（后端 6 + 前端 3）、E2E（26 断言全绿）

### Sprint 6 · 提醒 + 仪表盘 + 联调 ✅（约 6 天）
- cron 每日 09:00 扫描：合同 30 天内到期、回款逾期 → notifications（dedup_key 去重）✅
- 站内通知列表/未读角标/标记已读 ✅
- 仪表盘：4 张卡片（本月签约/本月回款/进行中商单/逾期金额）+ 3 个列表（即将到期合同/逾期回款/近期赢单），数据范围按角色过滤 ✅
- 全链路联调 + 造数脚本（`cmd/seed`）+ 备份脚本（`scripts/backup.sh`）+ PRD §4 逐条验收 ✅
- **验收（M1 出口）**：从建客户 → 商单赢单 → 签合同 → 回款计划 → 到账核销 → 逾期提醒，E2E 29 断言全绿 ✅；备份脚本可用 ✅

---

## M2 · 二期增强（约 3–4 周）—— ✅ 已完成（2026-07-30）

1. ✅ **审批流**：合同签约审批、商单折扣审批；新增 approvals 表，状态机插入 `pending_approval` 节点（commit `184040c`）
2. ✅ **开票管理**：invoices 表、开票与合同/回款关联、开票状态机（commit `b554254`）
3. ✅ **报表中心**：月度签约/回款趋势、销售排行、客户转化漏斗；CSV 导出（commit `b3cdac7`）
4. ✅ **审计查询页 + 离职交接**：audit_logs 按实体/操作/时间查询；员工停用前批量转移名下客户/商单/合同（commit 见本次提交）

> M2 验收：后端 `go test ./...`、前端 `pnpm test`（34 用例）与三套 E2E（审批/开票沿用既有、报表 `e2e_reports.py`、审计+交接 `e2e_m24.py`）全绿。

## M3 · 三期可选（按业务反馈）

- 候选清单已全部交付：公海池（M3-1）、Excel 导入（M3-2）、项目/交付（M3-3）、通知渠道扩展（M3-4）

> 完整候选清单统一维护在 **PRD 附录 A · Backlog**，此处不重复登记。

### M3-1 客户公海池 ✅（2026-07-30）
- 无主客户池（`owner_id=0`）+ 销售领取 / 超时自动回收（每日 02:00 定时任务）
- 回收规则单行表 `pool_settings`：领取上限、超期未跟进、领取后未建商单、保护期
- `customer_pool_logs` 流水（领取/释放/回收/指派），区别于通用审计
- 离职交接打通：离职不指定交接人 → 名下客户退回公海（`ErrSuccessorRequired` 守卫有商单/合同时必须指定交接人）
- 配套迁移 `0007`：去掉 `customers.owner_id` 指向 employees 的外键，以支持 `owner_id=0` 公海语义
- 验收：后端 `go test ./...`、前端 `pnpm test`（39 用例）、E2E `e2e_m31.py`（44 用例）全绿

### M3-2 Excel 数据导入 ✅（2026-07-31）
- 存量客户/联系人批量导入：无新增表（复用 customers/contacts），excelize 解析 .xlsx（模板表头 + 填写说明双 sheet）。
- 端点：`POST /api/v1/imports/customers`（multipart，限 admin/sales/sales_lead）、`GET /api/v1/imports/customers/template`（下载模板）。
- 列：客户名称(必填,按名去重跳过)、行业/来源/等级、负责人邮箱(留空归导入人；admin/主管可跨人分配)、备注、联系人*(可选,含是否首要联系人)。
- 返回导入摘要（有效行/新建客户/新建联系人/跳过/失败行明细），逐行事务落库，负责人解析失败整行跳过并计入错误。
- 前端：`web/src/pages/ImportCustomers.tsx`（拖拽上传 + 模板下载 + 结果统计与错误明细），菜单「Excel 导入」。
- 验收：单测 `import_test.go`（基础导入/重名跳过/无效负责人邮箱）+ `cmd/server/m3_e2e_test.go`（httptest 复用生产路由全链路）全绿。

### M3-3 项目/交付管理 ✅（2026-07-31）
- 三表：projects（XM 单号、状态机 planning/in_progress/on_hold/completed/cancelled、关联客户/项目经理）、project_members（成员+计划/实际人天，人天用 REAL 允许小数）、project_tasks（任务/里程碑合并 kind，状态 todo/doing/done，预估人天）。
- 迁移 `0008_projects.sql`；模型 `Project.Members/Tasks` 关联字段；`ScopeProject` 数据范围（admin/finance/hr 看全部，销售/主管看「自己负责或自己是成员」）；写操作鉴权 owner/成员。
- 端点：项目 CRUD + `/members`、`/tasks` 增删改查（限 admin/sales/sales_lead）。
- 前端：`web/src/pages/Projects.tsx`（列表 + 新建/编辑 + Drawer 详情三 Tab：任务/里程碑/成员含人天汇总）。
- 验收：`cmd/server/m3_e2e_test.go` 覆盖项目建/成员/任务/详情/删除全链路；`go test ./...` 与 `pnpm build` 全绿；`make e2e` 业务流回归 ALL PASS 无回归。

### M3-4 通知渠道扩展 ✅（2026-07-31）
- 站内信之外新增两个外发渠道：**SMTP 邮件**（隐式 TLS/STARTTLS 自适应，主题 RFC2047 编码、正文 base64）与**企业微信群机器人 webhook**（markdown 消息，校验 `errcode`）。
- 迁移 `0009_notify_channels.sql`：`notify_settings` 单行配置（两渠道开关 + SMTP 参数 + webhook + 类型白名单）、`notify_logs` 外发日志（成功/失败均留痕，含错误原因）。
- 派发挂在 `ScanReminders` 尾部（`defer dispatchAll`）：新生成的站内通知按开关同步外发；**渠道全关时零开销直接返回**，外发失败只写日志，绝不影响站内信与扫描本身。
- 安全：`smtp_password` 不参与 JSON 序列化，读接口返回 `********` 掩码，PUT 回传掩码或留空表示「保持原值」。
- 端点（仅 admin）：`GET/PUT /notify-settings`、`POST /notify-settings/test`（邮件可指定收件人，缺省发给自己）、`GET /notify-logs`。
- 前端：`web/src/pages/NotifyChannels.tsx`（SMTP 表单 + webhook + 推送类型白名单 + 一键测试 + 外发日志表），菜单「通知渠道」限 admin。
- 验收：单测 `notify_channel_test.go`（默认关闭/掩码保存/校验/白名单过滤/webhook 成功与 errcode 失败落日志/邮件打桩/报文编码）+ `cmd/server/m34_notify_e2e_test.go`（httptest 假 webhook 全链路）+ 权限矩阵补充 M3-2/3/4 端点；`go test ./...`、`pnpm build`、`make e2e` 全绿。

---

## M4 · 业务增强（按需）

> 路线图主线 M0–M3 已全部交付。以下是 PRD 附录 A 中"视业务反馈"的二期候选功能，按业务需要逐块补做，每块独立提交。

### M4-1 客户查重合并 ✅（2026-07-31）
- 基于联系人**手机/邮箱**硬证据识别疑似重复客户（`Customer.Name` 有唯一约束，完全同名在 DB 层被拦，故以跨客户共享联系方式作为重复证据，比名称模糊匹配更可靠）。
- 迁移：**零新增表**。合并 `MergeCustomers` 在事务内把从客户的联系人/商单/合同 `customer_id` 改为主客户后软删从客户；回款计划/记录仅挂合同，随合同自动归属主客户。
- 安全：查重与合并均限 **admin**（跨 owner 数据迁移）；合并前降级从客户 primary 联系人，避免主客户出现多个首要联系人。
- 端点（仅 admin）：`GET /customers/duplicates`、`POST /customers/merge {primary_id, secondary_id}`。
- 前端：`web/src/pages/DuplicateCustomers.tsx`（按重复组分 Card，Radio 选主、一键合并其余），菜单「客户查重」限 admin。
- 验收：单测 `dupmerge_test.go`（查重分组/合并迁移/自身合并报错/缺失客户 404）+ `cmd/server/m2x_dupmerge_e2e_test.go`（httptest 全链路：查重→合并→复查重清空→从客户软删、下游迁移）；`go test ./...`、`pnpm build` 全绿。

### M4-2 商单输单分析 ✅（2026-07-31）
- 对 `lost` 商单按**输单原因 / 负责人 / 月份**三维聚合（`LostDealAnalysis`），纯只读统计。
- 原因枚举复用商单状态机 `validLostReasons`（no_purchase/competitor/budget/qualified_out/other），前端映射中文标签；月份取 `updated_at` 的 `YYYY-MM`（输单发生月）。
- 端点（限 admin/主管）：`GET /reports/lost-analysis`。
- 前端：`web/src/pages/LostDealAnalysis.tsx`（三张 Card + Progress 条形），菜单「输单分析」限 admin/主管，图标 PieChartOutlined。
- 验收：单测 `analysis_test.go`（reason/owner/month 聚合）+ `cmd/server/m2x_analysis_e2e_test.go`（httptest 端点）；`go test ./...`、`pnpm build` 全绿。

### M4-3 银企对账 ✅（2026-07-31）
- 新增银行流水表 `bank_statements`（迁移 `0010_bank_statements.sql`）：交易日期 / 对方户名 / 金额 / 方向(收/付) / 摘要，及 `payment_record_id` 勾对关联（FK 到回款记录）。
- 服务层 `Reconcile` 在事务内将流水关联到回款记录，并阻止**同一回款被多条流水勾对**；`Unreconcile` 取消勾对；`ReconciliationSummary` 输出未达账项——银行已收企业未收（income 且未勾对流水）、企业已收银行未收（未被任何流水勾对的回款记录）。
- 端点（限 admin/finance）：`POST /bank-statements`、`GET /bank-statements`（可按 reconciled 过滤）、`POST /bank-statements/{id}/reconcile`、`POST /bank-statements/{id}/unreconcile`、`GET /reconciliation`。
- 前端：`web/src/pages/BankReconciliation.tsx`（流水录入草稿 + 未达账项汇总 Alert + 流水列表勾对/取消），菜单「银企对账」限 admin/finance，图标 AccountBookOutlined。
- 验收：单测 `reconciliation_test.go`（录入/勾对/重复勾对报错/未达账项）+ `cmd/server/m2x_reconciliation_e2e_test.go`（httptest 录入→勾对→汇总）；`go test ./...`、`pnpm build`、`make e2e` 全绿无回归。

---

## 风险与预案

| 风险 | 预案 |
|---|---|
| ~~商单↔合同基数不确定~~（已解决） | 评审已定为 N:N 中间表（最开放方案），1:1/1:N/N:N 场景全覆盖，此风险关闭 |
| SQLite 并发写入瓶颈 | 已用单写连接 + WAL 兜底；若在线 > 50 人，评估迁移 PostgreSQL（models 层已用 GORM，迁移成本可控） |
| 需求蔓延（加考勤/薪酬） | PRD §3 明确不做，新增需求一律进 PRD 附录 A Backlog 评审排期 |
| 单点部署数据丢失 | backup.sh 每日备份 + 异机拷贝；Docker 部署挂 named volume；M1 出口前演练一次恢复 |
| 金额精度 bug | `pkg/money` 单测全覆盖 + 代码评审 checklist 禁出现 float64 金额 |
| 状态机阶段被随意推进（漏斗注水） | 已引入 SF 式"退出标准"校验（PRD §4.4），主管在二期报表中监控阶段停留时长 |

## 第一条命令（明天就能执行）

```bash
mkdir -p bss/{cmd/server,internal,migrations,web,data} && cd bss && go mod init bss
```
