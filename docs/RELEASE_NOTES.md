# BSS 业务管理系统 · 程序包说明与使用说明

BSS 是面向 10–100 人服务型公司的轻量业务管理系统，打通 **获客 → 成交 → 签约 → 收钱** 完整闭环。

---

## 一、程序包说明

| 项 | 说明 |
| --- | --- |
| 交付形态 | **单二进制文件**（Go embed 已内嵌前端页面与全部数据库迁移脚本），无需安装运行时、无需外部依赖 |
| 包含平台 | Linux amd64 / Linux arm64 / macOS amd64（Intel）/ macOS arm64（Apple Silicon） |
| 数据库 | 内置 SQLite（WAL 模式，纯 Go 驱动），数据落在 `BSS_DATA` 指定的目录 |
| 校验 | 每个文件均提供 SHA256，见发布附件 `checksums.txt` |
| 架构 | 单租户、私有化部署，无外部网络依赖（仅在你配置的 SMTP / 企业微信 webhook 开启时才会外发） |

> 下载后请先核对 `checksums.txt` 中的 SHA256，再赋予可执行权限：
> ```bash
> chmod +x bss-server-linux-amd64
> ```

---

## 二、使用说明（快速开始）

### 1. 运行

```bash
# Linux / macOS 通用
chmod +x bss-server-<平台>-<架构>
BSS_DATA=./data BSS_JWT_SECRET=$(openssl rand -hex 32) ./bss-server-<平台>-<架构>
```

- 默认监听 `127.0.0.1:8080`，浏览器打开 **http://<服务器IP>:8080** 即可使用。
- 首次启动会自动建库、建表，并创建管理员账号。
- 进程前台运行；生产环境建议用 systemd / Docker 托管（见 `DEPLOY.md`）。

### 2. 环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `BSS_ADDR` | `127.0.0.1:8080` | 监听地址。对外服务需改为 `0.0.0.0:8080`（配合反向代理更佳） |
| `BSS_DATA` | `./data` | 数据目录，存放 `bss.db` 与上传附件 `uploads/`。**请挂载持久化卷** |
| `BSS_JWT_SECRET` | 空（运行时随机） | JWT 签名密钥。**生产必须显式设置固定值**，否则每次重启全员掉线 |

生成固定密钥：`openssl rand -hex 32`

### 3. 首次登录

- 账号 `admin@bss.local`，初始密码 `admin123`，登录后强制改密。
- 建议先到「系统配置」建部门，再到「员工」建销售账号（初始密码统一 `Bss@1234`，同样强制首登改密）。

---

## 三、数据备份与恢复

- 备份（复制主库及 WAL 附属文件，保证一致性）：
  ```bash
  ./scripts/backup.sh ./data ./backup
  ```
  建议配合 `crontab` 每日执行；脚本自动保留最近 30 份。
- 恢复：停服后，用备份目录中的 `bss.db` / `bss.db-wal` / `bss.db-shm` 覆盖 `BSS_DATA` 对应文件，再启动。

---

## 四、升级

1. 停服（systemd：`systemctl stop bss`；Docker：`docker compose down`）。
2. 备份 `BSS_DATA` 数据目录（见上）。
3. **仅替换二进制文件 / 更新镜像**，保留 `BSS_DATA` 不动。
4. 启动。迁移脚本会在启动时自动执行，数据向下兼容。

---

## 五、容器部署（简述）

仓库提供 `Dockerfile` 与 `docker-compose.yml`，一行命令即可起服务：

```bash
BSS_JWT_SECRET=$(openssl rand -hex 32) docker compose up -d
```

完整部署、HTTPS 反向代理、systemd 单元、备份策略与故障排查见 **`DEPLOY.md`**。

---

## 六、功能地图

- 客户 / 联系人、商单（状态机 + 加权预测）、合同（N:N 关联 + 附件）、回款计划与记录
- 客户公海池、Excel 批量导入、项目 / 交付管理（任务 / 里程碑 / 人天）
- 审批流、开票管理、报表中心、审计查询
- 提醒（合同到期 / 回款逾期）+ 通知渠道（SMTP 邮件 / 企业微信 webhook）
- 客户查重合并、商单输单分析、银企对账
