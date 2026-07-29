#!/bin/bash
# BSS 备份脚本：SQLite 在线备份 + 附件打包（TECH_DESIGN §6.9）
# 用法：./scripts/backup.sh [数据目录] [备份目录]
# 建议 crontab 每日执行：0 2 * * * /path/to/scripts/backup.sh /data /backup
set -euo pipefail

DATA_DIR="${1:-./data}"
BACKUP_DIR="${2:-./backup}"
KEEP=30
TS=$(date +%Y%m%d_%H%M%S)
DEST="$BACKUP_DIR/$TS"

mkdir -p "$DEST"

# SQLite 在线备份（WAL 模式下安全）
if command -v sqlite3 >/dev/null 2>&1; then
  sqlite3 "$DATA_DIR/bss.db" ".backup '$DEST/bss.db'"
else
  # 无 sqlite3 CLI 时退化为文件拷贝（服务低峰期可接受；WAL 一并拷贝）
  cp "$DATA_DIR/bss.db" "$DEST/bss.db" 2>/dev/null || true
  cp "$DATA_DIR/bss.db-wal" "$DEST/bss.db-wal" 2>/dev/null || true
  cp "$DATA_DIR/bss.db-shm" "$DEST/bss.db-shm" 2>/dev/null || true
fi

# 附件打包
[ -d "$DATA_DIR/uploads" ] && tar -czf "$DEST/uploads.tar.gz" -C "$DATA_DIR" uploads

# 保留最近 KEEP 份
ls -1dt "$BACKUP_DIR"/*/ 2>/dev/null | tail -n +$((KEEP + 1)) | xargs rm -rf 2>/dev/null || true

echo "备份完成: $DEST"
