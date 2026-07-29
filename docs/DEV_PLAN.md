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

### Sprint 3 · 商单（约 4 天）
- 商单 CRUD、单号 SD-、金额元↔分全链路
- 状态机（对齐 SF 6 态）prospecting→qualifying→proposal→negotiating→won/lost，含回退边与退出标准软校验（422 warning + force 确认）
- probability 按阶段自动带出（10/25/60/80/100/0）可手调；lost 必填输单原因（5 类枚举）
- 加权预测金额 Σ(金额×概率) 的 service 层计算 + 单测（仪表盘呈现在 Sprint 6）
- 客户详情页商单区接入
- **验收**：非法流转被 422 拒绝；回退合法且记审计；加权预测单测通过；lost 必填原因；won 后金额/客户只读

### Sprint 4 · 合同 + 附件（约 6 天）
- 合同 CRUD、单号 HT-；**deal_contracts N:N 关联（0..N 个 won 商单多选，且必须与合同同客户）+ PUT /contracts/:id/deals**
- 状态机 draft→pending→signed→performing→completed，回退 pending→draft，旁路 cancelled（签约前）/terminated（签约后必填原因）/expired（人工标记）
- signed 后金额/客户/关联商单只读（editableFields 判定）
- 附件上传/下载（鉴权、类型白名单、20MB）
- **验收**：仅 won 商单可被关联；跨客户商单关联被 422 拒绝；多商单合并签一份合同可建；pending 可退回 draft 并记审计；附件非登录不可下载；terminated 必填原因

### Sprint 5 · 回款（约 5 天）
- 回款计划 CRUD（总额 ≤ 合同额校验）；**已核销计划禁改金额/禁删，只能新增调整期**
- 回款记录录入/核销、计划状态自动推进 pending→partial→paid
- 逾期派生查询（is_overdue）+ 合同维度汇总（应收/已收/余额/逾期额）
- **验收**：核销金额准确到分（边界用例：多退少补、跨计划核销）；已核销计划改删被 422 拒绝；财务角色才能录入

### Sprint 6 · 提醒 + 仪表盘 + 联调（约 6 天）
- cron 每日 09:00 扫描：合同 30 天内到期、回款逾期 → notifications（dedup_key 去重）
- 站内通知列表/未读角标/标记已读
- 仪表盘：4 张卡片 + 3 个列表（数据范围按角色过滤）
- 全链路联调 + 造数脚本 + PRD §4 逐条验收
- **验收（M1 出口）**：从建客户 → 商单赢单 → 签合同 → 回款计划 → 到账核销 → 逾期提醒，全链路演示通过；备份脚本可用

---

## M2 · 二期增强（约 3–4 周）

1. **审批流**（约 1.5 周）：合同签约审批、商单折扣审批；新增 approvals 表，状态机插入 `pending_approval` 节点（状态机预留扩展位此时兑现）
2. **开票管理**（约 1 周）：invoices 表、开票与合同/回款关联、开票状态
3. **报表中心**（约 1 周）：月度签约/回款趋势、销售排行、客户转化漏斗；CSV 导出
4. **审计查询页 + 离职交接**（约 0.5 周）：audit_logs 按实体查询；员工停用前批量转移名下数据

## M3 · 三期可选（按业务反馈）

- 项目/交付管理（若"做"的过程需要管：任务、里程碑、人天）
- 通知渠道扩展：邮件 → 企业微信 webhook
- 客户公海池、Excel 数据导入（评审确认一期不做，已入 PRD 附录 A Backlog）

> 完整候选清单统一维护在 **PRD 附录 A · Backlog**，此处不重复登记。

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
