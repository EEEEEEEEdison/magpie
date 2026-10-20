package settings

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/catalog"
)

// A price with windows of the week, a zone and off days is kept and read back
// whole; a window's part left out is the day price's, and is written out only
// where it differs; a window that is no window, a zone this computer does not
// know and a day that is not YYYY-MM-DD are refused by name.
func TestModelPriceTimeTiers(t *testing.T) {
	p := catalog.Price{Input: 0.3, Output: 1.2, CacheRead: 0.01, TimeZone: "Asia/Shanghai",
		TimeTiers: []catalog.TimeTier{{Days: []int{1, 2, 3, 4, 5}, From: "09:00", To: "12:00",
			Input: 0.6, Output: 2.4, CacheRead: 0.02}},
		OffDates: []string{"2026-10-01", "2026-10-02"}}
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

	// a window that bills as the day does has its parts left out, and reads
	// back as them
	quiet := catalog.Price{Input: 1, Output: 2, TimeTiers: []catalog.TimeTier{{From: "00:00", To: "09:00", Input: 1, Output: 2}}}
	b, err = json.Marshal(StatedPrice(quiet))
	if err != nil {
		t.Fatal(err)
	}
	back = ModelPrice{}
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if len(back.TimeTiers) != 1 || back.TimeTiers[0].Input != nil || back.TimeTiers[0].Output != nil {
		t.Errorf("a window billing the day price is written %s", b)
	}
	if got, bad := back.Price(); bad != "" || !got.Same(quiet) {
		t.Errorf("read back %s as %+v, %q", b, got, bad)
	}

	for name, m := range map[string]ModelPrice{
		"time tier 1":   {TimeTiers: []TimeTier{{From: "09:00", To: "09:00"}}},
		"time tier 1 ":  {TimeTiers: []TimeTier{{From: "", To: "12:00"}}},
		"time tier 1  ": {TimeTiers: []TimeTier{{Days: []int{0}, From: "09:00", To: "12:00"}}},
		"time tier 2":   {TimeTiers: []TimeTier{{From: "09:00", To: "12:00"}, {From: "14:00", To: "18:00", Input: new(-1.0)}}},
		"off date":      {OffDates: []string{"2026-10-01", "2026-13-01"}},
		"time zone":     {TimeZone: new("Mars/Olympus")},
	} {
		m.Input, m.Output, m.CacheRead, m.CacheWrite = new(1.0), new(2.0), new(0.0), new(0.0)
		if _, bad := m.Price(); bad != strings.TrimSpace(name) {
			t.Errorf("%s: bad = %q", name, bad)
		}
		if err := CheckModelPrice("relay/x", m); err == nil {
			t.Errorf("%s: no error", name)
		}
	}

	// a zone of this computer's is no zone to name
	local := ModelPrice{Input: new(1.0), Output: new(2.0), CacheRead: new(0.0), CacheWrite: new(0.0), TimeZone: new("Local")}
	if _, bad := local.Price(); bad != "" {
		t.Errorf("time zone Local: bad = %q", bad)
	}
}
