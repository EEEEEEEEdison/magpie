package sessions

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/catalog"
	"github.com/yetone/magpie/internal/settings"
)

func sessionPriceFixture(t *testing.T) settings.Settings {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	if err := os.MkdirAll(filepath.Dir(catalog.CachePath()), 0700); err != nil {
		t.Fatal(err)
	}
	data := `{
"google":{"id":"google","models":{
 "gemini-3.7-flash":{"id":"gemini-3.7-flash","cost":{"input":0.75,"output":3.75}},
 "gemini-3.8-flash":{"id":"gemini-3.8-flash","cost":{"input":0.75,"output":3.75}},
 "gemini-9-flash-high":{"id":"gemini-9-flash-high","cost":{"input":9}},
 "gemini-9-flash":{"id":"gemini-9-flash","cost":{"input":1}}}},
"xiaomi":{"id":"xiaomi","models":{"mimo-v2.6-flash":{"id":"mimo-v2.6-flash","cost":{"input":0.14,"output":0.28}}}},
"meta":{"id":"meta","models":{
 "muse-spark-1.2-contributor":{"id":"muse-spark-1.2-contributor","cost":{"input":0.1,"output":0.2}},
 "muse-spark-1.3-contributor":{"id":"muse-spark-1.3-contributor","cost":{"input":0.1,"output":0.2}}}},
"stepfun-ai":{"id":"stepfun-ai","models":{"step-5-preview":{"id":"step-5-preview","cost":{"input":1,"output":2.7}}}},
"moonshotai":{"id":"moonshotai","models":{"kimi-k3":{"id":"kimi-k3","cost":{"input":3,"output":15}}}},
"tencent-tokenhub":{"id":"tencent-tokenhub","models":{"hy4-preview":{"id":"hy4-preview","cost":{"input":0.834,"output":2.501}}}}
}`
	if err := os.WriteFile(catalog.CachePath(), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	catalog.Reset()
	t.Cleanup(catalog.Reset)
	return settings.Settings{ModelPrices: map[string]settings.ModelPrice{
		"*/workbuddy2api/deepseek-v4.1-flash": {Input: new(0.15), Output: new(0.6), CacheRead: new(0.003), CacheWrite: new(0.0), TimeZone: new("UTC"), TimeTiers: []settings.TimeTier{{Days: []int{1}, From: "09:00", To: "12:00", Input: new(0.3), Output: new(1.2)}}},
		"relay/mystery-model":                 {Input: new(99.0), Output: new(99.0), CacheRead: new(0.0), CacheWrite: new(0.0)},
	}}
}

func TestSessionPricesUseKnownModelNamesAndWildcardAliases(t *testing.T) {
	prices := sessionPriceFixture(t)
	for _, tc := range []struct {
		model string
		input float64
	}{
		{"deepseek-v4.1-flash", 0.15}, {"gemini-3.7-flash-high", 0.75},
		{"gemini-3.8-flash-high", 0.75}, {"gemini-9-flash-high", 9},
		{"mimo-v2.6-flash", 0.14}, {"muse-spark-1.2-contributor", 0.1},
		{"muse-spark-1.3-contributor", 0.1}, {"step-5-preview", 1},
		{"hy4-preview-f", 0.834}, {"k3", 3},
	} {
		p, ok := priceOf(prices, tc.model)
		if !ok || p.Input != tc.input {
			t.Errorf("%s: price=%+v known=%v, want input=%v", tc.model, p, ok, tc.input)
		}
	}
	if p, ok := priceOf(prices, "deepseek-v4.1-flash"); ok {
		at := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
		if got := p.CostSplitAt(at, 1000000, 0, 0, 0, 0); got != 0.3 {
			t.Errorf("wildcard alias lost peak pricing: %v", got)
		}
	}
	prices.ModelPrices["*/deepseek-v4.1-flash"] = settings.ModelPrice{Input: new(0.0), Output: new(0.0), CacheRead: new(0.0), CacheWrite: new(0.0)}
	if p, ok := priceOf(prices, "deepseek-v4.1-flash"); !ok || p.Input != 0 {
		t.Errorf("exact user-set zero must beat alias price: %+v %v", p, ok)
	}
	// A relay's own tariff, a free-looking id, or a model with no catalog
	// entry is not a price for a bare session-file model.
	for _, model := range []string{"mystery-model", "mimo-v2.5-free", "minimax-m3.1-flash-free", "omen-alpha", "union-alpha", "step-6-preview"} {
		if p, ok := priceOf(prices, model); ok {
			t.Errorf("unknown %s was guessed or borrowed: %+v", model, p)
		}
	}
	prices.ModelPrices["*/hy4-preview"] = settings.ModelPrice{Input: new(0.5), Output: new(1.0), CacheRead: new(0.0), CacheWrite: new(0.0)}
	if p, ok := priceOf(prices, "hy4-preview-f"); !ok || p.Input != 0.5 {
		t.Errorf("stated canonical alias price: %+v %v", p, ok)
	}
}

func TestExplicitZeroPricesMakeModelsFree(t *testing.T) {
	prices := sessionPriceFixture(t)
	for _, model := range []string{"omen-alpha", "union-alpha"} {
		prices.ModelPrices["*/"+model] = settings.ModelPrice{Input: new(0.0), Output: new(0.0), CacheRead: new(0.0), CacheWrite: new(0.0)}
	}
	for _, model := range []string{"omen-alpha", "union-alpha", "stealth/union-alpha"} {
		if p, ok := priceOf(prices, model); !ok || p.Input != 0 || p.Output != 0 || p.CacheRead != 0 || p.CacheWrite != 0 {
			t.Errorf("%s: free price=%+v known=%v", model, p, ok)
		}
	}
}
