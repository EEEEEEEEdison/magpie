package sessions

import (
	"math"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/catalog"
)

// A session's tokens are billed at the price of the window each hour they were
// spent in is in (DeepSeek's peak hours), and what no hour holds at the base
// price; taking a message back takes back its hour too.
func TestHourCostBillsEachHourItsWindowsPrice(t *testing.T) {
	where, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("Asia/Shanghai: %v", err)
	}
	peak := catalog.Price{Input: 0.5, Output: 2, TimeZone: "Asia/Shanghai",
		TimeTiers: []catalog.TimeTier{
			{Days: []int{1, 2, 3, 4, 5}, From: "09:00", To: "12:00", Input: 1, Output: 4},
			{Days: []int{1, 2, 3, 4, 5}, From: "14:00", To: "18:00", Input: 1, Output: 4},
		}}
	monday := time.Date(2026, 10, 19, 10, 30, 0, 0, where)
	evening := time.Date(2026, 10, 19, 20, 30, 0, 0, where)
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

// A zone whose offset is not a whole hour puts a window's end inside an
// hour, and that hour is billed by the minutes it spends on each side
// rather than wholly at the price of one of them.
func TestHourCostSplitsAnHourOnAHalfHourZone(t *testing.T) {
	if _, err := time.LoadLocation("Asia/Kolkata"); err != nil {
		t.Skipf("Asia/Kolkata: %v", err)
	}
	// 09:00-12:00 Kolkata is 03:30-06:30 UTC: the hour keyed 03 holds half
	// an hour off-peak and half on-peak. Billed wholly at the hour's start
	// it would be off-peak; at its end, on-peak.
	peak := catalog.Price{Input: 1, Output: 4, TimeZone: "Asia/Kolkata",
		TimeTiers: []catalog.TimeTier{{From: "09:00", To: "12:00", Input: 2, Output: 8}}}
	one := Tokens{Input: 1_000_000, Output: 1_000_000}
	hours := map[string]Tokens{
		hourOf(time.Date(2026, 10, 20, 3, 0, 0, 0, time.UTC)): one, // half off, half on
	}
	// a million of each: off-peak an hour costs 1+4 = 5, peak 2+8 = 10.
	// Half and half is half of each: 2.5+5 = 7.5. An implementation that
	// bills the hour whole at either end's price gives 5 or 10.
	got := hourCost(hours, Tokens{Input: 1_000_000, Output: 1_000_000}, peak)
	if math.Abs(got-7.5) > 1e-9 {
		t.Errorf("an hour half either side of a window's start cost %v, want 7.5", got)
	}
	// the hour wholly inside the window keeps the peak price whole, so a
	// zone on the hour bills what it always did
	inside := map[string]Tokens{hourOf(time.Date(2026, 10, 20, 4, 0, 0, 0, time.UTC)): one}
	if got := hourCost(inside, Tokens{Input: 1_000_000, Output: 1_000_000}, peak); math.Abs(got-10) > 1e-9 {
		t.Errorf("an hour wholly at peak cost %v, want 10", got)
	}
	// and an hour wholly outside it the base price
	outside := map[string]Tokens{hourOf(time.Date(2026, 10, 20, 1, 0, 0, 0, time.UTC)): one}
	if got := hourCost(outside, Tokens{Input: 1_000_000, Output: 1_000_000}, peak); math.Abs(got-5) > 1e-9 {
		t.Errorf("an hour wholly off peak cost %v, want 5", got)
	}
}
