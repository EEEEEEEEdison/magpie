package usage

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/catalog"
	"github.com/yetone/magpie/internal/provider"
	"github.com/yetone/magpie/internal/sessions"
	"github.com/yetone/magpie/internal/settings"
)

// A call magpie reads from an agent's own session file is that provider's call
// when the file names one of the user's providers: Pi writes the provider it
// was set up to call on every reply, and that name is where the call went, so
// the provider's tariff prices it and the row says where it went. A name that
// is no provider of the user's is left as the file recorded it
// (Record.SessionProvider) and the row claims no supplier, as it always has.
func TestASessionFilesProviderPricesItsCalls(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	catalog.Reset()
	t.Cleanup(catalog.Reset)

	if err := provider.Save(provider.Provider{ID: "cpa", Name: "CPA", Key: "k", Chat: "https://cpa.example/v1"}); err != nil {
		t.Fatal(err)
	}
	s := settings.Load()
	s.ModelPrices = map[string]settings.ModelPrice{
		"cpa/gemini-3.8-flash-high": {Input: new(0.75), Output: new(3.75), CacheRead: new(0.075), CacheWrite: new(0.0)},
	}
	if err := settings.Save(s); err != nil {
		t.Fatal(err)
	}

	call := sessions.Call{Time: time.Now(), Agent: "pi", Session: "s", Model: "gemini-3.8-flash-high",
		Upstream: "cpa", Tokens: sessions.Tokens{Input: 1000000, Output: 1000000}}

	row := logRecord(call)
	if row.Provider != "cpa" {
		t.Fatalf("the row's provider is %q, want cpa: the file named it", row.Provider)
	}
	if pr := Pricer()(row); pr == nil || pr.Input != 0.75 || pr.Output != 3.75 {
		t.Fatalf("the call priced at %v; want the $0.75/$3.75 the user set for cpa's model", pr)
	}

	// a name no provider has: the row claims no supplier, and the price set
	// for cpa's model is not spent on it
	call.Upstream = "apis_gemini"
	row = logRecord(call)
	if row.Provider != UnknownProvider {
		t.Fatalf("the row's provider is %q, want %s: no provider of the user's has that name", row.Provider, UnknownProvider)
	}
	if pr := Pricer()(row); pr != nil {
		t.Fatalf("priced at %v; want unpriced: nothing prices the model for a provider that isn't one", pr)
	}
}
