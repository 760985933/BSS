package services

import (
	"context"

	"gorm.io/gorm"

	"bss/internal/models"
)

// KV 简单的键值计数
type KV struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

// OwnerKV 按负责人的计数（带姓名）
type OwnerKV struct {
	OwnerID uint   `json:"owner_id"`
	Name    string `json:"name"`
	Count   int    `json:"count"`
}

// LostAnalysis 商单输单分析结果
type LostAnalysis struct {
	Total    int      `json:"total"`
	ByReason []KV     `json:"by_reason"`
	ByOwner  []OwnerKV `json:"by_owner"`
	ByMonth  []KV     `json:"by_month"`
}

// LostDealAnalysis 对 lost 商单按输单原因 / 负责人 / 月份聚合分布。
// 纯只读统计，不写任何数据。
func LostDealAnalysis(ctx context.Context, db *gorm.DB) (*LostAnalysis, error) {
	res := &LostAnalysis{}

	var total int64
	if err := db.Model(&models.Deal{}).
		Where("status = ? AND deleted_at IS NULL", models.DealLost).
		Count(&total).Error; err != nil {
		return nil, err
	}
	res.Total = int(total)

	// 按输单原因
	type rc struct {
		LostReason string
		Cnt        int
	}
	var rs []rc
	if err := db.Model(&models.Deal{}).
		Select("lost_reason, COUNT(*) AS cnt").
		Where("status = ? AND deleted_at IS NULL", models.DealLost).
		Group("lost_reason").Scan(&rs).Error; err != nil {
		return nil, err
	}
	for _, r := range rs {
		res.ByReason = append(res.ByReason, KV{Key: r.LostReason, Count: r.Cnt})
	}

	// 按负责人
	type oc struct {
		OwnerID uint
		Cnt     int
	}
	var os []oc
	if err := db.Model(&models.Deal{}).
		Select("owner_id, COUNT(*) AS cnt").
		Where("status = ? AND deleted_at IS NULL", models.DealLost).
		Group("owner_id").Scan(&os).Error; err != nil {
		return nil, err
	}
	empNames := map[uint]string{}
	for _, o := range os {
		name, ok := empNames[o.OwnerID]
		if !ok {
			var e models.Employee
			if err := db.Where("id = ?", o.OwnerID).First(&e).Error; err == nil {
				name = e.Name
			}
			empNames[o.OwnerID] = name
		}
		res.ByOwner = append(res.ByOwner, OwnerKV{OwnerID: o.OwnerID, Name: name, Count: o.Cnt})
	}

	// 按月份（取 updated_at 的 YYYY-MM，即输单发生月份）
	type mc struct {
		Month string
		Cnt   int
	}
	var ms []mc
	if err := db.Model(&models.Deal{}).
		Select("substr(updated_at,1,7) AS month, COUNT(*) AS cnt").
		Where("status = ? AND deleted_at IS NULL", models.DealLost).
		Group("month").Order("month").Scan(&ms).Error; err != nil {
		return nil, err
	}
	for _, m := range ms {
		res.ByMonth = append(res.ByMonth, KV{Key: m.Month, Count: m.Cnt})
	}

	return res, nil
}
