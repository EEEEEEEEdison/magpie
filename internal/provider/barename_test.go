package provider

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/catalog"
	"github.com/yetone/magpie/internal/settings"
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

func TestCanonicalModelNormalizesVariants(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"workbuddy2api/deepseek-v4.1-flash", "deepseek-v4.1-flash"},
		{"minimax-m3.1-flash-free", "minimax-m3.1-flash"},
		{"gemini-3.8-flash-high", "gemini-3.8-flash"},
		{"gemini-3.8-flash-low", "gemini-3.8-flash"},
		{"claude-3-7-sonnet-thinking", "claude-3-7-sonnet"},
		{"k3", "kimi-k3"},
		{"kimi-k3-auto", "kimi-k3"},
		{"hy4-preview-f", "hy4-preview"},
		{"deepseek-v4.1-flash-online", "deepseek-v4.1-flash"},
		{"cpa/workbuddy2api/deepseek-v4.1-flash-free", "deepseek-v4.1-flash"},
	} {
		if got := CanonicalModel(tc.in); got != tc.want {
			t.Errorf("CanonicalModel(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// A relay that pins a model's effort in its id (gemini-3.8-flash-high,
// claude-opus-4-6-thinking) offers the levels of the model it is a variant
// of, as its price does through CanonicalModel: the id the catalog has no
// entry for falls back to the bare name the tag came off. A vendor's own
// word on a variant stands — mimo-v2.6-flash's thinking switch alone gives
// no levels, under any tag.
func TestEffortsOfARunningTagInTheModelName(t *testing.T) {
	priceHome(t)
	if err := os.MkdirAll(filepath.Dir(catalog.CachePath()), 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{
	  "google": {"models": {"gemini-3.8-flash": {"id":"gemini-3.8-flash","reasoning":true,
	    "reasoning_options":[{"type":"effort","values":["low","medium","high"]}]}}},
	  "anthropic": {"models": {"claude-opus-4-6": {"id":"claude-opus-4-6","reasoning":true,
	    "reasoning_options":[{"type":"effort","values":["low","medium","high","max"]}]}}},
	  "xiaomi": {"models": {"mimo-v2.6-flash": {"id":"mimo-v2.6-flash","reasoning":true,
	    "reasoning_options":[{"type":"toggle"}]}}}
	}`
	if err := os.WriteFile(catalog.CachePath(), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog.Reset()
	t.Cleanup(catalog.Reset)

	for _, tc := range []struct{ id, want string }{
		{"gemini-3.8-flash", "low,medium,high"},
		{"gemini-3.8-flash-high", "low,medium,high"},
		{"gemini-3.8-flash-low", "low,medium,high"},
		{"cpa/gemini-3.8-flash-high", "low,medium,high"},
		{"claude-opus-4-6-thinking", "low,medium,high,max"},
		// the vendor's own word on the variant is not overridden by a tag
		{"mimo-v2.6-flash-free", ""},
		{"mimo-v2.6-flash", ""},
		{"unknown-model-high", ""},
	} {
		if got := strings.Join(borrowedEfforts(tc.id), ","); got != tc.want {
			t.Errorf("borrowedEfforts(%q) = %q, want %q", tc.id, got, tc.want)
		}
	}

	for _, tc := range []struct {
		id   string
		want bool
	}{
		{"gemini-3.8-flash-high", true},
		{"claude-opus-4-6-thinking", true},
		{"mimo-v2.6-flash-free", true},
		{"unknown-model-high", false},
	} {
		if got := thinksInCatalog(tc.id); got != tc.want {
			t.Errorf("thinksInCatalog(%q) = %v, want %v", tc.id, got, tc.want)
		}
	}

	// what the agents' lists carry: the levels and the thinking flag both
	e := entryFor(Provider{ID: "cpa", Chat: "https://cpa.example/v1"}, catalog.Model{ID: "gemini-3.8-flash-high"}, settings.Settings{})
	if strings.Join(e.Efforts, ",") != "low,medium,high" || !e.Reasoning {
		t.Errorf("entryFor(gemini-3.8-flash-high): efforts %v reasoning %v; want low,medium,high true", e.Efforts, e.Reasoning)
	}
}

// When the user has configured a price under one specific provider's relay name
// (e.g. cpa/workbuddy2api/glm-5.3-flash) and never set an all-provider wildcard
// (*/...), a call via another provider (opencode-go/glm-5.3-flash) or using an
// operational variant (glm-5.3-flash-free) borrows that custom price rather
// than remaining unpriced.
func TestCrossProviderPriceBorrowing(t *testing.T) {
	priceHome(t)
	priceCatalog(t)
	for _, p := range []Provider{
		{ID: "cpa", Name: "CPA", Key: "k", Chat: "https://cpa.example/v1", Models: []string{"workbuddy2api/glm-5.3-flash"}},
		{ID: "opencode-go", Name: "OpenCode", Key: "k", Chat: "https://oc.example/v1", Models: []string{"glm-5.3-flash"}},
		{ID: "direct", Name: "Direct", Key: "k", Chat: "https://direct.example/v1", Models: []string{"glm-5.3-flash-free"}},
	} {
		if err := Save(p); err != nil {
			t.Fatal(err)
		}
	}

	// User only priced cpa's relay naming; no wildcard */... is configured
	if err := SetModelPrice("cpa/workbuddy2api/glm-5.3-flash", &catalog.Price{Input: 0.15, Output: 0.5, CacheRead: 0.03}); err != nil {
		t.Fatal(err)
	}

	// Opencode-go's clean model name borrows cpa's custom tariff
	pr, ok := EffectivePrice("opencode-go", "glm-5.3-flash")
	if !ok || pr.Input != 0.15 || pr.Output != 0.5 {
		t.Fatalf("opencode-go/glm-5.3-flash: %+v %v; want borrowed $0.15/$0.50 from cpa", pr, ok)
	}

	// Direct provider's free variant also borrows it via CanonicalModel
	prVar, okVar := EffectivePrice("direct", "glm-5.3-flash-free")
	if !okVar || prVar.Input != 0.15 || prVar.Output != 0.5 {
		t.Fatalf("direct/glm-5.3-flash-free: %+v %v; want borrowed $0.15/$0.50 from cpa", prVar, okVar)
	}

	// But an explicit wildcard still overrides borrowed provider prices
	if err := SetModelPrice("*/glm-5.3-flash", &catalog.Price{Input: 0.20, Output: 0.80}); err != nil {
		t.Fatal(err)
	}
	prWild, okWild := EffectivePrice("opencode-go", "glm-5.3-flash")
	if !okWild || prWild.Input != 0.20 || prWild.Output != 0.80 {
		t.Fatalf("opencode-go/glm-5.3-flash: %+v %v; want wildcard $0.20/$0.80 over borrowed cpa price", prWild, okWild)
	}
}

// A remote magpie is never lent a price set here for another provider's model
// of the same name: its own list is what that magpie counts the call at, and
// one from before it told prices tells nothing, which leaves the model to
// models.dev (remote_magpie.go) rather than to a tariff of some other
// provider. TestRemoteMagpiePrices is this from the gateway's side.
func TestRemoteMagpieDoesNotBorrowAnotherProvidersPrice(t *testing.T) {
	priceHome(t)
	priceCatalog(t)
	for _, p := range []Provider{
		{ID: "cpa", Name: "CPA", Key: "k", Chat: "https://cpa.example/v1", Models: []string{"glm-5.3-flash"}},
		{ID: "attic", Name: "Attic", Key: "k", Chat: "https://attic.example", Preset: RemoteMagpiePreset},
	} {
		if err := Save(p); err != nil {
			t.Fatal(err)
		}
	}
	if err := SetModelPrice("cpa/glm-5.3-flash", &catalog.Price{Input: 0.15, Output: 0.5}); err != nil {
		t.Fatal(err)
	}

	// a plain provider still borrows it
	if pr, ok := EffectivePrice("cpa", "glm-5.3-flash"); !ok || pr.Input != 0.15 {
		t.Fatalf("cpa/glm-5.3-flash: %+v %v; want its own $0.15", pr, ok)
	}
	// the remote's list said nothing about the model, so nothing prices it here
	if pr, ok := EffectivePrice("attic", "glm-5.3-flash"); ok {
		t.Fatalf("attic/glm-5.3-flash: %+v, priced by cpa's tariff; want no price at all", pr)
	}
}

// Foundation model makers listed in models.dev without dedicated presets
// (Meta) or whose vendor presets host third-party models (Tencent Tokenhub)
// are consulted by MakerPrice and CanonicalModel.
func TestMakerPriceForMetaAndTencent(t *testing.T) {
	priceHome(t)
	if err := os.MkdirAll(filepath.Dir(catalog.CachePath()), 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{
		"tencent-tokenhub": {
			"models": {
				"hy4-preview": {"id": "hy4-preview", "cost": {"input": 0.834, "output": 2.501, "cache_read": 0.042}}
			}
		},
		"meta": {
			"models": {
				"muse-spark-1.3-contributor": {"id": "muse-spark-1.3-contributor", "cost": {"input": 0.1, "output": 0.2, "cache_read": 0.002}}
			}
		}
	}`
	if err := os.WriteFile(catalog.CachePath(), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog.Reset()
	t.Cleanup(catalog.Reset)

	// hy4-preview-f canonicalizes to hy4-preview and resolves against tencent-tokenhub
	prHy4, okHy4 := MakerPrice("hy4-preview-f")
	if !okHy4 || prHy4.Input != 0.834 || prHy4.Output != 2.501 {
		t.Errorf("MakerPrice(hy4-preview-f) = %+v, ok=%v; want 0.834/2.501 from tencent-tokenhub", prHy4, okHy4)
	}

	// muse-spark-1.3-contributor resolves against meta
	prMuse, okMuse := MakerPrice("muse-spark-1.3-contributor")
	if !okMuse || prMuse.Input != 0.1 || prMuse.Output != 0.2 {
		t.Errorf("MakerPrice(muse-spark-1.3-contributor) = %+v, ok=%v; want 0.1/0.2 from meta", prMuse, okMuse)
	}

	// Session file rows with unknown providers resolve through MakerPrice
	s := settings.Load()
	epHy4, okEpHy4 := EffectivePriceIn(s, "session-unknown", "hy4-preview-f")
	if !okEpHy4 || epHy4.Input != 0.834 {
		t.Errorf("EffectivePriceIn(session-unknown, hy4-preview-f) = %+v, ok=%v; want 0.834", epHy4, okEpHy4)
	}
	epMuse, okEpMuse := EffectivePriceIn(s, "session-unknown", "muse-spark-1.3-contributor")
	if !okEpMuse || epMuse.Input != 0.1 {
		t.Errorf("EffectivePriceIn(session-unknown, muse-spark-1.3-contributor) = %+v, ok=%v; want 0.1", epMuse, okEpMuse)
	}
}

// Free promotional variants or aggregator channels marked as cost 0 (e.g.
// minimax-m3.1-flash-free or mimo-v2.5-free) resolve to their maker's official
// API prices rather than remaining $0.000, while anonymous models explicitly
// priced zero by the user (omen-alpha, union-alpha) retain $0.
func TestFreeChannelVariantsResolveToMakerOfficialPrice(t *testing.T) {
	priceHome(t)
	if err := os.MkdirAll(filepath.Dir(catalog.CachePath()), 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{
		"opencode-go": {
			"models": {
				"minimax-m3.1-flash-free": {"id": "minimax-m3.1-flash-free", "cost": {"input": 0, "output": 0, "cache_read": 0}},
				"mimo-v2.5-free": {"id": "mimo-v2.5-free", "cost": {"input": 0, "output": 0, "cache_read": 0}}
			}
		},
		"minimax": {
			"models": {
				"MiniMax-M3.1-Flash": {"id": "MiniMax-M3.1-Flash", "cost": {"input": 0.15, "output": 0.6, "cache_read": 0.015}}
			}
		},
		"xiaomi": {
			"models": {
				"mimo-v2.5": {"id": "mimo-v2.5", "cost": {"input": 0.14, "output": 0.28, "cache_read": 0.0028}}
			}
		}
	}`
	if err := os.WriteFile(catalog.CachePath(), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog.Reset()
	t.Cleanup(catalog.Reset)

	if err := Save(Provider{ID: "opencode-go", Name: "OpenCode", Key: "k", Chat: "https://oc.example/v1", Catalog: "opencode-go",
		Models: []string{"minimax-m3.1-flash-free", "mimo-v2.5-free"}}); err != nil {
		t.Fatal(err)
	}

	s := settings.Load()
	// An explicit zero set by the user for an anonymous test model stays zero
	s.ModelPrices = map[string]settings.ModelPrice{
		"*/omen-alpha": {Input: new(0.0), Output: new(0.0), CacheRead: new(0.0), CacheWrite: new(0.0)},
	}

	// 1. minimax-m3.1-flash-free must not be zero; must be MiniMax official API price $0.15/$0.60
	prMiniMax, okMiniMax := EffectivePriceIn(s, "opencode-go", "minimax-m3.1-flash-free")
	if !okMiniMax || prMiniMax.Input != 0.15 || prMiniMax.Output != 0.6 {
		t.Errorf("EffectivePriceIn(minimax-m3.1-flash-free) = %+v, ok=%v; want official API price 0.15/0.60", prMiniMax, okMiniMax)
	}

	// 2. mimo-v2.5-free must not be zero; must be Xiaomi official API price $0.14/$0.28
	prMimo, okMimo := EffectivePriceIn(s, "opencode-go", "mimo-v2.5-free")
	if !okMimo || prMimo.Input != 0.14 || prMimo.Output != 0.28 {
		t.Errorf("EffectivePriceIn(mimo-v2.5-free) = %+v, ok=%v; want official API price 0.14/0.28", prMimo, okMimo)
	}

	// 3. User explicit zero on anonymous model stays zero
	prOmen, okOmen := EffectivePriceIn(s, "opencode-go", "omen-alpha")
	if !okOmen || prOmen.Input != 0 || prOmen.Output != 0 {
		t.Errorf("EffectivePriceIn(omen-alpha) = %+v, ok=%v; want 0", prOmen, okOmen)
	}
}

// A model a plugin lists carries the plugin's own provider id, which the
// catalog doesn't know: it gave no word on levels, so the model's own are
// read, as for a relay added by its endpoint. One a catalog provider lists
// without levels still takes none.
func TestEffortsOfAModelAPluginLists(t *testing.T) {
	priceHome(t)
	if err := os.MkdirAll(filepath.Dir(catalog.CachePath()), 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{
	  "opencode": {"models": {
	    "deepseek-v4.1-flash-free": {"id":"deepseek-v4.1-flash-free","reasoning":true,
	      "reasoning_options":[{"type":"effort","values":["low","high","max"]}]}}},
	  "xiaomi": {"models": {
	    "mimo-v2.6-flash": {"id":"mimo-v2.6-flash","reasoning":true,"reasoning_options":[{"type":"toggle"}]}}},
	  "azure": {"models": {
	    "deepseek-v4.1-flash": {"id":"deepseek-v4.1-flash","reasoning":true,"reasoning_options":[]}}}
	}`
	if err := os.WriteFile(catalog.CachePath(), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog.Reset()
	t.Cleanup(catalog.Reset)

	for _, tc := range []struct {
		id, provider, want string
	}{
		// a plugin's id the catalog doesn't know: the model's levels
		{"deepseek-v4.1-flash-free", "opencode-zen-free", "low,high,max"},
		// a vendor's own word stands: xiaomi's switch alone gives no levels
		{"mimo-v2.6-flash-free", "opencode-zen-free", ""},
		// a catalog provider listing the model without levels says it takes none
		{"deepseek-v4.1-flash", "azure", ""},
	} {
		m := catalog.Model{ID: tc.id, Provider: tc.provider}
		if got := strings.Join(effortsOf(m), ","); got != tc.want {
			t.Errorf("effortsOf(%s at %s) = %q, want %q", tc.id, tc.provider, got, tc.want)
		}
	}
}

// A zero a relay or a plugin wrote for a running variant is read at the
// model's maker's price; an id that is its own name, or one no maker lists,
// is not.
func TestMakerPriceOfARunningVariant(t *testing.T) {
	priceHome(t)
	if err := os.MkdirAll(filepath.Dir(catalog.CachePath()), 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{
	  "minimax": {"models": {"minimax-m3.1-flash": {"id":"minimax-m3.1-flash","cost":{"input":0.15,"output":0.6}}}},
	  "xiaomi": {"models": {"mimo-v2.6-flash": {"id":"mimo-v2.6-flash","cost":{"input":0.14,"output":0.28}}}},
	  "opencode": {"models": {"mimo-v2.6-flash-free": {"id":"mimo-v2.6-flash-free","cost":{"input":0,"output":0}}}}
	}`
	if err := os.WriteFile(catalog.CachePath(), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	catalog.Reset()
	t.Cleanup(catalog.Reset)

	for _, tc := range []struct {
		id     string
		input  float64
		output float64
		works  bool
	}{
		{"minimax-m3.1-flash-free", 0.15, 0.6, true},
		{"mimo-v2.6-flash-free", 0.14, 0.28, true},
		// its own name, not a variant; and one no maker lists
		{"big-pickle", 0, 0, false},
		{"deepseek-v4.1-flash-free", 0, 0, false},
	} {
		pr, ok := makerPriceOfVariant(tc.id)
		if ok != tc.works || pr.Input != tc.input || pr.Output != tc.output {
			t.Errorf("makerPriceOfVariant(%q) = %+v, %v; want %v/%v, %v", tc.id, pr, ok, tc.input, tc.output, tc.works)
		}
	}
}
