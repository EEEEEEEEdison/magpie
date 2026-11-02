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
		got := st.Overview("claude-desktop", "", "", 8)
		top := got.Top["tokens"]
		if got.Count != 1 || len(top) != 1 || top[0].Tokens != (Tokens{Input: 13, Output: 3, CacheRead: 120}) {
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

// metadata 改变后，未改写或只追加的 CLI 文件也必须重新判断 entrypoint fallback。
func TestDesktopFallbackChangesWithoutTranscriptRewrite(t *testing.T) {
	for _, tc := range []struct {
		name          string
		disk, growing bool
	}{
		{"memory", false, false}, {"disk", true, false},
		{"memory continuation", false, true}, {"disk continuation", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := setupCalls(t)
			held := desktopTitlesHeld
			desktopTitlesHeld = 0
			t.Cleanup(func() { desktopTitlesHeld = held })
			path := filepath.Join(d.claude, "projects", "project", "sess1.jsonl")
			msg := func(id string, sec int) string {
				return swap(claudeMsg(id, "sol", 10, 2, 0, 0, sec), `"entrypoint":"cli"`, `"entrypoint":"claude-desktop"`)
			}
			writeLines(t, path, msg("first", 1))
			check := func(want string, count int) {
				t.Helper()
				var got []Call
				for _, source := range CallSources() {
					if source.Path == path {
						if tc.disk {
							got = ReadCallSource(source)
						} else {
							got = readCalls(file{path: source.Path, key: source.Key, agent: source.Agent, noDesktopApp: source.NoDesktopApp, size: source.Size, mod: source.Modified}).Calls
						}
					}
				}
				if len(got) != count {
					t.Fatalf("calls: %d, want %d", len(got), count)
				}
				for _, c := range got {
					if c.Agent != want {
						t.Errorf("call %s belongs to %s, want %s", c.RequestID, c.Agent, want)
					}
				}
			}
			check("claude-desktop", 1)
			meta := filepath.Join(d.desktop, "claude-code-sessions", "account", "org", "local_owned.json")
			writeLines(t, meta, `{"sessionId":"local_owned","cliSessionId":"another-session","title":""}`)
			count := 1
			if tc.growing {
				appendText(t, path, msg("second", 2)+"\n")
				count++
			}
			if tc.disk {
				Reset()
			}
			check("claude", count)
			var scans int
			scanHook = func(p string, off int64) {
				if p == path {
					scans++
				}
			}
			t.Cleanup(func() { scanHook = nil })
			check("claude", count)
			Reset()
			check("claude", count)
			if scans != 0 {
				t.Errorf("unchanged ownership rescanned the transcript %d times after restart", scans)
			}
			scanHook = nil
			if err := os.Remove(meta); err != nil {
				t.Fatal(err)
			}
			if tc.growing {
				appendText(t, path, msg("third", 3)+"\n")
				count++
			}
			if tc.disk {
				Reset()
			}
			check("claude-desktop", count)
		})
	}
}
