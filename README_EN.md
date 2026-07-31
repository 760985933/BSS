# BSS — Business Management System

A lightweight business management system for service-oriented teams of 10–100 people, covering the full **lead → deal → contract → payment** lifecycle.

- **Stack**: React 18 + TypeScript + AntD 5 (frontend) / Go 1.25 + chi + GORM (backend) / SQLite (WAL, pure-Go driver, no CGO)
- **Delivery**: single binary (Go embed bundles the frontend), deploy via Docker or bare-metal, single-tenant
- **Docs**: [docs/PRD.md](docs/PRD.md) (PRD), [docs/TECH_DESIGN.md](docs/TECH_DESIGN.md) (tech design), [docs/DEV_PLAN.md](docs/DEV_PLAN.md) (dev plan)

> 📖 中文文档 / Chinese README: [README.md](README.md)

## Quick Start

### Option 1: Dev mode (separate frontend/backend, hot reload)

```bash
# Terminal 1 — backend (:8080, data dir ./data, auto-migrate on first run)
make dev-server

# Terminal 2 — frontend (Vite dev server :5173, /api proxied to 8080)
make dev-web
```
Open **http://localhost:5173**.

### Option 2: Single binary (verify delivery shape)

```bash
make run   # build frontend dist + Go embed compile + run
```
Open **http://127.0.0.1:8080**.

### Option 3: Prebuilt binaries from Release (recommended for production)

Pushing a `v*` tag triggers GitHub Actions to build single binaries for **Linux / macOS (amd64 + arm64)** and publish them to Releases, along with `checksums.txt`, usage notes, and deployment docs. After downloading:

```bash
chmod +x bss-server-linux-amd64
BSS_DATA=./data BSS_JWT_SECRET=$(openssl rand -hex 32) ./bss-server-linux-amd64
```

### First Login

- Account `admin@bss.local`, initial password `admin123`; password change is forced on first login.
- Recommended: create departments under "系统配置" (System Config), then create sales accounts under "员工" (Employees) — initial password `Bss@1234`, also forced to change on first login.

## Deploy

- **Deployment guide**: [docs/DEPLOY.md](docs/DEPLOY.md) (systemd / Docker / Nginx HTTPS / backup-restore / upgrade)
- **Release notes**: [docs/RELEASE_NOTES.md](docs/RELEASE_NOTES.md)
- The repo ships `Dockerfile`, `docker-compose.yml`, `deploy/nginx.conf`; start with `docker compose up -d`.

## Notes

- **Frontend uses pnpm only**: `cd web && pnpm install` (npm is known to hang behind the proxy; `pnpm-lock.yaml` is committed).
- Env vars: `BSS_ADDR` (listen addr, default 127.0.0.1:8080), `BSS_DATA` (data dir, default ./data), `BSS_JWT_SECRET` (**must be set in production**, otherwise all sessions are lost on restart).
- Tests: `make test`; Backup: `./scripts/backup.sh ./data ./backup` (recommended via crontab daily).

## Project Layout

```
cmd/server/     # backend entrypoint
internal/       # config / db / models / handlers / services / middleware / pkg
migrations/     # goose SQL migrations (incremental, auto-run on start)
web/src/        # frontend (pages / layouts / api / auth)
scripts/        # backup.sh
data/           # runtime: bss.db + uploads/ (gitignored)
docs/           # PRD / tech design / dev plan
```

## Status

All milestones delivered: ✅ M0 foundation → ✅ M1 MVP (employees / customers / deals / contracts / payments / reminders + dashboard) → ✅ M2 (approval / invoicing / reports / audit / offboarding) → ✅ M3 (customer pool / Excel import / project delivery / notify channels) → ✅ M4 (dedup merge / lost-deal analysis / bank reconciliation). Deployment hardening (GitHub Release workflow + Docker + docs) done. See [docs/DEV_PLAN.md](docs/DEV_PLAN.md).

## License

Licensed under the [Apache License 2.0](LICENSE).
