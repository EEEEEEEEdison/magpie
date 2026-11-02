package sessions

import (
	"testing"
	"time"

	"github.com/yetone/magpie/internal/catalog"
)

// A session's tokens are billed at the price of each hour they were spent in
// (DeepSeek's peak hours), and what no hour holds at the base price; taking a
// message back takes back its hour too.
func TestHourCostBillsEachHourItsPeakPrice(t *testing.T) {
	peak := catalog.Price{Input: 0.5, Output: 2, Peak: catalog.PeakWeekdays}
	monday := time.Date(2026, 10, 19, 2, 30, 0, 0, time.UTC)
	evening := time.Date(2026, 10, 19, 20, 30, 0, 0, time.UTC)
	if monday.Weekday() != time.Monday {
		t.Fatalf("2026-10-19 is a %s", monday.Weekday())
	}
	one := Tokens{Input: 1_000_000, Output: 1_000_000}

	s := &state{}
	s.useAt(monday, "deepseek", one)
	s.useAt(evening, "deepseek", one)
	if got, want := hourCost(s.hoursOf()["deepseek"], s.Models.of("deepseek"), peak), 1+4+0.5+2.0; got != want {
		t.Errorf("an hour at peak and an hour off it cost %v, want %v", got, want)
	}
	// a reader that knows the day but no hour bills the day at the base price
	if got, want := hourCost(nil, s.Models.of("deepseek"), peak), 2*(0.5+2.0); got != want {
		t.Errorf("a day with no hour cost %v, want %v", got, want)
	}

	s.unuseAt(evening, "deepseek", one)
	if got, want := hourCost(s.hoursOf()["deepseek"], s.Models.of("deepseek"), peak), 1+4.0; got != want {
		t.Errorf("after taking the evening back, %v, want %v", got, want)
	}
	if len(s.hoursOf()["deepseek"]) != 1 {
		t.Errorf("the evening's hour is still counted: %v", s.hoursOf()["deepseek"])
	}

	// a reader that knows only the date still counts the tokens, in no hour
	s.use("2026-10-19", "other", one)
	if got, want := hourCost(s.hoursOf()["other"], s.Models.of("other"), peak), 0.5+2.0; got != want {
		t.Errorf("a day-only count costs %v, want %v", got, want)
	}
}

// 多次短请求的小时总量不能被当成一次长上下文请求，未知时间部分也用基础档。
func TestHourCostDoesNotTierTheAggregate(t *testing.T) {
	at := time.Date(2026, 10, 19, 2, 30, 0, 0, time.UTC)
	price := catalog.Price{Input: 2, Output: 8, CacheRead: 0.5, CacheWrite: 2.5, CacheWrite1h: 4,
		Peak: catalog.PeakWeekdays, Tiers: []catalog.Tier{{Above: 100_000, Input: 20, Output: 80, CacheRead: 5, CacheWrite: 25,
			From: "2026-10-19", To: "2026-10-19"}}}
	s := &state{}
	one := Tokens{Input: 25_000, Output: 10_000, CacheRead: 25_000, CacheWrite: 10_000, CacheWrite1h: 5_000}
	for i := 0; i < 20; i++ {
		s.useAt(at, "sol", one)
	}
	if got := hourCost(s.hoursOf()["sol"], s.Models.of("sol"), price); got != 7 {
		t.Errorf("20 small calls in a peak hour cost %v, want 7", got)
	}
	if got := hourCost(nil, s.Models.of("sol"), price); got != 3.5 {
		t.Errorf("aggregate without an hour costs %v, want 3.5", got)
	}
	s.use("2026-10-19", "sol", Tokens{Input: 1_000_000})
	if got := hourCost(s.hoursOf()["sol"], s.Models.of("sol"), price); got != 9 {
		t.Errorf("peak hour plus unknown-hour input costs %v, want 9", got)
	}
}
