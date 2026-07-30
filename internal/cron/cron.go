package cron

import (
	"context"
	"log"
	"time"

	"bss/internal/services"

	"gorm.io/gorm"
)

// Start 启动后台调度：每日 09:00 扫描到期/逾期并生成提醒通知。
// 在独立 goroutine 中运行，直到 ctx 取消。
func Start(ctx context.Context, db *gorm.DB) {
	go func() {
		for {
			now := time.Now()
			next := nextAt(now, 9, 0)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Until(next)):
				if _, err := services.ScanReminders(ctx, db, time.Now()); err != nil {
					log.Printf("[cron] 提醒扫描失败: %v", err)
				} else {
					log.Printf("[cron] 提醒扫描完成 @ %s", time.Now().Format(time.RFC3339))
				}
			}
		}
	}()
}

// nextAt 返回当前时刻之后最近的 h:m（本地时区）。
func nextAt(t time.Time, h, m int) time.Time {
	next := time.Date(t.Year(), t.Month(), t.Day(), h, m, 0, 0, t.Location())
	if !next.After(t) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}
