package sessions

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDesktopCodeNativeIdentity(t *testing.T) {
	d := setupCalls(t)
	held := desktopTitlesHeld
	desktopTitlesHeld = 0
	t.Cleanup(func() { desktopTitlesHeld = held })
	root := filepath.Join(d.claude, "projects", "project")
	writeLines(t, filepath.Join(root, "sess1.jsonl"), claudeMsg("main", "deepseek-v4.1-flash", 10, 2, 100, 0, 1))
	writeLines(t, filepath.Join(root, "sess1", "subagents", "agent-child.jsonl"), claudeMsg("child", "deepseek-v4.1-flash", 3, 1, 20, 0, 2))
	writeLines(t, filepath.Join(root, "independent.jsonl"), swap(claudeMsg("other", "deepseek-v4.1-flash", 1, 1, 0, 0, 3), `"sessionId":"sess1"`, `"sessionId":"independent"`))
	// Warm both call caches before Desktop publishes its ownership metadata.
	for _, s := range CallSources() {
		ReadCallSource(s)
	}
	Calls(time.Time{})
	if _, ok := Get("claude:sess1"); !ok {
		t.Fatal("unowned CLI file was not listed as Claude Code")
	}
	meta := filepath.Join(d.desktop, "claude-code-sessions", "account", "org", "local_desktop.json")
	// An empty title still establishes ownership; matching titles do not.
	writeLines(t, meta, `{"sessionId":"local_desktop","cliSessionId":"sess1","title":""}`)
	for _, again := range []bool{false, true} {
		if again {
			Reset() // restored disk calls must have the same identity
		}
		st, native := StatsAndNative(0)
		if !native["claude-desktop|sess1"] || native["claude|sess1"] || !native["claude|independent"] {
			t.Fatalf("native ownership: %v", native)
		}
		if got := st.Overview("claude-desktop", "", "", 8); got.Count != 1 || got.Top["tokens_with_cache"][0].Tokens != (Tokens{Input: 13, Output: 3, CacheRead: 120}) {
			t.Fatalf("Desktop and its subagent: %+v", got)
		}
		if got := st.Overview("claude", "", "", 8); got.Count != 1 {
			t.Fatalf("independent Claude Code session: %+v", got)
		}
		if s, ok := Get("claude-desktop:sess1"); !ok || s.Gateway || s.Path == "" || s.Tokens != (Tokens{Input: 13, Output: 3, CacheRead: 120}) {
			t.Fatalf("native Desktop lookup: %+v, found=%v", s, ok)
		}
		if _, ok := Get("claude:sess1"); ok {
			t.Fatal("Desktop transcript also appeared as a separate Claude Code session")
		}
		if ids := NativeIDs(map[string]bool{"claude-desktop": true, "claude": true}); !ids["claude-desktop"]["sess1"] || ids["claude"]["sess1"] {
			t.Fatalf("management native ids: %v", ids)
		}
		for _, c := range Calls(time.Time{}) {
			want := "claude-desktop"
			if c.Session == "independent" {
				want = "claude"
			}
			if c.Agent != want {
				t.Errorf("call %s: agent=%s, want %s", c.RequestID, c.Agent, want)
			}
		}
	}
	// Removing ownership restores the CLI identity without altering the file.
	if err := os.Remove(meta); err != nil {
		t.Fatal(err)
	}
	if _, ok := Get("claude:sess1"); !ok {
		t.Fatal("deleted ownership metadata retained a Desktop identity")
	}
	for _, c := range Calls(time.Time{}) {
		if c.Agent != "claude" {
			t.Errorf("deleted metadata: call %s still belongs to %s", c.RequestID, c.Agent)
		}
	}
}
