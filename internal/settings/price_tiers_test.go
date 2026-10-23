package settings

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/catalog"
)

// A price with a long-context tier and a 1-hour cache write is kept and read
// back whole; one written before either reads as before, its 1-hour write
// unset (2× input); a tier not above the one before it, or with a bad part,
// is refused by name.
func TestModelPriceTiers(t *testing.T) {
	p := catalog.Price{Input: 10, Output: 50, CacheRead: 1, CacheWrite: 12.5, CacheWrite1h: 20,
		Tiers: []catalog.Tier{{Above: 272000, Input: 20, Output: 75, CacheRead: 2, CacheWrite: 25}}}
	b, err := json.Marshal(StatedPrice(p))
	if err != nil {
		t.Fatal(err)
	}
	var back ModelPrice
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if got, bad := back.Price(); bad != "" || !got.Same(p) {
		t.Errorf("read back %s as %+v, %q; want %+v", b, got, bad, p)
	}

	var old ModelPrice
	if err := json.Unmarshal([]byte(`{"input":1,"output":2,"cache_read":0.1,"cache_write":0}`), &old); err != nil {
		t.Fatal(err)
	}
	if got, bad := old.Price(); bad != "" || !got.Same(catalog.Price{Input: 1, Output: 2, CacheRead: 0.1}) || got.OneHour() != 0 {
		t.Errorf("an old price = %+v, %q", got, bad)
	}
	if b, _ := json.Marshal(StatedPrice(catalog.Price{Input: 1, Output: 2})); strings.Contains(string(b), "cache_write_1h") || strings.Contains(string(b), "tiers") {
		t.Errorf("a price without either is written %s, as before", b)
	}

	for name, m := range map[string]ModelPrice{
		"tier 2":             {Tiers: []catalog.Tier{{Above: 272000, Input: 1}, {Above: 272000, Input: 2}}},
		"tier 1":             {Tiers: []catalog.Tier{{Above: 0, Input: 1}}},
		"tier 1 ":            {Tiers: []catalog.Tier{{Above: 1000, Input: math.NaN()}}},
		"1-hour cache write": {CacheWrite1h: new(-1.0)},
	} {
		m.Input, m.Output, m.CacheRead, m.CacheWrite = new(1.0), new(2.0), new(0.0), new(0.0)
		if _, bad := m.Price(); bad != strings.TrimSpace(name) {
			t.Errorf("%s: bad = %q", name, bad)
		}
		if err := CheckModelPrice("relay/x", m); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// A long-context price with the days it holds on, or the user's word that the
// model has none, is kept and read back whole; a day that is not a date, or
// a from after its to, is refused by name.
func TestModelPriceTierDaysAndNone(t *testing.T) {
	p := catalog.Price{Input: 10, Output: 50, CacheRead: 1, CacheWrite: 12.5,
		Tiers: []catalog.Tier{{Above: 272000, Input: 20, Output: 75, CacheRead: 2, CacheWrite: 25, From: "2026-09-01", To: "2026-09-30"}}}
	b, err := json.Marshal(StatedPrice(p))
	if err != nil {
		t.Fatal(err)
	}
	var back ModelPrice
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if got, bad := back.Price(); bad != "" || !got.Same(p) {
		t.Errorf("read back %s as %+v, %q; want %+v", b, got, bad, p)
	}
	if err := CheckModelPrice("relay/x", back); err != nil {
		t.Errorf("a dated tier: %v", err)
	}

	// the user's word that it has none: the parts stay, the tiers don't
	none := StatedPrice(catalog.Price{Input: 10, Output: 50, CacheRead: 1, CacheWrite: 12.5, NoTiers: true,
		Tiers: []catalog.Tier{{Above: 272000, Input: 20, Output: 75}}})
	got, bad := none.Price()
	if bad != "" || len(got.Tiers) != 0 || !got.NoTiers {
		t.Errorf("no tiers = %+v, %q; want the parts and no tier", got, bad)
	}
	if !strings.Contains(string(mustJSON(t, none)), `"no_tiers":true`) {
		t.Errorf("no_tiers isn't written: %s", mustJSON(t, none))
	}

	for name, m := range map[string]ModelPrice{
		"tier 1":  {Tiers: []catalog.Tier{{Above: 272000, Input: 1, From: "2026-09-31"}}},
		"tier 1 ": {Tiers: []catalog.Tier{{Above: 272000, Input: 1, From: "2026-10-01", To: "2026-09-01"}}},
	} {
		m.Input, m.Output, m.CacheRead, m.CacheWrite = new(1.0), new(2.0), new(0.0), new(0.0)
		if _, bad := m.Price(); bad != strings.TrimSpace(name) {
			t.Errorf("%s: bad = %q", name, bad)
		}
		if err := CheckModelPrice("relay/x", m); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
