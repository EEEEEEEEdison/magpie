package usage

import (
	"testing"
	"time"

	"github.com/yetone/magpie/internal/catalog"
	"github.com/yetone/magpie/internal/provider"
)

// A reply's speed is its whole turn's, whatever it reasoned: every token
// it wrote, reasoning included, over the ms the call took. What still tells
// none is a turn whose content a vendor held and let go (its flow a small
// part of the turn), and one outside the bounds that make any rate mean
// anything.
func TestWholeTurnSpeed(t *testing.T) {
	visible := Record{Time: time.Now(), Output: 1500, Reasoning: 1400, Millis: 15000, TTFT: 1000, FirstReasoning: 1000, Flow: 12000, Status: 200}
	for _, c := range []struct {
		name string
		r    Record
		n    int
		w    int64
	}{
		{"a turn that streamed as it was written", visible, 1500, 15000},
		{"its reasoning counts with it", func() Record { r := visible; r.FirstText = 14000; return r }(), 1500, 15000},
		{"old records without the timing", func() Record { r := visible; r.FirstReasoning = 0; return r }(), 1500, 15000},
		{"no flow known", func() Record { r := visible; r.Flow = 0; return r }(), 1500, 15000},
		{"a reasoning count over its output", func() Record { r := visible; r.Reasoning = 1600; return r }(), 1500, 15000},
		{"content held and let go in a twelfth of the turn", func() Record { r := visible; r.Flow = 1000; return r }(), 0, 0},
		{"held right up to the quarter", func() Record { r := visible; r.Flow = 3749; return r }(), 0, 0},
		{"at the quarter", func() Record { r := visible; r.Flow = 3750; return r }(), 1500, 15000},
		{"below the decode minimum", Record{Output: 10, Millis: 99}, 0, 0},
		{"at the decode minimum", Record{Output: 10, Millis: 100}, 10, 100},
		{"above the max speed", Record{Output: 1001, Millis: 100}, 0, 0},
		{"at the max speed", Record{Output: 1000, Millis: 100}, 1000, 100},
		{"reasoning hidden from the stream", Record{Output: 120, Millis: 9000, TTFT: 1000, Flow: 7000, ReasoningHidden: true}, 120, 9000},
		{"a late hidden think, held to the end", Record{Output: 1500, Reasoning: 1400, Millis: 9000, TTFT: 8000, Flow: 900}, 0, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			if n, w := c.r.Decode(); n != c.n || w != c.w {
				t.Fatalf("speed counted %d tokens/%d ms; want %d/%d", n, w, c.n, c.w)
			}
		})
	}
	var chunk rowChunk
	for i := 0; i < 80; i++ {
		chunk.add(Row{Record: visible}, "", int64(i), false)
	}
	packed := chunk.pack()
	if len(packed.Archive) == 0 {
		t.Fatal("snapshot not compressed")
	}
	for _, c := range []*rowChunk{&chunk, packed.unpack()} {
		row := c.row(0)
		if n, w := row.Decode(); row.FirstReasoning != 1000 || n != 1500 || w != 15000 {
			t.Fatalf("cache lost streamed reasoning: %+v", row)
		}
	}
	sealed := visible
	sealed.ReasoningHidden = true
	var hiddenChunk rowChunk
	for i := 0; i < 80; i++ {
		hiddenChunk.add(Row{Record: sealed}, "", int64(i), false)
	}
	for _, c := range []*rowChunk{&hiddenChunk, hiddenChunk.pack().unpack()} {
		row := c.row(0)
		n, w := row.Decode()
		if !row.ReasoningHidden || n != 1500 || w != 15000 {
			t.Fatal("cache lost hidden reasoning flag")
		}
	}
	var sum Totals
	sum.addRow(Row{Record: visible})
	hidden := visible
	hidden.FirstReasoning = 0
	sum.addRow(Row{Record: hidden})
	if sum.DecodeOut != 3000 || sum.DecodeMs != 30000 {
		t.Fatalf("aggregate: %+v", sum)
	}
}

