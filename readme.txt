两种方式，都已封装在 Makefile 里：

## 方式一：开发模式（前后端分离，改代码热更新）

```bash
# 终端 1 —— 后端（127.0.0.1:8080，数据目录 ./data，首启自动建库建表）
make dev-server

# 终端 2 —— 前端（Vite 开发服务器 :5173，/api 自动代理到 8080）
make dev-web
```
浏览器打开 **http://localhost:5173**。

## 方式二：单二进制模式（验证交付形态，embed 前端）

```bash
make run   # = 构建前端 dist + Go embed 编译 + 启动
```
浏览器打开 **http://127.0.0.1:8080**，API 和页面同源。

## 首次登录

- 账号 `admin@bss.local`，初始密码 `admin123`，登录后强制改密
- 建议登录后先到「系统配置」建部门，再去「员工」建销售账号（初始密码统一 `Bss@1234`，同样强制首登改密）

## 注意事项

- **前端依赖安装/重装只认 pnpm**：`cd web && pnpm install`（本机 npm 会被代理卡死，`pnpm-lock.yaml` 已入库）
- 环境变量：`BSS_ADDR`（监听地址）、`BSS_DATA`（数据目录）、`BSS_JWT_SECRET`（**生产必须显式设置**，否则每次重启全员掉线）
- 跑测试：`make test`；备份：`./scripts/backup.sh ./data ./backup`
- 常用命令忘了就 `grep -A2 '^[a-z-]*:' Makefile` 或直接看 Makefile 顶部注释