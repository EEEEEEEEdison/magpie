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
// or magpie-<number>.anthropic.<claude model>. Unknown ids stay unknown.
func DesktopAlias(id string) (string, bool) {
	id = strings.TrimSuffix(id, "[1m]")
	var number string
	if n, ok := strings.CutPrefix(id, "anthropic/magpie-"); ok {
		number = n
	} else if n, ok := strings.CutPrefix(id, "mythos-magpie-"); ok {
		number = n
	} else if n, ok := strings.CutPrefix(id, "claude-magpie-"); ok {
		// Claude Science's (gateway/science.go)
		number = n
	} else if rest, ok := strings.CutPrefix(id, "magpie-"); ok {
		if i := strings.Index(rest, ".anthropic."); i > 0 {
			number = rest[:i]
		}
	}
	if len(number) != 10 {
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
