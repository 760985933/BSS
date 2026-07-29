package db

import (
	"encoding/json"
	"time"

	"bss/internal/actor"

	"gorm.io/gorm"
)

// auditedTables 审计白名单（PRD §5：合同、回款、客户/商单的关键变更）
var auditedTables = map[string]bool{
	"customers":       true,
	"deals":           true,
	"contracts":       true,
	"payment_plans":   true,
	"payment_records": true,
	"employees":       true,
}

// RegisterAuditCallbacks 注册 GORM 审计钩子（M0 骨架）。
// create/update 全覆盖；软删除本质是 deleted_at 的 update，在 after_update 中识别为 action=delete。
// 关键约束：连接池=1 且 GORM 单条写默认包事务，回调内禁止另开 GORM 会话（会等锁超时），
// 一律通过 db.Statement.ConnPool（当前事务/连接）执行原生 SQL。
func RegisterAuditCallbacks(gdb *gorm.DB) {
	gdb.Callback().Create().After("gorm:create").Register("audit:after_create", afterCreate)
	gdb.Callback().Update().Before("gorm:update").Register("audit:before_update", beforeUpdate)
	gdb.Callback().Update().After("gorm:update").Register("audit:after_update", afterUpdate)
}

const ctxBeforeKey = "audit:before_json"

func audited(db *gorm.DB) bool {
	return db.Statement.Schema != nil && auditedTables[db.Statement.Schema.Table]
}

func primaryID(db *gorm.DB) uint {
	if db.Statement.Schema == nil {
		return 0
	}
	pf := db.Statement.Schema.PrioritizedPrimaryField
	if pf == nil {
		return 0
	}
	v, _ := pf.ValueOf(db.Statement.Context, db.Statement.ReflectValue)
	id, _ := v.(uint)
	return id
}

func afterCreate(db *gorm.DB) {
	if !audited(db) || db.Error != nil {
		return
	}
	writeLog(db, "create", "", toJSON(db.Statement.Dest))
}

func beforeUpdate(db *gorm.DB) {
	if !audited(db) {
		return
	}
	db.Statement.Set(ctxBeforeKey, snapshotRow(db, db.Statement.Schema.Table, primaryID(db)))
}

func afterUpdate(db *gorm.DB) {
	if !audited(db) || db.Error != nil {
		return
	}
	before, _ := db.Statement.Get(ctxBeforeKey)
	beforeStr, _ := before.(string)

	// 识别软删除：deleted_at 由 null 变为非 null
	action := "update"
	if m, ok := db.Statement.Dest.(interface{ GetDeletedAtValid() bool }); ok && m.GetDeletedAtValid() {
		action = "delete"
	}
	writeLog(db, action, beforeStr, toJSON(db.Statement.Dest))
}

// snapshotRow 在当前连接上读取行旧值（before_json）
func snapshotRow(db *gorm.DB, table string, id uint) string {
	if !auditedTables[table] || id == 0 { // 表名仅取白名单，无注入风险
		return ""
	}
	rows, err := db.Statement.ConnPool.QueryContext(db.Statement.Context,
		"SELECT * FROM "+table+" WHERE id = ?", id)
	if err != nil {
		return ""
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil || !rows.Next() {
		return ""
	}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return ""
	}
	m := make(map[string]any, len(cols))
	for i, c := range cols {
		m[c] = vals[i]
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func writeLog(db *gorm.DB, action, beforeJSON, afterJSON string) {
	_, err := db.Statement.ConnPool.ExecContext(db.Statement.Context,
		`INSERT INTO audit_logs (entity_type, entity_id, action, operator_id, before_json, after_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		db.Statement.Schema.Table, primaryID(db), action,
		actor.From(db.Statement.Context), beforeJSON, afterJSON,
		time.Now().UTC())
	if err != nil {
		_ = db.AddError(err)
	}
}

func toJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
