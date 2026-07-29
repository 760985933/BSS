package money

import "testing"

func TestParseYuan(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		err  bool
	}{
		{"0", 0, false},
		{"1", 100, false},
		{"1234", 123400, false},
		{"1234.5", 123450, false},
		{"1234.56", 123456, false},
		{"1,234.56", 123456, false},
		{"-9.99", -999, false},
		{" 0.07 ", 7, false},
		{"1.005", 0, true},   // 超 2 位小数
		{"abc", 0, true},
		{"", 0, true},
		{"1.2.3", 0, true},
	}
	for _, c := range cases {
		got, err := ParseYuan(c.in)
		if c.err {
			if err == nil {
				t.Errorf("ParseYuan(%q) 应报错，得到 %d", c.in, got)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("ParseYuan(%q) = %d, %v；期望 %d", c.in, got, err, c.want)
		}
	}
}

func TestFormat(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0.00"},
		{7, "0.07"},
		{100, "1.00"},
		{123450, "1,234.50"},
		{123456, "1,234.56"},
		{-999, "-9.99"},
		{123456789, "1,234,567.89"},
	}
	for _, c := range cases {
		if got := Format(c.in); got != c.want {
			t.Errorf("Format(%d) = %q；期望 %q", c.in, got, c.want)
		}
	}
}

func TestYuanToCentRounding(t *testing.T) {
	if got := YuanToCent(1.005); got != 101 && got != 100 {
		t.Errorf("YuanToCent(1.005) = %d，四舍五入应接近 100/101", got)
	}
	if got := YuanToCent(0.07); got != 7 {
		t.Errorf("YuanToCent(0.07) = %d；期望 7", got)
	}
}

// 往返一致性：Parse -> Format 关键位不丢精度
func TestRoundTrip(t *testing.T) {
	cent, err := ParseYuan("1234567.89")
	if err != nil {
		t.Fatal(err)
	}
	if got := Format(cent); got != "1,234,567.89" {
		t.Errorf("往返后 = %q；期望 1,234,567.89", got)
	}
}
