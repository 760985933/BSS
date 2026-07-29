package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bss "bss"

	"github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Open 打开 SQLite 并执行全部 PRAGMA 与 migration。
// SQLite 单写入者约束：连接池固定 MaxOpenConns=1（写串行化），20 人在线量级足够。
func Open(dataDir string) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Join(dataDir, "uploads"), 0o755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(on)",
		filepath.Join(dataDir, "bss.db"))

	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
		// PRD §5：服务端统一 UTC 存储
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)

	if err := Migrate(sqlDB); err != nil {
		return nil, err
	}
	RegisterAuditCallbacks(gdb)
	return gdb, nil
}

// Migrate 执行 goose migration（embed 内嵌 SQL，可重复执行，增量生效）
func Migrate(sqlDB *sql.DB) error {
	goose.SetBaseFS(bss.MigrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}
	if err := goose.Up(sqlDB, "migrations"); err != nil {
		return fmt.Errorf("migration 执行失败: %w", err)
	}
	return nil
}
