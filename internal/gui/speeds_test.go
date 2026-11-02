package gui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/usage"
)

// /api/model-speeds says how fast each provider answered a model this
// week, keyed by the ref an agent's model picker gives the row (axuhere on
// X: one model from several providers, at very different speeds). A model
// with no timed reply, and one older than the week, say nothing.
func TestModelSpeedsByRef(t *testing.T) {
	sandboxHome(t)
	now := time.Now()
	for _, r := range []usage.Record{
		// Two replies: 400 tokens over 5 whole seconds, including both waits.
		{Time: now.Add(-time.Hour), Provider: "quick", Model: "glm-5", Output: 200, Millis: 2400, TTFT: 400, Status: 200},
		{Time: now.Add(-time.Hour), Provider: "quick", Model: "glm-5", Output: 200, Millis: 2600, TTFT: 600, Status: 200},
		// 200 tokens over the whole 13s turn: about 15.38 tok/s.
		{Time: now.Add(-time.Hour), Provider: "slow", Model: "glm-5", Output: 200, Millis: 13000, TTFT: 3000, Status: 200},
		// A whole turn without a first-token clock still has a speed: 40 tok/s.
		{Time: now.Add(-time.Hour), Provider: "plain", Model: "glm-5", Output: 200, Millis: 5000, Status: 200},
		// Held output counts for TTFT, not the speed or its sample count.
		{Time: now.Add(-time.Hour), Provider: "quick", Model: "glm-5", Output: 1000, Millis: 10000, TTFT: 500, Flow: 100, Status: 200},
		// Only its first token timed: no speed, but still a wait to report.
		{Time: now.Add(-time.Hour), Provider: "wait", Model: "glm-5", Millis: 5000, TTFT: 700, Status: 200},
		// before the week
		{Time: now.AddDate(0, 0, -9), Provider: "old", Model: "glm-5", Output: 200, Millis: 2400, TTFT: 400, Status: 200},
	} {
		usage.Append(r)
	}
	mux := http.NewServeMux()
	speedsAPI(mux)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/model-speeds", nil))
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var got map[string]speedJSON
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if q := got["quick/glm-5"]; q.TTFT != 500 || q.TPS < 79.99 || q.TPS > 80.01 || q.N != 2 {
		t.Errorf("quick/glm-5 = %+v; want 500ms, 80 tok/s of 2", q)
	}
	if s := got["slow/glm-5"]; s.TTFT != 3000 || s.TPS < 15.38 || s.TPS > 15.39 || s.N != 1 {
		t.Errorf("slow/glm-5 = %+v; want 3000ms, 15.38 tok/s of 1", s)
	}
	if s := got["plain/glm-5"]; s.TTFT != 0 || s.TPS != 40 || s.N != 1 {
		t.Errorf("plain/glm-5 = %+v; want no first-token clock, 40 tok/s of 1", s)
	}
	if s := got["wait/glm-5"]; s.TTFT != 700 || s.TPS != 0 || s.N != 1 {
		t.Errorf("wait/glm-5 = %+v; want 700ms, no speed, 1 timed reply", s)
	}
	for _, k := range []string{"old/glm-5"} {
		if s, ok := got[k]; ok {
			t.Errorf("%s says %+v; want nothing", k, s)
		}
	}
}
