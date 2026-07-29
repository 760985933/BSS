package code

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_pragma=busy_timeout(5000)"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	// 与生产一致：单连接池写串行化，验证并发取号在该约束下依然安全
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS code_counters (
		prefix TEXT NOT NULL, year INTEGER NOT NULL, seq INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (prefix, year))`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func TestNextSequence(t *testing.T) {
	db := setupTestDB(t)
	g := NewGenerator(db)
	ctx := context.Background()
	year := time.Now().Year()

	c1, err := g.Next(ctx, PrefixCustomer)
	if err != nil {
		t.Fatal(err)
	}
	c2, _ := g.Next(ctx, PrefixCustomer)
	c3, _ := g.Next(ctx, PrefixCustomer)

	if c1 != Format(PrefixCustomer, year, 1) || c2 != Format(PrefixCustomer, year, 2) || c3 != Format(PrefixCustomer, year, 3) {
		t.Errorf("序列不连续: %s %s %s", c1, c2, c3)
	}

	// 不同前缀互不影响
	d1, _ := g.Next(ctx, PrefixDeal)
	if d1 != Format(PrefixDeal, year, 1) {
		t.Errorf("不同前缀应独立计数，得到 %s", d1)
	}
}

// 并发取号不允许重复（SQLite 写串行化 + UPSERT 原子性）
func TestNextConcurrent(t *testing.T) {
	db := setupTestDB(t)
	g := NewGenerator(db)
	ctx := context.Background()

	const n = 50
	seen := make(map[string]bool, n)
	var mu sync.Mutex
	var wg sync.WaitGroup
	errs := make(chan error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c, err := g.Next(ctx, "TS")
			if err != nil {
				errs <- fmt.Errorf("goroutine %d: %w", i, err)
				return
			}
			mu.Lock()
			if seen[c] {
				errs <- fmt.Errorf("单号重复: %s", c)
			}
			seen[c] = true
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if len(seen) != n {
		t.Errorf("期望 %d 个唯一单号，实际 %d", n, len(seen))
	}
}
