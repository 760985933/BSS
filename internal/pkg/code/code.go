// Package code 业务单号生成（TECH_DESIGN §6.4：事务内原子递增，防并发重号）
package code

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// 单号前缀（PRD §5）
const (
	PrefixCustomer = "KH"
	PrefixDeal     = "SD"
	PrefixContract = "HT"
)

// Generator 基于 code_counters 表的单号生成器
type Generator struct {
	db *gorm.DB
}

func NewGenerator(db *gorm.DB) *Generator {
	return &Generator{db: db}
}

// Next 生成下一个单号，格式 PREFIX-YYYY-####（年度内递增，允许断号）。
// 利用写事务 + UPSERT RETURNING 保证并发安全（SQLite 写串行化兜底）。
func (g *Generator) Next(ctx context.Context, prefix string) (string, error) {
	year := time.Now().Year()
	var seq int
	err := g.db.WithContext(ctx).Raw(
		`INSERT INTO code_counters (prefix, year, seq) VALUES (?, ?, 1)
		 ON CONFLICT(prefix, year) DO UPDATE SET seq = seq + 1
		 RETURNING seq`, prefix, year).Scan(&seq).Error
	if err != nil {
		return "", fmt.Errorf("生成单号失败(%s): %w", prefix, err)
	}
	return Format(prefix, year, seq), nil
}

// Format 单号格式化（导出便于测试与人工校验）
func Format(prefix string, year, seq int) string {
	return fmt.Sprintf("%s-%d-%04d", prefix, year, seq)
}
