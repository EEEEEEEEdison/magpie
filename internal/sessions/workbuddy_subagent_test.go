package sessions

import (
	"bytes"
	"testing"
	"time"
)

// A subagent's file names no title and no first prompt, so its state has no
// StateMore when its first usage line is read: counting it must make one
// for the reply's time (MsgAt) instead of writing through nil. Shaped as
// WorkBuddy writes a subagent's function_call under <id>/subagents/
// (isSubAgent), one usage line per messageId.
func TestWorkBuddySubagentUsage(t *testing.T) {
	inZone(t, 0)
	const line = `{"type":"function_call","timestamp":1792316747099,"sessionId":"01a14e67-2484-77df-a0d7-87ed9ea99d7a","providerData":{"messageId":"m1","model":"deepseek-v4.1-flash","requestModelId":"deepseek-v4.1-flash","isSubAgent":true,"usage":{"requests":1,"inputTokens":243,"outputTokens":39,"inputTokensDetails":[{"cached_tokens":0}]}}}`
	s := &state{}
	workbuddyLine(s, []byte(line), false)
	// the same reply's line again: its usage stands for the whole, so the
	// reply is counted once and its hour taken back before it is put again
	workbuddyLine(s, []byte(line), false)
	if m := s.Models.of("deepseek-v4.1-flash"); m != (Tokens{Input: 243, Output: 39}) {
		t.Fatalf("subagent's usage: %+v", m)
	}
	at := time.UnixMilli(1792316747099)
	if h := s.hoursOf()["deepseek-v4.1-flash"][hourKeyOf(at)]; h != (Tokens{Input: 243, Output: 39}) {
		t.Fatalf("its hour: %+v", h)
	}
	if !s.more().MsgAt.Equal(at) {
		t.Fatalf("the reply's time kept: %s", s.more().MsgAt)
	}

	// the cache keeps what tells the next read of the file a repeated line
	var b bytes.Buffer
	if err := encodeCache(&b, cacheFile{Version: cacheVersion, Zone: "z", Parsers: parseVersions, Files: map[string]*state{"f": s}}); err != nil {
		t.Fatal(err)
	}
	files, ok := decodeCache(b.Bytes(), "z")
	if !ok || files["f"] == nil {
		t.Fatalf("decode: %v %v", ok, files)
	}
	got := files["f"]
	if got.Msg != "m1" || got.MsgModel != "deepseek-v4.1-flash" || got.MsgUse != s.MsgUse || got.MsgDay != s.MsgDay || !got.more().MsgAt.Equal(at) {
		t.Fatalf("read back: %q %q %+v %q %s", got.Msg, got.MsgModel, got.MsgUse, got.MsgDay, got.more().MsgAt)
	}
	workbuddyLine(got, []byte(line), false)
	if m := got.Models.of("deepseek-v4.1-flash"); m != (Tokens{Input: 243, Output: 39}) {
		t.Fatalf("the repeated line after a reload counted twice: %+v", m)
	}
}