// A row the Requests page reads back has been priced, and pricing sets
// swapKnown on it: that bit is not ReasoningHidden's, or every priced reply
// with no answer text would lose its speed (#1463).
func TestPricedRowKeepsReasoningEvidence(t *testing.T) {
	visible := Record{Time: time.Now(), Provider: "deepseek", Model: "deepseek-chat", Output: 1500, Reasoning: 1400, Millis: 15000, TTFT: 1000, FirstReasoning: 1000, Flow: 12000, Status: 200}
	toolOnly := Record{Time: time.Now(), Provider: "deepseek", Model: "deepseek-chat", Output: 120, Millis: 9000, TTFT: 1000, Flow: 7000, Status: 200}
	sealed := visible
	sealed.ReasoningHidden = true
	for _, c := range []struct {
		name   string
		r      Record
		hidden bool
		n      int
		w      int64
	}{
		{"visible reasoning", visible, false, 1500, 15000},
		{"tool call without reasoning", toolOnly, false, 120, 9000},
		{"hidden reasoning", sealed, true, 1500, 15000},
	} {
		t.Run(c.name, func(t *testing.T) {
			var chunk rowChunk
			for i := 0; i < 80; i++ {
				chunk.add(Row{Record: c.r}, "", int64(i), false)
			}
			b := newBlockPricer(provider.Renames{}, nil, func(Record) *catalog.Price { return nil })
			defer b.release()
			for _, from := range []*rowChunk{&chunk, chunk.pack()} {
				priced := b.block(from)
				if priced.Rows[0].Flags&swapKnown == 0 {
					t.Fatal("the row wasn't priced")
				}
				row := priced.row(0)
				if n, w := row.Decode(); row.ReasoningHidden != c.hidden || n != c.n || w != c.w {
					t.Fatalf("priced row: hidden %v, %d tokens/%d ms; want %v, %d/%d", row.ReasoningHidden, n, w, c.hidden, c.n, c.w)
				}
			}
		})
	}
}

// A row's Flags bits are each its own: a reported call's own cost isn't
// read back as hidden reasoning, nor hidden reasoning as an own cost
// (rowOwnCost and rowReasoningHidden both took 256 when #1463 and #1874
// landed apart).
func TestRowFlagsApart(t *testing.T) {
	cost := 0.5
	own := Record{Time: time.Now(), Source: SourceReported, OwnCost: &cost, Provider: "deepseek", Model: "deepseek-chat", Output: 1500, Reasoning: 1400, Millis: 15000, TTFT: 1000, FirstReasoning: 1000, Flow: 12000, Status: 200}
	hidden := own
	hidden.OwnCost, hidden.ReasoningHidden = nil, true
	for _, c := range []struct {
		name   string
		r      Record
		hidden bool
		own    bool
	}{
		{"own cost", own, false, true},
		{"hidden reasoning", hidden, true, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			made := func() *rowChunk {
				var chunk rowChunk
				for i := 0; i < 80; i++ {
					chunk.add(Row{Record: c.r, Source: SourceReported}, "", int64(i), false)
				}
				return &chunk
			}
			b := newBlockPricer(provider.Renames{}, nil, func(Record) *catalog.Price { return nil })
			defer b.release()
			raw := made()
			for _, row := range []Row{raw.row(0), b.block(raw).row(0), b.block(made().pack()).row(0)} {
				if row.ReasoningHidden != c.hidden || (row.OwnCost != nil) != c.own {
					t.Fatalf("hidden %v, own cost %v; want %v, %v", row.ReasoningHidden, row.OwnCost, c.hidden, c.own)
				}
			}
		})
	}
	bits := []uint16{1, 2, 4, 8, 16, 32, swapKnown, rowHandedOn, rowReasoningHidden, rowOwnCost, rowCompactionKnown, rowCompaction, rowCredits, rowCacheUnknown}
	var all uint16
	for _, b := range bits {
		if all&b != 0 {
			t.Fatalf("Flags bit %d is taken twice", b)
		}
		all |= b
	}
}
