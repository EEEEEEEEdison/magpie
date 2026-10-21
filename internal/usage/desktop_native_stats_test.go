package usage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/sessions"
)

func TestDesktopNativeStatsExcludeGatewayDuplicate(t *testing.T) {
	pageHome(t)
	now := time.Now()
	holdClock(t, now)
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	line := func(id string, in, out, read int) string {
		return fmt.Sprintf(`{"type":"assistant","uuid":%q,"timestamp":%q,"sessionId":%q,"cwd":"/work","entrypoint":"claude-desktop-3p","message":{"id":%q,"role":"assistant","model":"m","content":[{"type":"text","text":"done"}],"usage":{"input_tokens":%d,"output_tokens":%d,"cache_read_input_tokens":%d}}}`, id, now.Format(time.RFC3339Nano), id, id, in, out, read)
	}
	root := filepath.Join(sessions.ClaudeDir(), "projects", "project")
	write(filepath.Join(root, "desktop.jsonl"), line("desktop", 10, 2, 100))
	write(filepath.Join(root, "log-only.jsonl"), line("log-only", 5, 0, 0))
	write(filepath.Join(root, "independent.jsonl"), line("independent", 1, 1, 0))
	// One gateway call is the native Desktop turn; another has the same ID
	// as an unrelated CLI session, but no Desktop metadata owns that file.
	Append(Record{Time: now, Agent: "claude-desktop", Session: "desktop", Provider: "p", Model: "m", Input: 10, Output: 2, CacheRead: 100, Status: 200})
	Append(Record{Time: now, Agent: "claude-desktop", Session: "independent", Provider: "p", Model: "m", Input: 7, Status: 200})
	// Warm request chunks and parser continuations under the original owner.
	before := QueryPage(All, Filter{}, 0, 20)
	meta := filepath.Join(sessions.DesktopDataDirs()[0], "claude-code-sessions", "account", "org", "local_desktop.json")
	write(meta, `{"sessionId":"local_desktop","cliSessionId":"desktop","priorCliSessionIds":["log-only"],"title":"Same title"}`)
	// Reset drops metadata's one-second memo, but leaves the usage page's
	// packed chunks alive: changed ownership must invalidate those too.
	sessions.Reset()
	for _, days := range []int{0, 1, 7} {
		st, native := sessions.StatsAndNative(days)
		if !native["claude-desktop|desktop"] || native["claude-desktop|independent"] {
			t.Fatalf("native identities: %v", native)
		}
		var since time.Time
		if days > 0 {
			since, _ = time.ParseInLocation(time.DateOnly, st.From, time.Local)
		}
		gateway, gd := GatewaySessionWindowExcept(since, native)
		if len(gateway) != 1 || gateway[0].ID != "independent" {
			t.Fatalf("days=%d: duplicated Desktop or lost unrelated gateway: %+v", days, gateway)
		}
		daily := make([]sessions.ExternalDayUsage, 0, len(gd))
		for _, d := range gd {
			daily = append(daily, sessions.ExternalDayUsage{Date: d.Date, Agent: d.Agent, Model: d.Model, Tokens: d.Tokens, Cost: d.Cost, Priced: d.Priced})
		}
		combined := sessions.MergeExternalStats(st, daily, nil)
		tokens := 0
		for _, d := range combined.Days {
			for _, u := range d.Usage {
				tokens += u.Input + u.Output + u.CacheRead + u.CacheWrite
			}
		}
		if tokens != 126 {
			t.Errorf("days=%d: combined tokens=%d, want 126 (112 + 5 + 2 + 7)", days, tokens)
		}
	}
	after := QueryPage(All, Filter{}, 0, 20)
	if before.Sum.AllTokens() != after.Sum.AllTokens() || after.Sum.AllTokens() != 126 || after.Total != 4 {
		t.Fatalf("request deduplication changed tokens: before=%+v after=%+v", before.Sum, after.Sum)
	}
	found := false
	for _, r := range after.Rows {
		if r.Source == "log" && r.Session == "log-only" {
			found = true
			if r.Agent != "claude-desktop" {
				t.Errorf("stale cached request agent: %+v", r)
			}
		}
	}
	if !found {
		t.Fatal("local-only Desktop request disappeared")
	}
}
