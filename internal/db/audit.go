package db

import (
	"encoding/json"
	"strings"
	"time"

	"bss/internal/actor"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	// 软删除触发的是 Delete 回调链（SQL 层面是 UPDATE deleted_at）。
	// 注意：GORM 在 gorm:delete 内部才添加主键 WHERE 条件，Before 时提取不到，
	// 故只在 After 时现场快照（软删行仍在表中，可查到完整 before）
	gdb.Callback().Delete().After("gorm:delete").Register("audit:after_delete", afterDelete)
}

const ctxBeforeKey = "audit:before_json"

func audited(db *gorm.DB) bool {
	return db.Statement.Schema != nil && auditedTables[db.Statement.Schema.Table]
}

func primaryID(db *gorm.DB) uint {
	if db.Statement.Schema != nil {
		if pf := db.Statement.Schema.PrioritizedPrimaryField; pf != nil {
			v, _ := pf.ValueOf(db.Statement.Context, db.Statement.ReflectValue)
			if id, _ := v.(uint); id != 0 {
				return id
			}
		}
	}
	// Model(&X{}).Where("id = ?", n) 写法下主键不在反射值中，回退从 WHERE 子句提取
	if w, ok := db.Statement.Clauses["WHERE"]; ok {
		return idFromExpr(w.Expression)
	}
	return 0
}

// idFromExpr 从 WHERE 表达式递归提取 id 等值条件
// （顶层是 clause.Where 包装，软删除会附加 clause.Eq{deleted_at} 条件）
func idFromExpr(expr clause.Expression) uint {
	switch e := expr.(type) {
	case clause.Where:
		for _, sub := range e.Exprs {
			if id := idFromExpr(sub); id != 0 {
				return id
			}
		}
	case clause.AndConditions:
		for _, sub := range e.Exprs {
			if id := idFromExpr(sub); id != 0 {
				return id
			}
		}
	case clause.Expr:
		if isIDPredicate(e.SQL) && len(e.Vars) == 1 {
			return toUint(e.Vars[0])
		}
	case clause.Eq:
		if col, ok := e.Column.(clause.Column); ok && col.Name == "id" {
			return toUint(e.Value)
		}
	case clause.IN:
		// GORM Delete(model, primaryKey) 生成 IN 条件，且回调阶段列名可能还是 ~~~py~~~ 占位符
		if len(e.Values) == 1 {
			if col, ok := e.Column.(clause.Column); ok && (col.Name == "id" || strings.Contains(col.Name, "~~~")) {
				return toUint(e.Values[0])
			}
		}
	}
	return 0
}

func isIDPredicate(sql string) bool {
	s := strings.ToLower(strings.NewReplacer("`", "", "\"", "").Replace(strings.TrimSpace(sql)))
	return s == "id = ?" || strings.HasSuffix(s, ".id = ?")
}

func toUint(v any) uint {
	switch n := v.(type) {
	case uint:
		return n
	case uint64:
		return uint(n)
	case int:
		return uint(n)
	case int64:
		return uint(n)
	}
	return 0
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
	// Update(column)/Updates(map) 时 Dest 是 map，不含完整行，改为快照当前行保证 after 完整
	afterJSON := toJSON(db.Statement.Dest)
	switch db.Statement.Dest.(type) {
	case map[string]any, *map[string]any:
		afterJSON = snapshotRow(db, db.Statement.Schema.Table, primaryID(db))
	}
	writeLog(db, action, beforeStr, afterJSON)
}

func afterDelete(db *gorm.DB) {
	if !audited(db) || db.Error != nil {
		return
	}
	// gorm:delete 已执行完：WHERE 已含主键条件；软删行仍在表中，快照作为 before
	beforeStr := snapshotRow(db, db.Statement.Schema.Table, primaryID(db))
	writeLog(db, "delete", beforeStr, "")
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
