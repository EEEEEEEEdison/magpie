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
// older spellings and Claude Code's context suffix. Unknown ids stay unknown.
func DesktopAlias(id string) (string, bool) {
	id = strings.TrimSuffix(id, "[1m]")
	var number string
	for _, prefix := range []string{"anthropic/magpie-", "mythos-magpie-", "claude-magpie-"} {
		if n, ok := strings.CutPrefix(id, prefix); ok {
			number = n
			break
		}
	}
	if rest, ok := strings.CutPrefix(id, "magpie-"); ok {
		if i := strings.Index(rest, ".anthropic."); i > 0 {
			number = rest[:i]
		}
	}
	if len(number) != 10 {
		return "", false
	}
	for _, e := range Catalog() {
		if DesktopAliasNumber(e.ID) == number {
			return e.ID, true
		}
	}
	return "", false
}
