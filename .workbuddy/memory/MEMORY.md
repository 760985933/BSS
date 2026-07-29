# BSS 项目长期记忆

## 项目概况
- 定位：服务型小团队业务管理系统（客户/商单/合同/回款/员工），React 18 + Go 1.23 + SQLite，Docker/裸机私有化部署，单租户。
- 文档：docs/PRD.md、docs/TECH_DESIGN.md、docs/DEV_PLAN.md（均 v1.2，决策已冻结）。

## 已冻结的关键决策
- 商单↔合同 = N:N（deal_contracts 中间表，关联须同客户、仅 won 状态）。
- 商单 6 态（prospecting→qualifying→proposal→negotiating→won/lost）+ probability（10/25/60/80/100/0）；合同主线 5 态 + cancelled/terminated/expired，approving 二期预留；未关闭可回退、终态不可逆且字段锁定（editableFields 白名单）。
- 金额 INTEGER 分（*_cent），禁浮点；逾期派生不持久化；单号 PREFIX-YYYY-#### 与主键分离（code_counters UPSERT RETURNING）；UTC 存储；软删除+审计白名单表。

## 技术栈定稿（M0 实践验证）
- 后端：chi v5 + GORM + glebarez/sqlite（纯 Go 免 CGO）+ goose/v3（embed migrations）+ golang-jwt/v5 + bcrypt。goose 用 "sqlite3" dialect。
- 前端：pnpm（不是 npm！）+ AntD 5 + TanStack Query + React Router 6。
- 交付：Go embed web/dist → 单二进制 bss-server；assets.go 在模块根承载 embed。

## 踩坑记录（重要，勿再犯）
1. **GORM 回调死锁**：连接池=1 时，GORM 回调内 `db.Session()` 另开会话会等锁超时。审计回调必须用 `db.Statement.ConnPool` 执行原生 SQL。
2. **glebarez 时间列**：DDL 时间列必须声明 DATETIME（不是 TEXT），否则 driver 按 string 返回，Scan 到 time.Time 报错。
3. **UTC 存储**：GORM 默认 time.Now() 本地时区，必须 `gorm.Config{NowFunc: time.Now().UTC}`。
4. **npm 卡死**：本机 HTTP_PROXY(127.0.0.1:58609) 让 npm 安装挂起无产物；网络到 npm registry 极慢。用 `pnpm install`（有全局 store），且 package.json 需配 `pnpm.onlyBuiltDependencies: ["esbuild"]` 否则 vite build 失败。
5. **embed 限制**：Go embed 不能引用上级目录，migrations 和 web/dist 都由根目录 assets.go embed。
6. **Edit 工具**：本会话写入的文件，跨会话 Edit 前必须先 Read。
7. **GORM WHERE 子句结构**（审计提取主键三轮才修好，audit_test.go 已钉死）：`Clauses["WHERE"].Expression` 顶层是 `clause.Where` 包装；用户 Where("id=?") 产生 `clause.Expr`（带反引号）；软删除附加 `clause.Eq{deleted_at}`；`Delete(model, pk)` 的主键条件是 `clause.IN` 且回调阶段列名是 `~~~py~~~` 占位符（Build 时才替换）。
8. **软删除回调时序**：GORM 软删除走 **Delete 回调链**（不是 Update）；且主键 WHERE 条件在 gorm:delete 内部才添加——Before("gorm:delete") 提取不到主键，只能 After 时快照（软删行仍在表中可查）。
9. **Update(column)/Updates(map)**：Dest 是 map 不含完整行，审计 after_json 需改为快照当前行。

## 工程环境
- Go 1.23.5（toolchain 自动升 1.25）；Node 用 managed：/Users/tianfeng/.workbuddy/binaries/node/versions/22.22.2/bin/（Makefile 已写死 NODE/NPM 路径，但安装用 pnpm）。
- Go 代理：GOPROXY=https://goproxy.cn,direct 流畅。
- 启动：make dev-server（8080）/ make run；首启 admin@bss.local / admin123 强制改密。
- 测试服务曾用 127.0.0.1:18099 + /tmp/bss-data。

## 进度
- M0 工程基座：✅ 2026-07-29 完成并端到端验证。
- M1 Sprint 1 员工档案：✅ 同日完成（commit bb25205，已 push）。
- M1 Sprint 2 客户+联系人：✅ 同日完成（1 万条性能 16-34ms）。
- M1 Sprint 3 商单：✅ 2026-07-30 完成（状态机/软校验/加权预测单测 5 项）。
- 下一步：M1 Sprint 4 合同+附件（N:N 关联同客户校验、状态机+回退、signed 锁定、附件 20MB 白名单）。
