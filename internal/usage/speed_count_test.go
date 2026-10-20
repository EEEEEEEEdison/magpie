package usage

import (
	"testing"
	"time"
)

// A group's speed is the whole turn's: every reply that was timed is in it,
// its tokens over the ms it took, and neither the floor nor the Codex gate
// takes one out — what a reply's turn took is what it took.
//
// The floor a group used to say the least it wrote at (Fallmonkey on Discord:
// gpt-6.1-sol showed a tok/s from the one reply of 260 that called no tool,
// beside "260 requests") tells nothing now: it was the whole turn's tokens
// over the whole turn, which is what every timed reply says itself.
func TestGroupSpeedCountsEveryTimedReply(t *testing.T) {
	at := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)
	var rows []Row
	// sol: 260 Codex calls read from its session files, all timed over their
	// 30 s turns; one of them a turn of one call, 500 tokens over 5 s
	for i := 0; i < 260; i++ {
		r := Row{Source: "log", Record: Record{Time: at, Agent: "codex", Model: "gpt-6.1-sol", Input: 9000, Output: 1500, Millis: 30000, TTFT: 800}}
		if i == 0 {
			r.Output, r.Millis, r.TTFT = 500, 5000, 1000
		}
		rows = append(rows, r)
	}
	// Claude Code: 40 calls from its session file, all timed; 10 wrote 300
	// tokens over their 20 s turns
	for i := 0; i < 40; i++ {
		r := Row{Source: "log", Record: Record{Time: at, Agent: "claude", Model: "claude-opus-5-5", Input: 5000, Output: 2000, Millis: 20000, TTFT: 900}}
		if i >= 30 {
			r.Output = 300
		}
		rows = append(rows, r)
	}
	// a failed Claude Code call isn't timed; one through the gateway is, its
	// turn's ms known though its first token wasn't timed
	rows = append(rows,
		Row{Source: "log", Record: Record{Time: at, Agent: "claude", Model: "claude-opus-5-5", Output: 2000, Millis: 20000, Error: "overloaded"}},
		Row{Record: Record{Time: at, Agent: "claude", Model: "claude-opus-5-5", Output: 2000, Millis: 20000, Status: 200}},
	)
	by := map[string]Share{}
	for _, s := range Breakdown(rows, "agent") {
		by[s.ID] = s
	}
	sol := by["codex"]
	if sol.Calls != 260 || sol.DecodeN != 260 || sol.FloorN != 0 {
		t.Fatalf("sol: %d calls, %d measured, %d floors; want 260, 260, 0", sol.Calls, sol.DecodeN, sol.FloorN)
	}
	if want := float64(500+259*1500) / ((5000.0 + 259*30000) / 1000); sol.Speed() != want {
		t.Fatalf("sol: %v tok/s, want %v", sol.Speed(), want)
	}
	cc := by["claude"]
	if cc.Calls != 42 || cc.DecodeN != 41 || cc.FloorN != 0 {
		t.Fatalf("Claude Code: %d calls, %d measured, %d floors; want 42, 41, 0", cc.Calls, cc.DecodeN, cc.FloorN)
	}
	if want := float64(30*2000+10*300+2000) / ((41 * 20000.0) / 1000); cc.Speed() != want {
		t.Fatalf("Claude Code: %v tok/s, want %v", cc.Speed(), want)
	}
	// the page's own sums count them the same way
	var sum Totals
	for _, r := range rows {
		sum.addRow(r)
	}
	if sum.DecodeN != 301 || sum.FloorN != 0 {
		t.Fatalf("sum: %+v", sum)
	}
	// a timed row tells its own speed, never a floor
	if n, ms := (Row{Source: "log", Record: Record{Agent: "claude", Output: 2000, Millis: 20000, TTFT: 4000}}).Floor(); n != 0 || ms != 0 {
		t.Fatalf("timed row's floor %d/%d", n, ms)
	}
	// records through the gateway count their measured replies too, a turn
	// whose first token wasn't timed among them
	recs := []Record{
		{Time: at, Provider: "openai", Model: "gpt-6.1-sol", Output: 1200, Millis: 15000, TTFT: 3000, Status: 200},
		{Time: at, Provider: "openai", Model: "gpt-6.1-sol", Output: 1200, Millis: 15000, Status: 200},
	}
	m := summarize(Today, at.Add(time.Hour), recs).Models[0]
	if m.Calls != 2 || m.DecodeN != 2 || m.DecodeMs != 30000 || m.DecodeOut != 2400 || m.Speed() != 80 {
		t.Fatalf("summarized: %+v", m.Totals)
	}
}

// A Codex turn of one call from its session file is timed by its whole turn
// too, with the same one-second-and-a-hundred-token gate on the smallest
// turn worth a speed (a reply of a few tokens over a few seconds reads
// anything), and a turn cut short by its flow tells none.
func TestCodexTurnTimedByItsWholeTurn(t *testing.T) {
	row := func(out int, ms, ttft, flow int64) Row {
		return Row{Source: "log", Record: Record{Agent: "codex", Model: "gpt-6.1-sol", Output: out, Millis: ms, TTFT: ttft, Flow: flow}}
	}
	for _, c := range []struct {
		name string
		r    Row
		n    int
		w    int64
	}{
		// Fallmonkey on Discord: 380 tokens 0.2 s after a 6.8 s first token
		// read 1,900 tok/s after it; over the whole turn it reads 54
		{"one push 0.2 s after a 6.8 s first token", row(380, 7000, 6800, 0), 380, 7000},
		{"1,200 tokens in one push 0.5 s after a 9.5 s first token", row(1200, 10000, 9500, 0), 1200, 10000},
		{"500 tokens over 4 s", row(500, 5000, 1000, 0), 500, 5000},
		{"1,000 tokens over 1.5 s", row(1000, 1500, 200, 0), 1000, 1500},
		{"60 tokens over 3 s: too small a turn", row(60, 3000, 1000, 0), 0, 0},
		{"1,000 tokens over 999 ms: under a second", row(1000, 999, 200, 0), 0, 0},
		// content held and let go in a tenth of the turn: DecodeOf's flow rule
		{"a turn whose content came in 1/10 of it", row(1500, 20000, 19000, 2000), 0, 0},
	} {
		if n, w := c.r.Decode(); n != c.n || w != c.w {
			t.Errorf("%s: measured %d/%d ms, want %d/%d", c.name, n, w, c.n, c.w)
		}
	}
	// the gateway's own rows and other agents' are timed the same way
	for _, r := range []Row{
		{Record: Record{Agent: "codex", Output: 380, Millis: 7000, TTFT: 6800}},
		{Source: "log", Record: Record{Agent: "claude", Output: 380, Millis: 7000, TTFT: 6800}},
	} {
		if n, w := r.Decode(); n != 380 || w != 7000 {
			t.Errorf("%s/%q: measured %d/%d ms, want 380/7000", r.Agent, r.Source, n, w)
		}
	}
	// a group sums the whole turns
	var g Totals
	g.addRow(row(380, 7000, 6800, 0))
	g.addRow(row(1200, 10000, 9500, 0))
	g.addRow(row(500, 5000, 1000, 0))
	if g.DecodeN != 3 || g.DecodeOut != 2080 || g.DecodeMs != 22000 || g.FloorN != 0 {
		t.Fatalf("group: %+v", g)
	}
	var p Part
	p.add(row(380, 7000, 6800, 0))
	if p.DecodeOut != 380 || p.DecodeMs != 7000 || p.Timed != 1 {
		t.Fatalf("part: %+v", p)
	}
}
