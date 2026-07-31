package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	"bss/internal/models"
)

func TestLostDealAnalysis_Basic(t *testing.T) {
	gdb := newTestDB(t)
	cust := mkCustomer(t, gdb, "分析客户", "KH-LA")
	emp1 := mkEmployee(t, gdb, "a1@x.com")
	emp2 := mkEmployee(t, gdb, "a2@x.com")

	seq := 0
	add := func(owner models.Employee, reason string, y, mo, day int) {
		seq++
		d := models.Deal{
			CustomerID: cust.ID,
			Code:       fmt.Sprintf("SD-L%d", seq),
			Title:      "d",
			Status:     models.DealLost,
			LostReason: reason,
			OwnerID:    owner.ID,
		}
		if err := gdb.Create(&d).Error; err != nil {
			t.Fatal(err)
		}
		// GORM 的 Update 会覆盖 updated_at 自动时间戳，用 Exec 直接改
		target := time.Date(y, time.Month(mo), day, 12, 0, 0, 0, time.UTC)
		if err := gdb.Exec("UPDATE deals SET updated_at = ? WHERE id = ?", target, d.ID).Error; err != nil {
			t.Fatal(err)
		}
	}
	// 2026-07：emp1 两个 competitor
	add(emp1, "competitor", 2026, 7, 15)
	add(emp1, "competitor", 2026, 7, 20)
	// 2026-06：emp2 一个 budget
	add(emp2, "budget", 2026, 6, 10)

	res, err := LostDealAnalysis(context.Background(), gdb)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 3 {
		t.Errorf("Total=%d, want 3", res.Total)
	}
	reasonMap := map[string]int{}
	for _, r := range res.ByReason {
		reasonMap[r.Key] = r.Count
	}
	if reasonMap["competitor"] != 2 || reasonMap["budget"] != 1 {
		t.Errorf("by_reason=%v, want competitor:2 budget:1", reasonMap)
	}
	ownerMap := map[uint]int{}
	for _, o := range res.ByOwner {
		ownerMap[o.OwnerID] = o.Count
	}
	if ownerMap[emp1.ID] != 2 || ownerMap[emp2.ID] != 1 {
		t.Errorf("by_owner=%v, want emp1:2 emp2:1", ownerMap)
	}
	monthMap := map[string]int{}
	for _, m := range res.ByMonth {
		monthMap[m.Key] = m.Count
	}
	if monthMap["2026-07"] != 2 || monthMap["2026-06"] != 1 {
		t.Errorf("by_month=%v, want 2026-07:2 2026-06:1", monthMap)
	}
}
