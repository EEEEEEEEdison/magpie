package provider

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yetone/magpie/internal/catalog"
)

// A model's own name is what is left of its id once the maker or path a
// provider sells it under, its version, the size of its context and its
// case are off: the names one model is sold by are one model.
func TestBareModelIsTheModelItself(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"workbuddy2api/deepseek-v4.1-flash", "deepseek-v4.1-flash"},
		{"9router/workbuddy/deepseek-v4.1-flash", "deepseek-v4.1-flash"},
		{"deepseek-v4.1-flash", "deepseek-v4.1-flash"},
		{"nvtokens/GPT-6.1-sol", "gpt-6.1-sol"},
		{"muse-spark-1.3-contributor", "muse-spark-1.3-contributor"},
		{"claude-opus-5-5-20260801", "claude-opus-5-5"},
		{"us.anthropic.claude-sonnet-4-5-20250929", "claude-sonnet-4-5"},
		{"grok-4.7@500k", "grok-4.7"},
		{"claude-opus-5[1m]", "claude-opus-5"},
		// a relay that lists its display name as the id: the model whose id
		// spells the space as a dash
		{"wangwang/GPT-6 Astra", "gpt-6-astra"},
		{"kedaya/GPT-6 Astra", "gpt-6-astra"},
		{"玩具/GPT-6 Astra", "gpt-6-astra"},
		{"GPT-6  Sol", "gpt-6-sol"},
		// a dash beside the space is the same dash, not a second one
		{"浮生云算/gpt-6- sol", "gpt-6-sol"},
		{"gpt-6 - sol", "gpt-6-sol"},
		// a space a tail left behind is not a dash
		{"claude-opus-5 [1m]", "claude-opus-5"},
		// v3 is another model than v2, so it is not a version tail
		{"deepseek-v3", "deepseek-v3"},
	} {
		if got := BareModel(tc.in); got != tc.want {
			t.Errorf("BareModel(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// A maker's list is read by the model's own name too: a relay that lists
// its display name as the id sells the maker's gpt-6-sol as
// "浮生云算/gpt-6- sol", and that is the model the list prices.
func TestMakerPriceReadsTheModelsOwnName(t *testing.T) {
	priceHome(t)
	if err := os.MkdirAll(filepath.Dir(catalog.CachePath()), 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"openai":{"models":{"gpt-6-sol":{"id":"gpt-6-sol","name":"GPT-6 Sol",` +
		`"cost":{"input":2,"output":10,"cache_read":0.25,"cache_write":2.5}}}}}`
	if err := os.WriteFile(catalog.CachePath(), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog.Reset()
	for _, model := range []string{"浮生云算/gpt-6- sol", "gpt-6- sol", "gpt-6-sol"} {
		if pr, ok := MakerPrice(model); !ok || pr.Input != 2 || pr.Output != 10 {
			t.Errorf("MakerPrice(%q) = %+v %v; want the $2/$10 listed for gpt-6-sol", model, pr, ok)
		}
	}
}

// A price the user set for a model from any provider is the price of that
// model under every name the providers serving it sell it by, so one price
// covers them all. A price set for one provider's model is not spent on
// another's, and a provider's own price for it still wins.
func TestAnyPriceCoversTheModelUnderItsOtherNames(t *testing.T) {
	priceHome(t)
	priceCatalog(t)
	for _, p := range []Provider{
		{ID: "cpa", Name: "CPA", Key: "k", Chat: "https://cpa.example/v1", Models: []string{"workbuddy2api/deepseek-v4.1-flash"}},
		{ID: "opencode-go", Name: "OpenCode", Key: "k", Chat: "https://oc.example/v1", Models: []string{"deepseek-v4.1-flash"}},
	} {
		if err := Save(p); err != nil {
			t.Fatal(err)
		}
	}
	if err := SetModelPrice("*/workbuddy2api/deepseek-v4.1-flash", &catalog.Price{Input: 3, Output: 30, CacheRead: 0.3, CacheWrite: 3}); err != nil {
		t.Fatal(err)
	}
	for _, q := range []struct{ provider, model string }{
		{"cpa", "workbuddy2api/deepseek-v4.1-flash"},
		{"opencode-go", "deepseek-v4.1-flash"},
	} {
		if pr, ok := EffectivePrice(q.provider, q.model); !ok || pr.Input != 3 || pr.Output != 30 {
			t.Errorf("%s/%s: %+v %v; want the $3/$30 set for the model from any provider", q.provider, q.model, pr, ok)
		}
	}
	if err := SetModelPrice("cpa/workbuddy2api/deepseek-v4.1-flash", &catalog.Price{Input: 7, Output: 70}); err != nil {
		t.Fatal(err)
	}
	if pr, ok := EffectivePrice("opencode-go", "deepseek-v4.1-flash"); !ok || pr.Input != 3 {
		t.Errorf("opencode-go: %+v %v; want the $3 set for the model from any provider, not cpa's $7", pr, ok)
	}
	if pr, ok := EffectivePrice("cpa", "workbuddy2api/deepseek-v4.1-flash"); !ok || pr.Input != 7 {
		t.Errorf("cpa: %+v %v; want its own $7", pr, ok)
	}

	// the same for a relay that lists its display name as the id: the space
	// in it is the dash the model's own id spells, so one price covers both
	if err := SetModelPrice("*/gpt-6-astra", &catalog.Price{Input: 10, Output: 50, CacheRead: 1, CacheWrite: 12.5}); err != nil {
		t.Fatal(err)
	}
	for _, q := range []struct{ provider, model string }{
		{"wanju", "wangwang/GPT-6 Astra"},
		{"wanju", "kedaya/GPT-6 Astra"},
		{"wanju", "gpt-6-astra"},
	} {
		if pr, ok := EffectivePrice(q.provider, q.model); !ok || pr.Input != 10 || pr.Output != 50 {
			t.Errorf("%s/%s: %+v %v; want the $10/$50 set for gpt-6-astra from any provider", q.provider, q.model, pr, ok)
		}
	}

	// and a dash beside the space: "gpt-6- sol" is gpt-6-sol, not gpt-6--sol
	if err := SetModelPrice("*/gpt-6-sol", &catalog.Price{Input: 4, Output: 16}); err != nil {
		t.Fatal(err)
	}
	for _, q := range []struct{ provider, model string }{
		{"wanju", "浮生云算/gpt-6- sol"},
		{"wanju", "gpt-6-sol"},
	} {
		if pr, ok := EffectivePrice(q.provider, q.model); !ok || pr.Input != 4 || pr.Output != 16 {
			t.Errorf("%s/%s: %+v %v; want the $4/$16 set for gpt-6-sol from any provider", q.provider, q.model, pr, ok)
		}
	}
}
