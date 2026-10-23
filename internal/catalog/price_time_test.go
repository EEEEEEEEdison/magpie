package catalog

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

// deepseek is a price as the user states DeepSeek's: off-peak in the base
// parts, on-peak in two windows of the working week, a holiday billed at
// off-peak whatever the weekday.
const deepseek = `{
	"input": 1.5, "output": 4.5, "cache_read": 0.05, "cache_write": 0,
	"time_zone": "Asia/Shanghai",
	"time_tiers": [
		{"days": [1, 2, 3, 4, 5], "from": "09:00", "to": "12:00", "input": 3, "output": 9, "cache_read": 0.1},
		{"days": [1, 2, 3, 4, 5], "from": "14:00", "to": "18:00", "input": 3, "output": 9, "cache_read": 0.1}
	],
	"off_dates": ["2026-01-01", "2026-10-01"]
}`

func shanghai(t *testing.T) *time.Location {
	t.Helper()
	where, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("Asia/Shanghai: %v", err)
	}
	return where
}

func TestAtTimePicksTheWindow(t *testing.T) {
	var p Price
	if err := json.Unmarshal([]byte(deepseek), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(p.TimeTiers) != 2 || p.TimeZone != "Asia/Shanghai" || len(p.OffDates) != 2 {
		t.Fatalf("read back: %+v", p)
	}
	// the second window leaves cache_write out, which is the base's 0
	if p.TimeTiers[1].CacheWrite != 0 || p.TimeTiers[1].CacheWrite1h != 0 {
		t.Fatalf("a part left out took the base's: %+v", p.TimeTiers[1])
	}
	where := shanghai(t)
	cases := []struct {
		at   time.Time
		peak bool
	}{
		{time.Date(2026, 10, 20, 8, 59, 0, 0, where), false},  // a Tuesday, before 9
		{time.Date(2026, 10, 20, 9, 0, 0, 0, where), true},    // 9:00 is in
		{time.Date(2026, 10, 20, 11, 59, 0, 0, where), true},  //
		{time.Date(2026, 10, 20, 12, 0, 0, 0, where), false},  // 12:00 is out
		{time.Date(2026, 10, 20, 13, 59, 0, 0, where), false}, //
		{time.Date(2026, 10, 20, 14, 0, 0, 0, where), true},   // the second window
		{time.Date(2026, 10, 20, 17, 59, 0, 0, where), true},  //
		{time.Date(2026, 10, 20, 18, 0, 0, 0, where), false},  //
		{time.Date(2026, 10, 24, 10, 0, 0, 0, where), false},  // a Saturday
		{time.Date(2026, 10, 1, 10, 0, 0, 0, where), false},   // a holiday, a Thursday
		{time.Date(2026, 10, 22, 10, 0, 0, 0, where), true},   // a Thursday
	}
	for _, c := range cases {
		got := p.AtTime(c.at)
		want := catalogPrice(1.5, 4.5, 0.05)
		if c.peak {
			want = catalogPrice(3, 9, 0.1)
		}
		if !got.At(0).Same(want) {
			t.Errorf("at %s: got %+v, want %+v", c.at.Format("2006-01-02 15:04 Mon"), got.At(0), want)
		}
		if in := p.InTimeTier(c.at); in != c.peak {
			t.Errorf("at %s: InTimeTier %v, want %v", c.at.Format("2006-01-02 15:04 Mon"), in, c.peak)
		}
	}
}

// catalogPrice is a flat price of input, output and cache read.
func catalogPrice(input, output, cacheRead float64) Price {
	return Price{Input: input, Output: output, CacheRead: cacheRead}
}

func TestAtTimeKeepsTheSizeTier(t *testing.T) {
	p := Price{Input: 1, Output: 2, CacheRead: 0.1, TimeZone: "UTC",
		TimeTiers: []TimeTier{{From: "09:00", To: "12:00", Input: 2, Output: 4, CacheRead: 0.2}},
		Tiers:     []Tier{{Above: 1000, Input: 5, Output: 10, CacheRead: 0.5}}}
	at := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	if got := p.AtTime(at).At(2000); got.Input != 5 {
		t.Errorf("a peak request over the size tier: %+v, want the tier's 5", got)
	}
	if got := p.AtTime(at).At(10); got.Input != 2 {
		t.Errorf("a peak request under the size tier: %+v, want the window's 2", got)
	}
	if got := p.CostSplitAt(at, 1000000, 0, 0, 0, 0); math.Abs(got-5) > 1e-9 {
		t.Errorf("a million input tokens at peak: %v, want the size tier's 5", got)
	}
}

func TestAtTimeWithoutATime(t *testing.T) {
	var p Price
	if err := json.Unmarshal([]byte(deepseek), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// no time told: the base price, as a price of no windows is
	if got := p.AtTime(time.Time{}); !got.At(0).Same(catalogPrice(1.5, 4.5, 0.05)) {
		t.Errorf("no time: %+v", got.At(0))
	}
	if p.InTimeTier(time.Time{}) {
		t.Error("no time is in a window")
	}
	if (Price{Input: 1}).InTimeTier(time.Now()) {
		t.Error("a price of no windows holds a request")
	}
}

func TestTimeTierWindowsWrapMidnight(t *testing.T) {
	p := Price{Input: 1, TimeZone: "UTC", TimeTiers: []TimeTier{{From: "22:00", To: "02:00", Input: 2}}}
	for _, c := range []struct {
		hour int
		peak bool
	}{{21, false}, {22, true}, {23, true}, {0, true}, {1, true}, {2, false}, {12, false}} {
		at := time.Date(2026, 10, 20, c.hour, 0, 0, 0, time.UTC)
		if got := p.InTimeTier(at); got != c.peak {
			t.Errorf("at %d:00: %v, want %v", c.hour, got, c.peak)
		}
	}
}

func TestTimeTierLeftOutWhenUnreadable(t *testing.T) {
	cases := []string{
		`{"input": 1, "time_tiers": [{"from": "9am", "to": "12:00"}]}`,
		`{"input": 1, "time_tiers": [{"from": "09:00", "to": "09:00"}]}`,
		`{"input": 1, "time_tiers": [{"from": "09:00", "to": "12:00", "input": 2, "days": [8]}]}`,
		`{"input": 1, "time_tiers": [{"from": "24:30", "to": "12:00"}]}`,
	}
	for _, c := range cases {
		var p Price
		if err := json.Unmarshal([]byte(c), &p); err != nil {
			t.Fatalf("%s: %v", c, err)
		}
		if len(p.TimeTiers) != 0 {
			t.Errorf("%s: kept %+v", c, p.TimeTiers)
		}
	}
}

func TestTimesAndSameKeepTheWindows(t *testing.T) {
	var p Price
	if err := json.Unmarshal([]byte(deepseek), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	half := p.Times(0.5)
	if half.TimeTiers[0].Input != 1.5 || half.Input != 0.75 || half.TimeZone != p.TimeZone || len(half.OffDates) != len(p.OffDates) {
		t.Fatalf("half: %+v", half)
	}
	if p.Same(half) || !p.Same(p) {
		t.Error("Same")
	}
	other := p
	other.OffDates = []string{"2026-01-01"}
	if p.Same(other) {
		t.Error("a shorter OffDates is not the same price")
	}
}

// A long-context price a vendor charges for a while is a tier with the days
// it holds on: a request within them is billed the tier, one before or after
// them the price above, and one of no time told the tier, as a price read
// without a time has only the file's word.
func TestADatedTierHoldsOnItsDays(t *testing.T) {
	p := Price{Input: 1, Output: 2, CacheRead: 0.1, TimeZone: "UTC",
		Tiers: []Tier{{Above: 272000, Input: 20, Output: 75, CacheRead: 2, From: "2026-09-01", To: "2026-09-30"}}}
	day := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 12, 0, 0, 0, time.UTC) }
	for _, c := range []struct {
		at   time.Time
		want float64
	}{
		{day(2026, 8, 31), 1},  // the day before it began
		{day(2026, 9, 1), 20},  // its first day
		{day(2026, 9, 30), 20}, // its last
		{day(2026, 10, 1), 1},  // the day it ended
		{time.Time{}, 20},      // no time told: the file's word
	} {
		if got := p.AtTime(c.at).At(500000); got.Input != c.want {
			t.Errorf("over 272k at %v: input %v, want %v", c.at, got.Input, c.want)
		}
	}
	// one day alone: From only, or To only
	open := Price{Input: 1, Output: 2, Tiers: []Tier{{Above: 100, Input: 9, Output: 9, From: "2026-01-01"}}}
	if got := open.AtTime(day(2027, 5, 1)).At(1000); got.Input != 9 {
		t.Errorf("a tier with only its first day ended: %+v", got)
	}
	if got := open.AtTime(day(2025, 12, 31)).At(1000); got.Input != 1 {
		t.Errorf("a tier before its first day: %+v", got)
	}
}

// A model the user says has no long-context price is billed the price above
// whatever its list gave it, and a priced one is not.
func TestNoTiersBillsThePriceAbove(t *testing.T) {
	p := Price{Input: 1, Output: 2, Tiers: []Tier{{Above: 272000, Input: 20, Output: 75}}, NoTiers: true}
	if got := p.At(500000); got.Input != 1 || got.Output != 2 {
		t.Errorf("At over 272k: %+v, want the price above", got)
	}
	if got := p.AtTime(time.Now()).At(500000); got.Input != 1 {
		t.Errorf("AtTime over 272k: %+v, want the price above", got)
	}
	kept := p
	kept.NoTiers = false
	if kept.At(500000).Input != 20 {
		t.Error("the tier is billed again once the word is taken back")
	}
	if p.Same(kept) {
		t.Error("Same reads NoTiers")
	}
}
