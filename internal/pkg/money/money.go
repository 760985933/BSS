// Package money 元 ↔ 分转换的唯一入口（PRD §7.3：全项目金额存整数分，禁用浮点运算）
package money

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ParseYuan 将用户输入的"元"字符串解析为分。
// 支持 "1234"、"1234.5"、"1234.56"、"1,234.56"；拒绝超过 2 位小数与非法输入。
func ParseYuan(s string) (int64, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", ""))
	if s == "" {
		return 0, fmt.Errorf("金额不能为空")
	}
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	parts := strings.Split(s, ".")
	if len(parts) > 2 {
		return 0, fmt.Errorf("金额格式非法: %q", s)
	}
	yuan, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("金额格式非法: %q", s)
	}
	var cent int64
	if len(parts) == 2 {
		frac := parts[1]
		if len(frac) > 2 {
			return 0, fmt.Errorf("金额最多支持 2 位小数: %q", s)
		}
		if len(frac) == 1 {
			frac += "0"
		}
		cent, err = strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("金额格式非法: %q", s)
		}
	}
	total := yuan*100 + cent
	if neg {
		total = -total
	}
	return total, nil
}

// YuanToCent 浮点元转分（四舍五入）。仅限万不得已的浮点输入场景，优先使用 ParseYuan。
func YuanToCent(yuan float64) int64 {
	return int64(math.Round(yuan * 100))
}

// Format 分转展示字符串："123456" -> "1,234.56"
func Format(cent int64) string {
	neg := cent < 0
	if neg {
		cent = -cent
	}
	yuan := cent / 100
	frac := cent % 100
	s := strconv.FormatInt(yuan, 10)
	// 千分位
	var sb strings.Builder
	n := len(s)
	for i, ch := range s {
		if i > 0 && (n-i)%3 == 0 {
			sb.WriteByte(',')
		}
		sb.WriteRune(ch)
	}
	if neg {
		return fmt.Sprintf("-%s.%02d", sb.String(), frac)
	}
	return fmt.Sprintf("%s.%02d", sb.String(), frac)
}
