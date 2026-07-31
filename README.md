# BSS 业务管理系统

面向 10–100 人服务型公司的轻量业务管理系统，打通 **获客 → 成交 → 签约 → 收钱** 完整闭环。

- 技术栈：React 18 + TypeScript + AntD 5（前端）/ Go 1.25 + chi + GORM（后端）/ SQLite（WAL，纯 Go 驱动免 CGO）
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

### 方式三：从 Release 下载预编译二进制（推荐上线用）

打 `v*` 标签会由 GitHub Actions 自动编译 **Linux / macOS（amd64 + arm64）** 四个平台单二进制并发布到 Release，附带 `checksums.txt`、使用说明与部署文档。下载后：

```bash
chmod +x bss-server-linux-amd64
BSS_DATA=./data BSS_JWT_SECRET=$(openssl rand -hex 32) ./bss-server-linux-amd64
```

### 首次登录

- 账号 `admin@bss.local`，初始密码 `admin123`，登录后强制改密
- 建议先到「系统配置」建部门，再去「员工」建销售账号（初始密码统一 `Bss@1234`，同样强制首登改密）

## 部署上线

- **部署指南**：[docs/DEPLOY.md](docs/DEPLOY.md)（裸机 systemd / Docker / Nginx HTTPS / 备份恢复 / 升级）
- **Release 使用说明**：[docs/RELEASE_NOTES.md](docs/RELEASE_NOTES.md)（程序包说明 + 快速开始）
- 仓库含 `Dockerfile`、`docker-compose.yml`、`deploy/nginx.conf`，可一行 `docker compose up -d` 起服务

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

主线与增强全部交付：✅ M0 工程基座 → ✅ M1 一期 MVP（员工/客户/商单/合同/回款/提醒+仪表盘）→ ✅ M2 二期增强（审批/开票/报表/审计/离职交接）→ ✅ M3 三期可选（公海池/Excel 导入/项目交付/通知渠道）→ ✅ M4 业务增强（客户查重合并/输单分析/银企对账）。部署上线收尾（GitHub Release 工作流 + Docker + 部署文档）已完成。详见 [docs/DEV_PLAN.md](docs/DEV_PLAN.md)。
