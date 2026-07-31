#!/usr/bin/env bash
# BSS 端到端回归套件入口
# 流程：构建 server+seed → 用临时数据目录启动实例 → 造种子数据 → 顺序运行 8 个 E2E 脚本 → 汇总 → 清理
# 端口可用环境变量覆盖：BSS_E2E_PORT=8099 ./tests/e2e/run_all.sh
set -u

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

PORT="${BSS_E2E_PORT:-8080}"
ADDR="127.0.0.1:$PORT"
DATADIR="$(mktemp -d /tmp/bss-e2e.XXXXXX)"
BIN="$(mktemp -p /tmp bss-server.XXXXXX)"
SEEDBIN="$(mktemp -p /tmp bss-seed.XXXXXX)"
SRV_PID=""

cleanup() {
  if [ -n "$SRV_PID" ]; then kill "$SRV_PID" 2>/dev/null || true; wait "$SRV_PID" 2>/dev/null || true; fi
  rm -f "$BIN" "$SEEDBIN"
  rm -rf "$DATADIR"
}
trap cleanup EXIT

echo "== 构建 bss-server =="
go build -o "$BIN" ./cmd/server || exit 1
echo "== 构建 seed =="
go build -o "$SEEDBIN" ./cmd/seed || exit 1

echo "== 启动 server (data=$DATADIR addr=$ADDR) =="
BSS_ADDR="$ADDR" BSS_DATA="$DATADIR" "$BIN" > /tmp/bss-e2e-server.log 2>&1 &
SRV_PID=$!
code="000"
for i in $(seq 1 40); do
  code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "http://$ADDR/api/v1/auth/login" \
    -H "Content-Type: application/json" -d '{"email":"admin@bss.local","password":"admin123"}' || echo "000")
  [ "$code" = "200" ] && break
  sleep 1
done
if [ "$code" != "200" ]; then
  echo "server 未就绪"; cat /tmp/bss-e2e-server.log; exit 1
fi
echo "server ready"

echo "== 造数 seed（demo-sales + 演示合同/回款/逾期/提醒）=="
BSS_DATA="$DATADIR" "$SEEDBIN" || { echo "seed 失败"; exit 1; }

# 顺序约束：reports 必须在 m24 之前（m24 会停用 demo-sales）；
# m31 的 offboard 使用自建员工，不污染 m24 对 demo-sales 的审计过滤。
SUITE=(e2e_approvals e2e_contracts e2e_invoices e2e_payments e2e_reminders e2e_m31 e2e_reports e2e_m24)
FAILED=0
for t in "${SUITE[@]}"; do
  echo ""
  echo "===== $t ====="
  if python3 "tests/e2e/$t.py" "http://$ADDR"; then
    echo "PASS  $t"
  else
    echo "FAIL  $t"
    FAILED=$((FAILED+1))
  fi
done

echo ""
if [ "$FAILED" -eq 0 ]; then
  echo "E2E SUITE: ALL PASS"
  exit 0
else
  echo "E2E SUITE: $FAILED 个脚本失败"
  exit 1
fi
