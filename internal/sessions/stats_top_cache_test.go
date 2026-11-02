package sessions

import (
	"slices"
	"testing"
)

// 上游仅按输入输出选取短名单，含缓存时由窗口重排这份名单。
// 缓存独多的会话不另建后端榜单，费用与活跃时长仍各自排名。
func TestOverviewTopTokensMatchesUpstreamContract(t *testing.T) {
	st := Stats{Sessions: []Summary{
		{Key: "input", Agent: "claude", Cwd: "/work/a", Models: []string{"model"}, Tokens: Tokens{Input: 900, Output: 100}, Cost: 3, Active: 30},
		{Key: "read", Agent: "claude", Cwd: "/work/a", Models: []string{"model"}, Tokens: Tokens{Input: 10, CacheRead: 2000}, Cost: 2, Active: 20},
		{Key: "write", Agent: "claude", Cwd: "/work/a", Models: []string{"model"}, Tokens: Tokens{CacheWrite: 1500, CacheWrite1h: 1500}, Cost: 1, Active: 10},
		{Key: "other", Agent: "codex", Cwd: "/work/b", Models: []string{"other"}, Tokens: Tokens{CacheRead: 9000}},
		{Key: "empty", Agent: "claude", Cwd: "/work/a", Models: []string{"model"}},
	}}
	keys := func(ss []Summary) []string {
		out := make([]string, len(ss))
		for i, s := range ss {
			out[i] = s.Key
		}
		return out
	}
	for _, filter := range [][3]string{{"claude", "", ""}, {"", "model", ""}, {"", "", "/work/a"}} {
		o := st.Overview(filter[0], filter[1], filter[2], 8)
		for by, want := range map[string][]string{
			"tokens": {"input", "read"},
			"cost":   {"input", "read", "write"},
			"active": {"input", "read", "write"},
		} {
			if got := keys(o.Top[by]); !slices.Equal(got, want) {
				t.Errorf("filter %v: top %s = %v, want %v", filter, by, got, want)
			}
		}
		if len(o.Top) != 3 {
			t.Errorf("filter %v: top has %d rankings, want upstream's three", filter, len(o.Top))
		}
	}
	if got := keys(st.Overview("", "", "", 1).Top["tokens"]); !slices.Equal(got, []string{"input"}) {
		t.Errorf("top 1 without cache = %v, want [input]", got)
	}
	if got := st.Overview("missing", "", "", 8).Top["tokens"]; len(got) != 0 {
		t.Errorf("empty filter: %v", got)
	}
}
