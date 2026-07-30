package cron

import (
	"context"
	"log"
	"time"

	"bss/internal/services"

	"gorm.io/gorm"
)

// Start 启动后台调度：
//   - 每日 09:00 扫描到期/逾期并生成提醒通知
//   - 每日 02:00 按公海规则回收超期未跟进客户（M3-1，规则未启用时跳过）
//
// 在独立 goroutine 中运行，直到 ctx 取消。
func Start(ctx context.Context, db *gorm.DB) {
	go loop(ctx, 9, 0, "提醒扫描", func() error {
		_, err := services.ScanReminders(ctx, db, time.Now())
		return err
	})
	go loop(ctx, 2, 0, "公海回收", func() error {
		pool := services.NewPoolService(db)
		st, err := pool.Settings(ctx)
		if err != nil {
			return err
		}
		if !st.Enabled {
			return nil // 规则未启用：不自动回收
		}
		res, err := pool.Recycle(ctx, time.Now(), false)
		if err != nil {
			return err
		}
		if res.Total > 0 {
			log.Printf("[cron] 公海回收 %d 个客户", res.Total)
		}
		return nil
	})
}

// loop 每日在 h:m 执行一次 fn，直到 ctx 取消。
func loop(ctx context.Context, h, m int, name string, fn func() error) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(nextAt(time.Now(), h, m))):
			if err := fn(); err != nil {
				log.Printf("[cron] %s失败: %v", name, err)
			} else {
				log.Printf("[cron] %s完成 @ %s", name, time.Now().Format(time.RFC3339))
			}
		}
	}
}

// nextAt 返回当前时刻之后最近的 h:m（本地时区）。
func nextAt(t time.Time, h, m int) time.Time {
	next := time.Date(t.Year(), t.Month(), t.Day(), h, m, 0, 0, t.Location())
	if !next.After(t) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}
