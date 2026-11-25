package usage

import (
	"testing"
	"time"

	"github.com/yetone/magpie/internal/sessions"
	"github.com/yetone/magpie/internal/settings"
)

// CrazyJal on X: Claude Desktop's Code tab writes the id the gateway listed
// the model by (magpie-<number>.anthropic.<Claude model>) as the model of
// each turn, and the gateway's own log counts the turn under the model that
// served it. The file's call is the same call: it showed again under the
// alias, a row for a model that is none, its cost counted twice. A call under
// a model of the agent's own stays (it never went through the gateway).
func TestAliasCallsAreTheGatewaysNotDirect(t *testing.T) {
	now := holdClock(t, time.Date(2026, 9, 30, 12, 0, 0, 0, time.Local)) // midnight never falls between a call and Today (#6486)
	tk := sessions.Tokens{Input: 120, Output: 40}
	logs := []sessions.Call{
		{Time: now.Add(-1 * time.Minute), Agent: "claude-desktop", Session: "s1", Model: "magpie-7416897221.anthropic.claude-opus-5-5", Tokens: tk},
		{Time: now.Add(-2 * time.Minute), Agent: "claude-desktop", Session: "s1", Model: "magpie-7416897221.anthropic.claude-opus-5-5[1m]", Tokens: tk},
		{Time: now.Add(-3 * time.Minute), Agent: "claude-desktop", Session: "s1", Model: "mythos-magpie-0123456789", Tokens: tk},
		{Time: now.Add(-4 * time.Minute), Agent: "claude-desktop", Session: "s1", Model: "anthropic/magpie-0123456789", Tokens: tk},
		{Time: now.Add(-5 * time.Minute), Agent: "claude", Session: "s2", Model: "claude-sonnet-5-5", Requested: "claude-magpie-0123456789", Tokens: tk},
		{Time: now.Add(-6 * time.Minute), Agent: "claude", Session: "s3", Model: "claude-opus-5-5", Tokens: tk}, // direct, never through the gateway
	}
	recs := []Record{{Time: now.Add(-time.Minute), Agent: "claude-desktop", Provider: "relay", Model: "claude-opus-5-5", Input: 120, Output: 40}}

	direct := logs[len(logs)-1]
	for _, alias := range logs[:len(logs)-1] {
		gateway := recs[0]
		gateway.Time, gateway.Agent = alias.Time, alias.Agent
		rows, sum, _, _ := ledgerWith(now.Add(-time.Hour), Filter{}, []Record{gateway}, []sessions.Call{alias, direct})
		if len(rows) != 2 || sum.Calls != 2 || sum.AllTokens() != 320 || rows[0].Provider != "relay" || rows[1].Session != direct.Session {
			t.Errorf("alias %q/%q: got %d rows, %d calls and %d tokens", alias.Model, alias.Requested, len(rows), sum.Calls, sum.AllTokens())
		}
	}
	// 只有第一条别名记录有对应的网关日志，其他四次历史调用仍需入账。
	rows, sum, _, _ := ledgerWith(now.Add(-time.Hour), Filter{}, recs, logs)
	if len(rows) != 6 || sum.Calls != 6 || sum.AllTokens() != 960 {
		t.Errorf("one duplicate among six session calls: got %d rows, %d calls and %d tokens", len(rows), sum.Calls, sum.AllTokens())
	}
	price := sessionRowPricer(settings.Load())
	local := sessionChunk(sessions.CallSource{}, logs, price, newSessionResolver(logs))
	gateway := &rowChunk{}
	gateway.add(price(recs[0], ""), "", 0, false)
	page := buildRequestPage(All, Filter{}, 0, 20, gateway, []*rowChunk{local})
	if page.Total != 6 || page.Sum.Calls != 6 || page.Sum.AllTokens() != 960 {
		t.Errorf("indexed page: got %d rows, %d calls and %d tokens", page.Total, page.Sum.Calls, page.Sum.AllTokens())
	}
	// 同时有两条可能对应的会话记录时，没有足够证据消去其中一条。
	ambiguous := logs[0]
	ambiguous.Session = "other"
	rows, _, _, _ = ledgerWith(now.Add(-time.Hour), Filter{}, recs, []sessions.Call{logs[0], ambiguous})
	if len(rows) != 3 {
		t.Errorf("ambiguous pair: got %d rows, want 3", len(rows))
	}

	old := LogCalls
	LogCalls = func(time.Time) []sessions.Call { return logs }
	t.Cleanup(func() { LogCalls = old })
	d := Direct(Today)
	if d.Calls != 1 || len(d.Models) != 1 || d.Models[0].Model != "claude-opus-5-5" {
		t.Fatalf("Direct counts the one call outside the gateway: %d calls, models %+v", d.Calls, d.Models)
	}
	if got := ownCalls(logs); len(got) != 1 || got[0].Session != "s3" {
		t.Fatalf("ownCalls: %+v", got)
	}
}

func TestMagpieAliasForms(t *testing.T) {
	for id, want := range map[string]bool{
		"magpie-7416897221.anthropic.claude-opus-5-5":   true,
		"magpie-7416897221.anthropic.claude-opus-5[1m]": true,
		"mythos-magpie-0123456789":                      true,
		"anthropic/magpie-0123456789":                   true,
		"claude-magpie-0123456789[1m]":                  true,
		"magpie":                                        false,
		"magpie-123.anthropic.claude-opus-5-5":          false, // not ten digits
		"claude-opus-5-5":                               false,
		"magpie-agent/claude-opus-5-5":                  false,
		"":                                              false,
	} {
		if got := MagpieAlias(id); got != want {
			t.Errorf("MagpieAlias(%q) = %v, want %v", id, got, want)
		}
	}
}
