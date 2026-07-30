#!/usr/bin/env bash
# BSS SQLite 备份脚本
# 用法：
#   ./scripts/backup.sh [数据目录] [备份根目录]
# 环境变量：BSS_DATA（默认 ./data），BACKUP_DIR（默认 <数据目录>/backups）
#
# 说明：SQLite 使用 WAL 模式，备份时一并复制 -wal / -shm 文件以保证一致性。
# 建议：在服务器停止或低峰期执行；如需在线热备可改用 `sqlite3 bss.db ".backup"`.

set -euo pipefail

DATA_DIR="${1:-${BSS_DATA:-./data}}"
BACKUP_ROOT="${2:-${BACKUP_DIR:-$DATA_DIR/backups}}"

DB="$DATA_DIR/bss.db"
if [[ ! -f "$DB" ]]; then
  echo "错误：找不到数据库文件 $DB" >&2
  exit 1
fi

TS="$(date +%Y%m%d-%H%M%S)"
DEST="$BACKUP_ROOT/$TS"
mkdir -p "$DEST"

# 复制主库及 WAL 附属文件（若存在）
cp -p "$DB" "$DEST/bss.db"
for ext in -wal -shm; do
  if [[ -f "${DB}${ext}" ]]; then
    cp -p "${DB}${ext}" "$DEST/bss.db${ext}"
  fi
done

# 保留最近 30 份
if command -v ls >/dev/null 2>&1; then
  ls -1dt "$BACKUP_ROOT"/*/ 2>/dev/null | tail -n +31 | xargs -r rm -rf
fi

echo "备份完成：$DEST"
echo "当前备份列表："
ls -1dt "$BACKUP_ROOT"/*/ 2>/dev/null | head
