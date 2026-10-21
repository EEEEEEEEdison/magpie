package usage

import (
	"testing"
	"time"

	"github.com/yetone/magpie/internal/provider"
	"github.com/yetone/magpie/internal/sessions"
)

func TestDesktopAliasesInLedger(t *testing.T) {
	pageHome(t)
	for _, p := range []provider.Provider{
		{ID: "opencode-go", Name: "OpenCode Go", Key: "test", Anthropic: "http://127.0.0.1:1", Models: []string{"deepseek-v4.1-flash"}},
		{ID: "cpa", Name: "CPA", Key: "test", Anthropic: "http://127.0.0.1:1", Models: []string{"workbuddy2api/deepseek-v4.1-flash"}},
	} {
		if err := provider.Save(p); err != nil {
			t.Fatal(err)
		}
	}
	defer provider.Hold()()
	const alias = "mythos-magpie-5487751739[1m]"
	for _, asked := range []string{alias, "anthropic/magpie-5487751739", "claude-magpie-5487751739", "magpie-5487751739.anthropic.claude-opus-4-8"} {
		r := logRecord(sessions.Call{Agent: "claude", Requested: asked, Model: "deepseek-v4.1-flash"})
		if r.Model != "opencode-go/deepseek-v4.1-flash" || r.Requested != asked || r.Served != "deepseek-v4.1-flash" || r.Provider != UnknownProvider || Swapped(r.Model, r.Served) {
			t.Errorf("alias %s: %+v", asked, r)
		}
	}
	for _, asked := range []string{"mythos-magpie-0000000000", "mythos-magpie-12", "mythos-magpie-abcdefghij", "real-model[1m]"} {
		if r := logRecord(sessions.Call{Agent: "claude", Requested: asked, Model: "different-model"}); r.Model != provider.ContextTail.ReplaceAllString(asked, "") {
			t.Errorf("unresolved/non-alias %s was guessed: %+v", asked, r)
		}
	}
	now := holdClock(t, time.Date(2026, 10, 21, 10, 45, 0, 0, time.Local))
	call := sessions.Call{Time: now.Add(-time.Minute), Agent: "claude", Session: "session", Requested: alias, Model: "deepseek-v4.1-flash",
		Tokens: sessions.Tokens{Input: 1233, Output: 464, CacheRead: 112640}, RequestID: "local"}
	duplicate := call
	duplicate.RequestID = "duplicate"
	failure := sessions.Call{Time: now.Add(-2 * time.Minute), Agent: "claude", Session: "session", Requested: "mythos-magpie-1127765277[1m]", Error: "rate_limit", ErrorText: "429 allowance exceeded"}
	logs := []sessions.Call{call, duplicate, failure}
	recs := []Record{{Time: duplicate.Time, Agent: "claude", Session: "session", RequestID: "duplicate", Provider: "opencode-go", Model: "deepseek-v4.1-flash", Requested: alias,
		Input: call.Input, Output: call.Output, CacheRead: call.CacheRead, Status: 200}}
	rows, sum, agents, providers := ledgerWith(time.Time{}, Filter{}, recs, logs)
	all := Ledgered{rows, sum, agents, providers}
	gateway, local := &rowChunk{}, &rowChunk{}
	for i, r := range recs {
		gateway.add(Row{Record: r}, "", int64(i), false)
	}
	for i, c := range logs {
		local.add(Row{Record: logRecord(c), Source: "log"}, "", int64(i), c.Error != "")
	}
	for _, page := range []RequestPage{pageFromLedger(All, Filter{}, 0, 20, all), buildRequestPage(All, Filter{}, 0, 20, gateway, []*rowChunk{local})} {
		if page.Total != 3 || page.Sum.Calls != 3 || page.Sum.Errors != 1 || page.Sum.AllTokens() != 2*114337 {
			t.Errorf("aliases must not add or drop calls/tokens: total=%d sum=%+v", page.Total, page.Sum)
		}
		if models := page.By["model"]; len(models) != 1 || models[0].ID != "deepseek-v4.1-flash" {
			t.Errorf("model ranking: %+v", models)
		}
	}
	filtered := pageFromLedger(All, Filter{Model: "deepseek-v4.1-flash", Query: "mythos-magpie"}, 0, 20, all)
	if filtered.Total != 3 {
		t.Errorf("real model and original alias filter: %d rows, want 3", filtered.Total)
	}
}
