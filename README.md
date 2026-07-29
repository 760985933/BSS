# BSS 业务管理系统

面向 10–100 人服务型公司的轻量业务管理系统，打通 **获客 → 成交 → 签约 → 收钱** 完整闭环。

- 技术栈：React 18 + TypeScript + AntD 5（前端）/ Go 1.23 + chi + GORM（后端）/ SQLite（WAL，纯 Go 驱动免 CGO）
- 交付形态：单二进制（Go embed 内嵌前端），Docker 或裸服务器私有化部署，单租户
- 设计文档：[docs/PRD.md](docs/PRD.md)（产品需求）、[docs/TECH_DESIGN.md](docs/TECH_DESIGN.md)（技术方案）、[docs/DEV_PLAN.md](docs/DEV_PLAN.md)（开发计划）

## 快速开始

### 方式一：开发模式（前后端分离，热更新）

```bash
# 终端 1 —— 后端（127.0.0.1:8080，数据目录 ./data，首启自动建库建表）
make dev-server

# 终端 2 —— 前端（Vite 开发服务器 :5173，/api 自动代理到 8080）
make dev-web
```
浏览器打开 **http://localhost:5173**。

### 方式二：单二进制模式（验证交付形态）

```bash
make run   # = 构建前端 dist + Go embed 编译 + 启动
```
浏览器打开 **http://127.0.0.1:8080**，API 与页面同源。

### 首次登录

- 账号 `admin@bss.local`，初始密码 `admin123`，登录后强制改密
- 建议先到「系统配置」建部门，再去「员工」建销售账号（初始密码统一 `Bss@1234`，同样强制首登改密）

## 注意事项

- **前端依赖只认 pnpm**：`cd web && pnpm install`（本机 npm 会被代理卡死；`pnpm-lock.yaml` 已入库）
- 环境变量：`BSS_ADDR`（监听地址，默认 127.0.0.1:8080）、`BSS_DATA`（数据目录，默认 ./data）、`BSS_JWT_SECRET`（**生产必须显式设置**，否则每次重启全员掉线）
- 测试：`make test`；备份：`./scripts/backup.sh ./data ./backup`（建议 crontab 每日执行）

## 目录结构

```
cmd/server/     # 后端入口
internal/       # config / db / models / handlers / services / middleware / pkg
migrations/     # goose SQL migration（增量，启动自动执行）
web/src/        # 前端（pages / layouts / api / auth）
scripts/        # backup.sh
data/           # 运行时生成：bss.db + uploads/（已 gitignore）
docs/           # PRD / 技术方案 / 开发计划
```

## 当前进度

M1 一期 MVP 进行中：✅ M0 工程基座 → ✅ Sprint 1 员工档案 → ✅ Sprint 2 客户+联系人 → ✅ Sprint 3 商单 → ⏳ Sprint 4 合同+附件 → Sprint 5 回款 → Sprint 6 提醒+仪表盘。详见 [docs/DEV_PLAN.md](docs/DEV_PLAN.md)。
