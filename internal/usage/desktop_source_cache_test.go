package usage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/sessions"
)

// 来源的 Desktop fallback 改变后，内存块和磁盘块都不能沿用旧身份。
func TestRequestBlocksReclassifyDesktopFallback(t *testing.T) {
	for _, disk := range []bool{false, true} {
		t.Run(fmt.Sprintf("disk=%v", disk), func(t *testing.T) {
			pageHome(t)
			now := holdClock(t, time.Now())
			path := filepath.Join(sessions.ClaudeDir(), "projects", "project", "fallback.jsonl")
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			body := fmt.Sprintf(`{"type":"assistant","uuid":"first","timestamp":%q,"sessionId":"fallback","entrypoint":"claude-desktop-3p","message":{"id":"first","role":"assistant","model":"m","content":[{"type":"text","text":"done"}],"usage":{"input_tokens":10,"output_tokens":2}}}`+"\n", now.Format(time.RFC3339Nano))
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			st := readRequestState(now)
			if len(st.sources) != 1 || st.sources[0].Path != path {
				t.Fatalf("sources: %+v", st.sources)
			}
			least := requestDiskLeast
			requestDiskLeast = 1
			t.Cleanup(func() { requestDiskLeast = least })
			idx := &requestIndex{root: st.root, meta: st.meta, chunks: map[string]*rowChunk{}}
			var reads int
			read := func(s sessions.CallSource) []sessions.Call {
				reads++
				return sessions.ReadCallSource(s)
			}
			var serial uint64
			check := func(noDesktop bool, want string, wantReads int) {
				t.Helper()
				st.sources[0].NoDesktopApp = noDesktop
				reads = 0
				b := readRequestBlocks(idx, st, All, now, time.Time{}, read, false)
				defer b.priced.release()
				page := buildRequestBlocks(All, Filter{}, 0, 20, now, b.gateways, b.chunks, b.others, nil)
				if len(page.Rows) != 1 || page.Rows[0].Agent != want || page.Sum.Input != 10 || page.Sum.Output != 2 {
					t.Errorf("NoDesktopApp=%v: rows=%+v sum=%+v, want one %s call with 10/2 tokens", noDesktop, page.Rows, page.Sum, want)
				}
				if reads != wantReads {
					t.Errorf("NoDesktopApp=%v: read %d sources, want %d", noDesktop, reads, wantReads)
				}
				c := idx.chunks[path]
				if wantReads != 0 && serial != 0 && c.serial == serial {
					t.Error("changed identity retained the derived page/heatmap serial")
				}
				serial = c.serial
				if disk {
					if err := toDisk(st.root, st.meta, idx.chunks, b.on); err != nil {
						t.Fatal(err)
					}
					idx.chunks = map[string]*rowChunk{}
					sessions.Reset()
				}
			}
			check(false, "claude-desktop", 1)
			check(false, "claude-desktop", 0)
			check(true, "claude", 1)
			check(true, "claude", 0)
			check(false, "claude-desktop", 1)
			check(false, "claude-desktop", 0)
		})
	}
}

// 已结算的统计也必须拒绝旧身份的磁盘结果。
func TestSettledDaysRejectChangedDesktopFallback(t *testing.T) {
	pageHome(t)
	at := time.Now()
	c := &rowChunk{Source: sessions.CallSource{Path: "/s/fallback.jsonl", Key: "claude:fallback", Agent: "claude", Size: 10, Modified: at}, Count: 1, Latest: at, serial: 1}
	key := func() string {
		k, ok := heatSettledKey(heatDiskRef{root: "r", meta: "m"}, at.Add(time.Hour), time.Time{}, Filter{Agent: "claude-desktop"}, nil, [3][]*rowChunk{nil, {c}}, heatSum{})
		if !ok {
			t.Fatal("settled rows have no key")
		}
		return k
	}
	if err := heatDiskWrite(key(), at.Add(time.Hour), time.Time{}, []Day{{Date: at.Format(time.DateOnly), Calls: 1, Tokens: 12}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := heatDiskRead(key()); !ok {
		t.Fatal("unchanged rows lost their settled days")
	}
	c.Source.NoDesktopApp = true
	if days, ok := heatDiskRead(key()); ok {
		t.Errorf("CLI-only source reused Desktop's settled days: %+v", days)
	}
}

// 两个查询看到不同的来源身份时，不能等待并复用对方的旧身份块。
func TestSharedBlocksKeepTheirDesktopFallback(t *testing.T) {
	pageHome(t)
	now := holdClock(t, time.Now())
	st := readRequestState(now)
	st.sources = []sessions.CallSource{{Path: "/s/fallback.jsonl", Key: "claude:fallback", Agent: "claude", Size: 10, Modified: now}}
	newer := st
	newer.sources = append([]sessions.CallSource(nil), st.sources...)
	newer.sources[0].NoDesktopApp = true
	inside, release := make(chan struct{}), make(chan struct{})
	joined := make(chan struct{}, 1)
	buildJoined = func() { joined <- struct{}{} }
	t.Cleanup(func() { buildJoined = nil })
	read := func(s sessions.CallSource) []sessions.Call {
		agent := "claude-desktop"
		if s.NoDesktopApp {
			agent = "claude"
		}
		return []sessions.Call{{Time: now, Agent: agent, Session: "fallback", Model: "m", Tokens: sessions.Tokens{Input: 10, Output: 2}}}
	}
	first, second := make(chan requestBlocks), make(chan requestBlocks)
	index := func() *requestIndex { return &requestIndex{chunks: map[string]*rowChunk{}} }
	go func() {
		first <- readRequestBlocks(index(), st, All, now, time.Time{}, func(s sessions.CallSource) []sessions.Call {
			close(inside)
			<-release
			return read(s)
		}, true)
	}()
	<-inside
	go func() { second <- readRequestBlocks(index(), newer, All, now, time.Time{}, read, true) }()
	var b requestBlocks
	select {
	case <-joined:
		close(release)
		b = <-second
	case b = <-second:
		close(release)
	}
	a := <-first
	defer a.priced.release()
	defer b.priced.release()
	for i, tc := range []struct {
		blocks requestBlocks
		agent  string
	}{{a, "claude-desktop"}, {b, "claude"}} {
		page := buildRequestBlocks(All, Filter{}, 0, 20, now, tc.blocks.gateways, tc.blocks.chunks, nil, nil)
		if len(page.Rows) != 1 || page.Rows[0].Agent != tc.agent {
			t.Errorf("query %d: rows %+v, want %s", i, page.Rows, tc.agent)
		}
	}
}

// 保存的来源必须保留 fallback 约束，不能只给旧行贴上新来源。
func TestDiskEntryKeepsDesktopFallback(t *testing.T) {
	source := sessions.CallSource{Path: "/s/fallback.jsonl", Key: "claude:fallback", Agent: "claude", NoDesktopApp: true, Size: 10, Modified: time.Unix(1790000000, 0)}
	got, err := readDiskEntry(diskEntry(nil, &rowChunk{Source: source, Archive: []byte{1}}))
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != source {
		t.Errorf("restored source %+v, want %+v", got.Source, source)
	}
}
