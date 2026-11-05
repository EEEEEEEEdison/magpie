package usage

import "testing"

// This fork times a reply over its whole turn. A reply whose content came in
// a small part of that turn was held and let go in a burst and still tells no
// speed; the wait for its first token is otherwise part of the speed.
func TestShortWindowAfterLongWaitTellsNoSpeed(t *testing.T) {
	for _, c := range []struct {
		name string
		r    Record
		n    int
		w    int64
	}{
		// the report: 392 tokens over 8.455 s, its content let go in 230 ms
		{"the report", Record{Output: 392, Millis: 8455, TTFT: 8200, FirstText: 8200, Flow: 230, Status: 200}, 0, 0},
		// a row of the owner's own ledger, its content in 172 ms of 10.325 s
		{"reasoned, answer after a long think", Record{Output: 418, Reasoning: 42, Millis: 10325, TTFT: 9949, FirstText: 10010, Flow: 172, ReasoningHidden: true, Status: 200}, 0, 0},
		// the flow unknown: the whole turn is all there is
		{"no flow kept", Record{Output: 392, Millis: 8455, TTFT: 8200, Status: 200}, 392, 8455},
		// a fast vendor: 500 tokens over 550 ms, its content in 250 ms
		{"fast vendor, short wait", Record{Output: 500, Millis: 550, TTFT: 300, Flow: 250, Status: 200}, 500, 550},
		// its content in 999 ms of a 4.995 s turn: a held burst
		{"wait four windows", Record{Output: 200, Millis: 4995, TTFT: 3996, Flow: 999, Status: 200}, 0, 0},
		{"wait past four windows", Record{Output: 200, Millis: 4996, TTFT: 3997, Flow: 999, Status: 200}, 0, 0},
		// a second of content in a 9.2 s turn: a held burst
		{"a second after a long wait", Record{Output: 100, Millis: 9200, TTFT: 8200, Flow: 1000, Status: 200}, 0, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			if n, w := c.r.Decode(); n != c.n || w != c.w {
				t.Fatalf("speed counted %d tokens/%d ms; want %d/%d", n, w, c.n, c.w)
			}
		})
	}
	// the report's row adds its first token to a period's totals, not a speed
	var tot Totals
	tot.addRow(Row{Record: Record{Output: 392, Millis: 8455, TTFT: 8200, FirstText: 8200, Flow: 230, Status: 200}})
	if tot.Timed != 1 || tot.DecodeN != 0 || tot.Speed() != 0 {
		t.Fatalf("totals: timed %d, decode_n %d, speed %v", tot.Timed, tot.DecodeN, tot.Speed())
	}
}
