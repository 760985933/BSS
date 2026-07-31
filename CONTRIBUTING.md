# 贡献指南 / Contributing

感谢你考虑为 **BSS 业务管理系统** 做出贡献！本文件说明如何参与开发、提交代码与代码规范。

## 如何参与 / How to Contribute

- **报告问题 / Report issues**：在 [Issues](../../issues) 中描述复现步骤、期望与实际表现、环境信息（系统 / 浏览器 / 版本）。
- **提交代码 / Submit code**：
  1. Fork 本仓库并创建特性分支（建议 `feat/xxx` 或 `fix/xxx`）；
  2. 本地开发与自测；
  3. 向 `main` 提交 Pull Request，描述变更动机与影响范围。

## 开发环境 / Development Setup

- 后端：Go 1.25+，`make dev-server` 启动（数据目录 `./data`，首启自动建库建表与迁移）。
- 前端：**仅使用 pnpm**，不要用 npm**。`cd web && pnpm install && pnpm dev`（Vite 开发服务器 `:5173`，`/api` 自动代理到后端 `:8080`）。

## 代码规范 / Code Standards

- 后端合并前提：`go test ./...` 全绿；前端合并前提：`pnpm test` 全绿。
- 提交信息建议遵循 [Conventional Commits](https://www.conventionalcommits.org/)：`feat:` / `fix:` / `docs:` / `chore:` / `test:` 等。
- 金额一律使用整数分（见 `internal/pkg/money`），禁止浮点金额参与计算。
- 状态机、RBAC、行级数据范围等**正确性敏感**改动，必须附带单测或 E2E，否则不予合并。
- 保持 `gofmt` / `pnpm lint` 干净。

## 行为准则 / Code of Conduct

请友好、专业地交流，尊重所有贡献者。

---

Thanks for contributing! Please file issues with reproduction steps, and open PRs against `main` with clear descriptions. Keep `go test ./...` and `pnpm test` green before requesting review.
