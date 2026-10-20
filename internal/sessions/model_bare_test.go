package sessions

import "testing"

// The Usage page's model filter and its model bars are by a model's own name
// (Bare), not the one it was sold under: one model called as
// workbuddy2api/glm-5.3-flash by one provider and as glm-5.3-flash by
// another is one row and one filter there (yetone: 另外这里模型肯定是合并过
// 的，不能出现前缀的). usage.Filter.keepsRecord matches a request the same
// way.
func TestModelsCarryTheirOwnNames(t *testing.T) {
	inZone(t, 0)
	setup(t)
	// a session's models, as the Sessions page lists them
	cc := List(0)[1]
	if m := model(cc, "claude-haiku-4-5-20251001"); m.Bare != "claude-haiku-4-5" {
		t.Fatalf("haiku's own name is %q, want claude-haiku-4-5", m.Bare)
	}
	if m := model(cc, "claude-opus-5-5"); m.Bare != "claude-opus-5-5" {
		t.Fatalf("opus's own name is %q, want the name it was called by", m.Bare)
	}
	// and a day's usage rows, as the Sessions tab reads them
	s := statsAt(0, statsNow)
	bare := map[string]string{}
	for _, d := range s.Days {
		for _, u := range d.Usage {
			bare[u.Model] = u.Bare
		}
	}
	if bare["claude-haiku-4-5-20251001"] != "claude-haiku-4-5" || bare["claude-opus-5-5"] != "claude-opus-5-5" {
		t.Fatalf("usage rows' own names: %v", bare)
	}
	// the overview behind the filter is asked for that name, and finds a
	// session that called the model either way round
	if n := s.Overview("", "claude-haiku-4-5", "", 5).Count; n != 1 {
		t.Fatalf("the model's own name found %d sessions, want 1", n)
	}
	if n := s.Overview("", "claude-haiku-4-5-20251001", "", 5).Count; n != 1 {
		t.Fatalf("the name it was called by found %d sessions, want 1", n)
	}
	if n := s.Overview("", "claude-haiku", "", 5).Count; n != 0 {
		t.Fatalf("another model found %d sessions, want 0", n)
	}
}
