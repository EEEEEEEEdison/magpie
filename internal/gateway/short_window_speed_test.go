package gateway

import (
	"math"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/provider"
)

// Speed first weighs a seat by the pace usage.DecodeOf tells: this fork
// times the whole turn, so a reply whose content came in 230 ms of an
// 8.455 s turn is a held burst and gives its seat a first token, not a
// pace. A reply that streamed through its turn keeps that whole-turn pace.
func TestShortWindowAfterLongWaitNotesNoPace(t *testing.T) {
	forgetSpeeds()
	t.Cleanup(forgetSpeeds)
	c := candidate{p: provider.Provider{ID: "v"}, rest: "v", model: "gpt-6.1-sol"}
	t0 := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	noteTrySpeed(c, Try{Millis: 8455, TTFT: 8200, FirstText: 8200, Flow: 230}, 392, 0, t0)
	if ttft, tps := speedOf(c, t0); ttft != 8200 || tps != 0 {
		t.Fatalf("burst after a long wait: ttft %v, tps %v; want 8200, 0", ttft, tps)
	}
	// a streamed answer keeps its pace: 392 tokens over the whole 12.2 s turn
	noteTrySpeed(c, Try{Millis: 12200, TTFT: 8200, FirstText: 8200, Flow: 4000}, 392, 0, t0)
	if _, tps := speedOf(c, t0); math.Abs(tps-392000.0/12200) > 0.01 {
		t.Fatalf("streamed answer: tps %v; want %v", tps, 392000.0/12200)
	}
}
