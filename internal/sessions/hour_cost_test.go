package sessions

import (
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
	if got, want := hourCost(s.hoursOf()["deepseek"], s.Models["deepseek"], peak), 1+4+0.5+2.0; got != want {
		t.Errorf("an hour at peak and an hour off it cost %v, want %v", got, want)
	}
	// a reader that knows the day but no hour bills the day at the base price
	if got, want := hourCost(nil, s.Models["deepseek"], peak), 2*(0.5+2.0); got != want {
		t.Errorf("a day with no hour cost %v, want %v", got, want)
	}

	s.unuseAt(evening, "deepseek", one)
	if got, want := hourCost(s.hoursOf()["deepseek"], s.Models["deepseek"], peak), 1+4.0; got != want {
		t.Errorf("after taking the evening back, %v, want %v", got, want)
	}
	if len(s.hoursOf()["deepseek"]) != 1 {
		t.Errorf("the evening's hour is still counted: %v", s.hoursOf()["deepseek"])
	}

	// a reader that knows only the date still counts the tokens, in no hour
	s.use("2026-10-19", "other", one)
	if got, want := hourCost(s.hoursOf()["other"], s.Models["other"], peak), 0.5+2.0; got != want {
		t.Errorf("a day-only count costs %v, want %v", got, want)
	}
}
