package provider

import (
	"fmt"
	"hash/fnv"
	"strings"
)

// DesktopAliasNumber is the stable number in a Claude Desktop/Science
// compatibility alias. Serving and usage must decode the same identity.
func DesktopAliasNumber(id string) string {
	h := fnv.New64a()
	h.Write([]byte(id))
	return fmt.Sprintf("%010d", h.Sum64()%1e10)
}

// DesktopAlias resolves only magpie's compatibility aliases, including the
// older spellings, Claude Science's and Claude Code's context suffix:
// anthropic/magpie-<number>, mythos-magpie-<number>, claude-magpie-<number>
// or magpie-<number>.anthropic.<claude model>. Unknown ids stay unknown:
// the gateway serves by it, and reads a tier's alias after it
// (desktopTierAlias), so a number no catalog model has is never served
// as the Claude model its tail names (DesktopAliasName is for naming).
func DesktopAlias(id string) (string, bool) {
	number, ok := desktopAliasShape(id)
	if !ok {
		return "", false
	}
	// the catalog Desktop and Science are listed from: a signed-in
	// subscription's model a group listing alone keeps out of Catalog
	// is still theirs (SubscriptionKept)
	for _, e := range CatalogOf("claude-desktop") {
		if DesktopAliasNumber(e.ID) == number {
			return e.ID, true
		}
	}
	return "", false
}

// desktopAliasShape is the 10-digit number a compatibility alias carries,
// whether or not a catalog entry still carries it.
func desktopAliasShape(id string) (string, bool) {
	id = strings.TrimSuffix(id, "[1m]")
	var number string
	if n, ok := strings.CutPrefix(id, "anthropic/magpie-"); ok {
		number = n
	} else if n, ok := strings.CutPrefix(id, "mythos-magpie-"); ok {
		number = n
	} else if n, ok := strings.CutPrefix(id, "claude-magpie-"); ok {
		// Claude Science's (gateway/science.go)
		number = n
	} else if n, _, ok := desktopClaudeAlias(id); ok {
		number = n
	}
	if len(number) != 10 || strings.Trim(number, "0123456789") != "" {
		return "", false
	}
	return number, true
}

// DesktopAliasModel is the model the usage pages count a call to a
// compatibility alias under: DesktopAlias's, or for a
// magpie-<number>.anthropic.<claude model> no catalog model has any more
// (a session saved on a provider removed since) the Claude model it
// names. Only for naming a call; never served by.
func DesktopAliasModel(id string) (string, bool) {
	if m, ok := DesktopAlias(id); ok {
		return m, true
	}
	if _, model, ok := desktopClaudeAlias(strings.TrimSuffix(id, "[1m]")); ok && model != "" {
		return model, true
	}
	return "", false
}

// DesktopAliasName is the name the usage pages count a call that asked for
// a compatibility alias under. An alias its catalog still lists is that
// entry (DesktopAliasModel, which also reads the Claude model a
// magpie-<number>.anthropic.<model> tail names). One whose provider has
// been removed since has no entry left, and is named by the model the
// call's own reply named (served): the number was minted from the entry
// magpie served, so the reply says what it stood for. Nothing is read
// from an id of no alias shape, or from a reply that is one too. "" for
// an id that is no alias of magpie's or an unresolved one with no reply.
func DesktopAliasName(asked, served string) string {
	if m, ok := DesktopAliasModel(asked); ok {
		return m
	}
	if _, ok := desktopAliasShape(asked); !ok || served == "" || served == strings.TrimSuffix(asked, "[1m]") {
		return ""
	}
	if _, ok := desktopAliasShape(served); ok {
		return ""
	}
	return served
}

// desktopClaudeAlias splits magpie-<number>.anthropic.<claude model>.
func desktopClaudeAlias(id string) (number, model string, ok bool) {
	rest, ok := strings.CutPrefix(id, "magpie-")
	if !ok {
		return "", "", false
	}
	i := strings.Index(rest, ".anthropic.")
	if i != 10 {
		return "", "", false
	}
	return rest[:i], rest[i+len(".anthropic."):], true
}
