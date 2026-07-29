package db

import (
	"encoding/json"
	"testing"

	"bss/internal/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupAuditDB(t *testing.T) *gorm.DB {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if sqlDB, err := gdb.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	for _, ddl := range []string{
		`CREATE TABLE customers (id INTEGER PRIMARY KEY AUTOINCREMENT, code TEXT, name TEXT, industry TEXT DEFAULT '', source TEXT DEFAULT '', level TEXT DEFAULT '', owner_id INTEGER, remark TEXT DEFAULT '', created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE audit_logs (id INTEGER PRIMARY KEY AUTOINCREMENT, entity_type TEXT, entity_id INTEGER, action TEXT, operator_id INTEGER DEFAULT 0, before_json TEXT DEFAULT '', after_json TEXT DEFAULT '', created_at DATETIME)`,
	} {
		if err := gdb.Exec(ddl).Error; err != nil {
			t.Fatal(err)
		}
	}
	RegisterAuditCallbacks(gdb)
	return gdb
}

func lastAudit(t *testing.T, gdb *gorm.DB) models.AuditLog {
	t.Helper()
	var log models.AuditLog
	if err := gdb.Table("audit_logs").Order("id DESC").Take(&log).Error; err != nil {
		t.Fatal(err)
	}
	return log
}

// Update(column) 写法：Model(&T{}).Where("id = ?")，entity_id 与 before/after 必须完整
func TestAuditUpdateColumnStyle(t *testing.T) {
	gdb := setupAuditDB(t)
	c := models.Customer{Code: "KH-1", Name: "测试客户", OwnerID: 1}
	if err := gdb.Create(&c).Error; err != nil {
		t.Fatal(err)
	}

	// 单字段 Update（Transfer 同款写法）
	if err := gdb.Model(&models.Customer{}).Where("id = ?", c.ID).Update("owner_id", 9).Error; err != nil {
		t.Fatal(err)
	}

	log := lastAudit(t, gdb)
	if log.EntityID != c.ID {
		t.Errorf("entity_id = %d，期望 %d（WHERE 提取失败）", log.EntityID, c.ID)
	}
	if log.Action != "update" {
		t.Errorf("action = %s，期望 update", log.Action)
	}
	var before, after map[string]any
	if err := json.Unmarshal([]byte(log.BeforeJSON), &before); err != nil {
		t.Fatalf("before_json 非法: %v（原值 %q）", err, log.BeforeJSON)
	}
	if err := json.Unmarshal([]byte(log.AfterJSON), &after); err != nil {
		t.Fatalf("after_json 非法: %v（原值 %q）", err, log.AfterJSON)
	}
	if before["name"] != "测试客户" {
		t.Errorf("before 不含完整行: %v", before)
	}
	if after["name"] != "测试客户" {
		t.Errorf("after 不含完整行: %v", after)
	}
}

// Updates(map) 写法（Updates 同款）
func TestAuditUpdatesMapStyle(t *testing.T) {
	gdb := setupAuditDB(t)
	c := models.Customer{Code: "KH-2", Name: "地图更新", OwnerID: 1}
	if err := gdb.Create(&c).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Model(&models.Customer{}).Where("id = ?", c.ID).Updates(map[string]any{"level": "A"}).Error; err != nil {
		t.Fatal(err)
	}
	log := lastAudit(t, gdb)
	if log.EntityID != c.ID {
		t.Errorf("entity_id = %d，期望 %d", log.EntityID, c.ID)
	}
	var after map[string]any
	if err := json.Unmarshal([]byte(log.AfterJSON), &after); err != nil || after["level"] != "A" {
		t.Errorf("after 异常: %v %q", err, log.AfterJSON)
	}
}

// 软删除应被识别为 action=delete 且 entity_id 正确
func TestAuditSoftDelete(t *testing.T) {
	gdb := setupAuditDB(t)
	c := models.Customer{Code: "KH-3", Name: "待删除", OwnerID: 1}
	if err := gdb.Create(&c).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Delete(&models.Customer{}, c.ID).Error; err != nil {
		t.Fatal(err)
	}
	log := lastAudit(t, gdb)
	if log.Action != "delete" {
		t.Errorf("action = %s，期望 delete", log.Action)
	}
	if log.EntityID != c.ID {
		t.Errorf("entity_id = %d，期望 %d", log.EntityID, c.ID)
	}
}
