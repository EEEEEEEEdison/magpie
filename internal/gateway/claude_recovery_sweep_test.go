package gateway

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/bridgesid"
	"github.com/yetone/magpie/internal/testenv"
)

// An account can wait for its quota longer than the ordinary idle hour.
// Starting another conversation after a restart must not sweep its kept
// transcript away, whether the index is still on disk or already loaded.
func TestClaudeSweepKeepsQuotaSessions(t *testing.T) {
	for _, state := range []string{"disk", "loaded", "resuming", "transferred", "running"} {
		t.Run(state, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			dir := filepath.Join(t.TempDir(), "projects", "work")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			old := time.Now().Add(-2 * idleLongest)
			for _, id := range []string{"kept", "expired"} {
				path := filepath.Join(dir, id+".jsonl")
				if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(path, old, old); err != nil {
					t.Fatal(err)
				}
			}
			entry := shelfEntry{Session: "kept", Sessions: []string{dir}, Key: "conversation", At: old, Until: time.Now().Add(time.Hour)}
			b, err := json.Marshal(shelfFile{Version: shelfFileVersion, Entries: []shelfEntry{entry}})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(shelfPath()), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(shelfPath(), b, 0o600); err != nil {
				t.Fatal(err)
			}
			bridge := newSubscriptionBridge()
			t.Cleanup(bridge.abortAll)
			if state != "disk" {
				bridge.loadShelf()
			}
			if state == "resuming" || state == "transferred" {
				saved := bridge.shelf[entry.Key]
				saved.gone = state == "transferred"
				delete(bridge.shelf, entry.Key)
				bridge.resuming = map[*savedSession]bool{saved: true}
				defer saved.timer.Stop()
			}
			if state == "running" {
				bridge.shelf[entry.Key].timer.Stop()
				delete(bridge.shelf, entry.Key)
				bridge.runs["active"] = &subscriptionRun{sessionID: entry.Session, sessions: entry.Sessions}
				defer func() {
					// Seeing the file disappear does not synchronize with
					// the sweeper's reads of the bridge's maps.
					bridge.mu.Lock()
					delete(bridge.runs, "active")
					bridge.mu.Unlock()
				}()
			}
			bridge.sweepSessions([]string{dir}, false)
			eventually(t, "the expired session wasn't swept", func() bool {
				_, err := os.Stat(filepath.Join(dir, "expired.jsonl"))
				return os.IsNotExist(err)
			})
			if _, err := os.Stat(filepath.Join(dir, "kept.jsonl")); err != nil {
				t.Fatalf("the session kept until the quota resets was swept: %v", err)
			}
		})
	}
}

func TestClaudeSweepUnknownShelfLeavesFiles(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(shelfPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{`{`, `{"version":999}`} {
		if err := os.WriteFile(shelfPath(), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, known := newSubscriptionBridge().keptSessions(); known {
			t.Fatal("unreadable shelf allowed a destructive sweep")
		}
	}
}

// A bridge id identifies the run even after a crash. Only its location
// in the bridge's own workspace makes the abandoned session sweepable.
func TestClaudeRunSessionsSweptOnlyInBridgeWorkspace(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("CLAUDE_CODE_TMPDIR", t.TempDir())
	config, tmp := filepath.Join(home, ".claude"), t.TempDir()
	project := func(work string) string { return claudeSessions(config, work)[0] }
	own := project(filepath.Join(tmp, claudeWorkName()))
	caller, other := project(filepath.Join(home, "caller")), project(filepath.Join(home, "other"))
	old := time.Now().Add(-2 * idleLongest)
	write := func(dir, sid string, at time.Time) string {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(dir, sid), 0o700); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, sid+".jsonl")
		if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, at, at); err != nil {
			t.Fatal(err)
		}
		return path
	}
	left, active, recent := bridgesid.New(), bridgesid.New(), bridgesid.New()
	if !bridgesid.Is(left) {
		t.Fatal("expected a session id recognized by this bridge")
	}
	expired := write(own, left, old)
	kept := []string{write(own, active, old), write(own, recent, time.Now())}
	for _, dir := range []string{caller, other} {
		kept = append(kept, write(dir, left, old), filepath.Join(dir, left), write(dir, newUUID(), old))
	}
	b := newSubscriptionBridge()
	b.runs["active"] = &subscriptionRun{sessionID: active, sessions: []string{own}}
	sweepBridgeLeftovers(config, tmp)
	b.sweepSessions([]string{caller}, true)
	b.sweepSessions([]string{own}, false)
	eventually(t, "expired bridge workspace session or sub-agent folder stayed", func() bool {
		_, fileErr := os.Stat(expired)
		_, dirErr := os.Stat(filepath.Join(own, left))
		return os.IsNotExist(fileErr) && os.IsNotExist(dirErr)
	})
	if _, swept := b.swept.Load(caller); swept {
		t.Error("scheduled a sweep of the caller's project")
	}
	for _, path := range kept {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected to keep %s: %v", path, err)
		}
	}
}
