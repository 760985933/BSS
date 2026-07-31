# 安全政策 / Security Policy

## 支持的版本 / Supported Versions

| 版本 / Version | 安全更新 / Security Fixes |
|---|---|
| v1.0.x（最新） / latest | ✅ 提供支持 |
| 更早版本 / older | ❌ 不再维护 |

## 报告漏洞 / Reporting a Vulnerability

⚠️ **请勿在公开 Issue 中披露安全漏洞。** 请通过以下方式**私下**报告：

- **推荐**：在仓库的 **Security → Report a vulnerability** 创建私有安全公告（GitHub Security Advisories）；
- 邮件 / Email：`maintainer@example.com`（占位，请替换为项目维护者真实邮箱）。

报告中请尽量提供：

- 漏洞类型与影响范围；
- 复现步骤（PoC）；
- 建议的修复方案（如有）。

## 响应时效 / Response Timeline

- 确认收到：≤ 3 个工作日；
- 初步评估与修复排期：≤ 7 个工作日；
- 修复随版本发布后，经你同意可公开致谢。

## 安全加固建议 / Hardening Notes

- 生产环境**必须**显式设置 `BSS_JWT_SECRET`，否则服务重启会导致全员会话失效；
- 数据库与上传目录（`data/`）应定期备份并限制文件权限；
- 建议通过 Nginx 等反向代理启用 HTTPS，避免明文传输凭据。
