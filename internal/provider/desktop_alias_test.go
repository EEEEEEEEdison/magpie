package provider

import "testing"

// DesktopAlias is what the gateway serves an alias by, so a number no
// catalog model has stays unknown there: a tier's own alias
// (desktopTierAlias) is read by the gateway after it, and a made-up
// number must not be served as the Claude model its tail names. The
// usage pages, which only name a call, read that tail (DesktopAliasModel):
// a session saved on a provider removed since still counts as its model.
func TestDesktopAliasKeepsAnUnknownNumberUnknown(t *testing.T) {
	for _, id := range []string{
		"magpie-0000000000.anthropic.claude-opus-4-8",
		"magpie-5377544721.anthropic.claude-opus-5-5[1m]",
		"mythos-magpie-0000000000",
	} {
		if got, ok := DesktopAlias(id); ok {
			t.Errorf("DesktopAlias(%s) = %q, served as a model no number names", id, got)
		}
	}
	for id, want := range map[string]string{
		"magpie-5377544721.anthropic.claude-opus-5-5[1m]": "claude-opus-5-5",
		"magpie-0000000000.anthropic.claude-opus-4-8":     "claude-opus-4-8",
	} {
		if got, ok := DesktopAliasModel(id); !ok || got != want {
			t.Errorf("DesktopAliasModel(%s) = %q %v, want %q", id, got, ok, want)
		}
		if got := BareModel(id); got != want {
			t.Errorf("BareModel(%s) = %q, want %q", id, got, want)
		}
	}
	for _, id := range []string{"mythos-magpie-0000000000", "magpie-12.anthropic.claude-opus-4-8", "claude-opus-4-8", "anthropic/claude-opus-4-8"} {
		if got, ok := DesktopAliasModel(id); ok {
			t.Errorf("DesktopAliasModel(%s) = %q, guessed a model", id, got)
		}
	}
}
