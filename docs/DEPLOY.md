# BSS 部署指南（上线收尾）

本文覆盖三种部署形态与上线必做项。单二进制已内嵌前端与迁移，**零外部依赖**，任选其一即可。

- 方式一：裸机单二进制（systemd 托管）— 最轻量
- 方式二：Docker / Docker Compose — 推荐，环境隔离
- HTTPS 反向代理（Nginx）— 生产必做
- 备份 / 恢复 / 升级 / 安全建议

---

## 方式一：裸机单二进制 + systemd

1. 从 Release 下载对应平台二进制，放到 `/opt/bss/`：
   ```bash
   mkdir -p /opt/bss/data
   cp bss-server-linux-amd64 /opt/bss/bss-server
   chmod +x /opt/bss/bss-server
   ```
2. 创建专用用户与数据目录：
   ```bash
   useradd -r -s /usr/sbin/nologin bss
   mkdir -p /var/lib/bss && chown -R bss:bss /var/lib/bss
   ```
3. 写入 `/etc/systemd/system/bss.service`：
   ```ini
   [Unit]
   Description=BSS Business Management Server
   After=network.target

   [Service]
   User=bss
   Group=bss
   WorkingDirectory=/opt/bss
   Environment=BSS_ADDR=127.0.0.1:8080
   Environment=BSS_DATA=/var/lib/bss
   Environment=BSS_JWT_SECRET=__此处填 openssl rand -hex 32 生成的固定值__
   ExecStart=/opt/bss/bss-server
   Restart=on-failure
   RestartSec=5

   [Install]
   WantedBy=multi-user.target
   ```
4. 启停：
   ```bash
   systemctl daemon-reload
   systemctl enable --now bss
   systemctl status bss
   ```
   访问 `http://<服务器IP>:8080`。

---

## 方式二：Docker / Docker Compose

仓库根目录已提供 `Dockerfile` 与 `docker-compose.yml`。

```bash
# 生成固定 JWT 密钥
export BSS_JWT_SECRET=$(openssl rand -hex 32)

# 构建并后台启动
docker compose up -d --build

# 查看日志
docker compose logs -f bss
```

`docker-compose.yml` 已将数据目录挂到命名卷 `bss-data`，重启不丢数据。
如需自定义端口 / 密钥，编辑 `docker-compose.yml` 的 `environment` 与 `ports` 即可。

构建镜像（可选，推送到私有仓库）：
```bash
docker build -t your-registry/bss:latest .
```

---

## HTTPS 反向代理（Nginx，生产必做）

不要让 Go 直接对外暴露 8080。典型做法：BSS 监听内网 `127.0.0.1:8080`，Nginx 终止 TLS 并反代。

示例 `deploy/nginx.conf`（放入 `/etc/nginx/conf.d/bss.conf` 或 `sites-enabled/`）：

```nginx
server {
    listen 80;
    server_name bss.example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    server_name bss.example.com;

    ssl_certificate     /etc/nginx/certs/fullchain.pem;
    ssl_certificate_key /etc/nginx/certs/privkey.pem;

    client_max_body_size 20m;   # 合同附件上限 20MB

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

若用 Docker 部署，可把上面的 Nginx 加进同一 compose（见 `docker-compose.yml` 注释示例），或将 `ports` 改为仅暴露给 Nginx 网络。

---

## 数据备份与恢复

- **备份**（仓库自带 `scripts/backup.sh`，复制主库 + WAL 附属文件保证一致性）：
  ```bash
  ./scripts/backup.sh /var/lib/bss /var/backups/bss
  ```
  建议 `crontab -e` 加入每日凌晨执行：
  ```cron
  0 3 * * * /opt/bss/scripts/backup.sh /var/lib/bss /var/backups/bss >> /var/log/bss-backup.log 2>&1
  ```
- **恢复**：停服 → 用备份中的 `bss.db` / `bss.db-wal` / `bss.db-shm` 覆盖 `BSS_DATA` 对应文件 → 启动。

---

## 升级流程

1. 停服：`systemctl stop bss` 或 `docker compose down`。
2. 备份 `BSS_DATA`（见上）。
3. **仅替换二进制 / 更新镜像**，保留 `BSS_DATA` 不动。
4. 启动。迁移脚本在启动时自动执行，数据向下兼容。

---

## 安全建议

- **必须设置固定 `BSS_JWT_SECRET`**，否则重启全员掉线。
- 生产务必走 HTTPS（反向代理终止 TLS），避免明文传输密码与 JWT。
- 用防火墙 / 安全组限制 8080 仅允许反向代理或内网访问。
- 数据库与上传附件都在 `BSS_DATA`，请做定期离线备份。

---

## 故障排查

| 现象 | 排查 |
| --- | --- |
| 启动报「数据库初始化失败」 | 检查 `BSS_DATA` 目录是否存在、是否可写 |
| 重启后所有用户被迫重新登录 | 未设置 `BSS_JWT_SECRET`，每次随机生成；设置固定值 |
| 页面空白 / 资源 404 | 使用的不是 Release 二进制（前端未内嵌）；请用官方发布包 |
| 上传附件失败 | 检查 `BSS_DATA/uploads` 写权限与反向代理 `client_max_body_size` |
| 通知收不到 | 「通知渠道」中 SMTP / 企业微信未开启或配置错误，查看「通知日志」 |
