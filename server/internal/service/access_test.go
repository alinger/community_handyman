package service

import (
	"testing"
	"time"

	"github.com/alinger/community-handyman/server/internal/model"
)

func day(y int, m time.Month, d int) model.Date {
	return model.NewDate(time.Date(y, m, d, 0, 0, 0, 0, time.Local))
}

func TestSlotRangeFormats(t *testing.T) {
	cases := []struct {
		code           string
		sh, sm, eh, em int
		ok             bool
	}{
		{"22:00-22:30", 22, 0, 22, 30, true},
		{"08-10", 8, 0, 10, 0, true},
		{"07:00-07:30", 7, 0, 7, 30, true},
		{"", 0, 0, 0, 0, false},
		{"abc", 0, 0, 0, 0, false},
		{"25:00-26:00", 0, 0, 0, 0, false},
		{"22:00", 0, 0, 0, 0, false},
	}
	d := day(2026, 11, 25)
	for _, c := range cases {
		start, end, ok := slotRange(d, c.code)
		if ok != c.ok {
			t.Fatalf("%q 可解析性期望 %v，实际 %v", c.code, c.ok, ok)
		}
		if !ok {
			continue
		}
		if start.Hour() != c.sh || start.Minute() != c.sm {
			t.Fatalf("%q 起点期望 %02d:%02d，实际 %02d:%02d",
				c.code, c.sh, c.sm, start.Hour(), start.Minute())
		}
		if end.Hour() != c.eh || end.Minute() != c.em {
			t.Fatalf("%q 终点期望 %02d:%02d，实际 %02d:%02d",
				c.code, c.eh, c.em, end.Hour(), end.Minute())
		}
	}
}

// TestAccessWindowBoundary 覆盖需求里的「服务开始前 2 小时至服务结束后」边界。
func TestAccessWindowBoundary(t *testing.T) {
	d := day(2026, 11, 25)
	// 时段 22:00-22:30，窗口应为 20:00 ~ 22:30
	cases := []struct {
		name string
		at   time.Time
		want bool
	}{
		{"前一天夜间", time.Date(2026, 11, 24, 23, 0, 0, 0, time.Local), false},
		{"窗口前 1 分钟", time.Date(2026, 11, 25, 19, 59, 0, 0, time.Local), false},
		{"窗口起点", time.Date(2026, 11, 25, 20, 0, 0, 0, time.Local), true},
		{"服务开始", time.Date(2026, 11, 25, 22, 0, 0, 0, time.Local), true},
		{"服务结束", time.Date(2026, 11, 25, 22, 30, 0, 0, time.Local), true},
		{"服务结束后 1 分钟", time.Date(2026, 11, 25, 22, 31, 0, 0, time.Local), false},
		{"次日", time.Date(2026, 11, 26, 9, 0, 0, 0, time.Local), false},
	}
	for _, c := range cases {
		got := accessWindowOpen(d, "22:00-22:30", c.at)
		if got != c.want {
			t.Fatalf("%s：期望 %v，实际 %v", c.name, c.want, got)
		}
	}
}

// TestAccessWindowShortFormat 短格式时段（08-10）同样要能算窗口。
func TestAccessWindowShortFormat(t *testing.T) {
	d := day(2026, 11, 25)
	if !accessWindowOpen(d, "08-10", time.Date(2026, 11, 25, 6, 30, 0, 0, time.Local)) {
		t.Fatal("06:30 应在 08-10 的窗口内（提前 2 小时）")
	}
	if accessWindowOpen(d, "08-10", time.Date(2026, 11, 25, 5, 59, 0, 0, time.Local)) {
		t.Fatal("05:59 还没到窗口（窗口从 06:00 起）")
	}
}

// TestAccessWindowBadCode 解析不了的时段码必须判不可见，
// 免得脏数据把门禁密码提前泄露出去。
func TestAccessWindowBadCode(t *testing.T) {
	d := day(2026, 11, 25)
	if accessWindowOpen(d, "unknown", time.Date(2026, 11, 25, 22, 0, 0, 0, time.Local)) {
		t.Fatal("非法时段码不应开放可见窗口")
	}
}
