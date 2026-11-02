package usage

import (
	"math"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/catalog"
	"github.com/yetone/magpie/internal/provider"
)

// 日期限定的长上下文价不能泄漏到日期外的 Requests 行；时段倍率仍按每行应用。
func TestBlockRowsKeepDatedTiersOnTheirDays(t *testing.T) {
	sessionHome(t)
	if err := provider.Save(provider.Provider{ID: "relay", Name: "Relay", Key: "k", Chat: "https://relay.example/v1",
		RateWindows: []provider.RateWindow{{Start: "00:00", End: "24:00", Rate: 3}}}); err != nil {
		t.Fatal(err)
	}
	price := catalog.Price{Input: 2, Output: 8, CacheRead: 0.5, CacheWrite: 2.5, CacheWrite1h: 4,
		Peak: catalog.PeakWeekdays, Tiers: []catalog.Tier{{Above: 100_000, Input: 4, Output: 16, CacheRead: 1, CacheWrite: 5, CacheWrite1h: 6,
			From: "2026-10-19", To: "2026-10-19"}}}
	local := time.Local
	t.Cleanup(func() { time.Local = local })
	for _, tc := range []struct {
		name, at string
		zone     int
		want     float64
	}{
		{"before", "2026-10-18T02:30:00Z", 0, 37.5},
		{"on", "2026-10-19T02:30:00Z", 0, 144},
		{"after", "2026-10-20T02:30:00Z", 0, 75},
		{"east date already started", "2026-10-18T23:30:00Z", 14, 72},
		{"west date not ended", "2026-10-20T02:30:00Z", -11, 144},
	} {
		t.Run(tc.name, func(t *testing.T) {
			time.Local = time.FixedZone(tc.name, tc.zone*3600)
			at, err := time.Parse(time.RFC3339, tc.at)
			if err != nil {
				t.Fatal(err)
			}
			var raw rowChunk
			for i := 0; i < logBlockRows; i++ {
				raw.add(Row{Record: Record{Time: at, Provider: "relay", Model: "sol", Input: 1_000_000, Output: 1_000_000,
					CacheRead: 1_000_000, CacheWrite: 500_000, CacheWrite1h: 500_000}}, "", int64(i), false)
			}
			b := newBlockPricer(provider.Renames{}, nil, func(Record) *catalog.Price { return &price })
			t.Cleanup(b.release)
			c := b.block(raw.pack())
			if c == nil {
				t.Fatal("no priced block")
			}
			for i := range c.Rows {
				if got := c.row(i).Cost; math.Abs(got-tc.want) > 1e-9 {
					t.Fatalf("row %d costs %v, want %v", i, got, tc.want)
				}
			}
		})
	}
}
