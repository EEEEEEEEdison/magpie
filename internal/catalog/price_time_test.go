package catalog

import (
	"testing"
	"time"
)

// Peak scales the size tiers with the base price, and CostSplit does not
// apply the same peak a second time.
func TestAtTimeKeepsTheSizeTier(t *testing.T) {
	p := Price{Input: 1, Output: 2, CacheRead: 0.1, Peak: PeakWeekdays,
		Tiers: []Tier{{Above: 1000, Input: 5, Output: 10, CacheRead: 0.5}}}
	at := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	if got := p.AtTime(at).At(2000); got.Input != 10 {
		t.Errorf("a peak request over the size tier: %+v, want the scaled tier's 10", got)
	}
	if got := p.AtTime(at).At(10); got.Input != 2 {
		t.Errorf("a peak request under the size tier: %+v, want the scaled base's 2", got)
	}
	if got := p.AtTime(at).CostSplit(1000000, 0, 0, 0, 0); got != 10 {
		t.Errorf("a million input tokens at peak: %v, want the scaled size tier's 10", got)
	}
}

func TestTimesAndSameKeepDatedTiers(t *testing.T) {
	p := Price{Input: 1.5, Output: 3, Peak: PeakWeekdays,
		Tiers: []Tier{{Above: 1000, Input: 3, Output: 6, From: "2026-09-01", To: "2026-09-30"}}}
	half := p.Times(0.5)
	if half.Tiers[0].Input != 1.5 || half.Input != 0.75 || half.Peak != p.Peak ||
		half.Tiers[0].From != p.Tiers[0].From || half.Tiers[0].To != p.Tiers[0].To {
		t.Fatalf("half: %+v", half)
	}
	if p.Same(half) || !p.Same(p) {
		t.Error("Same")
	}
	other := p
	other.Tiers = append([]Tier(nil), p.Tiers...)
	other.Tiers[0].To = "2026-09-29"
	if p.Same(other) {
		t.Error("a tier with another last day is not the same price")
	}
}

// A long-context price a vendor charges for a while is a tier with the days
// it holds on: a request within them is billed the tier, one before or after
// them the price above, and one of no time told the tier, as a price read
// without a time has only the file's word.
func TestADatedTierHoldsOnItsDays(t *testing.T) {
	p := Price{Input: 1, Output: 2, CacheRead: 0.1,
		Tiers: []Tier{{Above: 272000, Input: 20, Output: 75, CacheRead: 2, From: "2026-09-01", To: "2026-09-30"}}}
	day := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 12, 0, 0, 0, time.Local) }
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
