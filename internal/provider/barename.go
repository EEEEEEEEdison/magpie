package provider

import (
	"regexp"
	"strings"

	"github.com/yetone/magpie/internal/catalog"
)

// A model's own name — not a bare model id (BareModelIDs, which is an id
// without the provider in front of it): what is left of a model id once
// the maker or path a provider sells it under, its version, the size of
// its context and its case are off. The usage pages group a model's calls
// by it, so one model's calls at every provider are one row, and a price
// the user set for the model under one of its names is found under the
// others, so one price covers every provider serving it.

// versionTail is what a vendor puts after a model's name for the version
// it answered with: a date, a build number, Bedrock's v1:0, latest,
// preview. Not v3 alone: deepseek-v3 is another model than deepseek-v2.
var versionTail = regexp.MustCompile(`(?:[-_@:](?:\d{4}-\d{2}-\d{2}|\d{2}-\d{2}|\d{6,8}|\d{3,4}|v\d+:\d+|latest|preview|exp))+$`)

// vendorDot is Bedrock's region and maker before a model's name
// (us.anthropic.claude-…), the geography of an inference profile as
// catalog's bedrockGeos has them: global. and us-gov. among them. The
// makers are those models.dev's bedrock list names ids by (xai.grok-4.7,
// writer.palmyra-x5, moonshot.kimi-k2-thinking, nvidia.nemotron), which
// the reply names as Anthropic's own are, without the geography.
var vendorDot = regexp.MustCompile(`^(?:(?:global|us-gov|[a-z]{2,4})\.)?(?:anthropic|amazon|meta|mistral|cohere|ai21|deepseek|qwen|openai|google|moonshotai|moonshot|minimax|nvidia|writer|xai|zai)\.`)

// ContextTail is what Claude Code writes after a model's name for the size of
// its context: claude-opus-5[1m].
var ContextTail = regexp.MustCompile(`\[[^\]]*\]$`)

// sepRun is a run of separators with a space in it. A relay that lists its
// display name as the id spells it "GPT-6 Astra" where the model's own id is
// "gpt-6-astra", and "gpt-6- sol" where it is "gpt-6-sol": the same model,
// and it has to be one name for a price set under either and one row. The
// dashes and underscores beside the space go with it, as sameModel folds a
// run of spaces, "_" and "-" into one "-" for a routing group's members. A
// name with no space in it is left as it is.
var sepRun = regexp.MustCompile(`[-_]*\s+[-_\s]*`)

// windowTail is the window magpie offers a model at beside its default
// (grok-4.7@500k, Cursor's grok-4.7@256k): the vendor answers under the
// plain name, which is the same model (#1645).
var windowTail = regexp.MustCompile(`@\d+[km]?$`)

// modifierTail matches operational modifiers, promotional tags, and runtime
// effort variants attached to a model name (e.g. "-free", "-thinking",
// "-thought", "-high", "-low", "-medium", "-minimal", "-online", "-search").
var modifierTail = regexp.MustCompile(`(?:[-_](?:free|thinking|thought|high|low|medium|minimal|online|search))+$`)

// HasTurnTag reports whether a model id ends in a running tag a relay adds
// to the model's own name — a promotional one (-free), an effort pinned in
// the id (-high, -thinking) or a toggle's (-online) — rather than being the
// model's name: gemini-3.8-flash-high and minimax-m3.1-flash-free carry
// one, k3, hy4-preview-f and deepseek-v4-pro-202606 don't. A price or a
// level list read for such an id is the model's, its tag aside; one read
// for an id without a tag is the id's own, whatever the alias map folds.
func HasTurnTag(m string) bool {
	bare := BareModel(m)
	return bare != "" && modifierTail.MatchString(bare)
}

// VariantName is the name of the model a relay's id is a variant of — the
// id with its running tag off (gemini-3.8-flash-high) or with a suffix of
// the same name off (hy4-preview-f is hy4-preview, kimi-k3-auto is
// kimi-k3) — and false when the id is its own name. A short alias that
// stands for a different name is not a variant: k3 is Kimi Code's plan
// name, not the API's kimi-k3, so nothing of kimi-k3's is read for it.
func VariantName(id string) (string, bool) {
	canon, bare := CanonicalModel(id), BareModel(id)
	if canon == "" || canon == bare {
		return "", false
	}
	if HasTurnTag(id) || strings.HasPrefix(bare, canon) {
		return canon, true
	}
	return "", false
}

// CanonicalModel returns the normalized core model identifier by stripping
// channel wrappers, operational tags, effort variants, and folding well-known
// aliases, so disparate relay namings resolve to the same underlying model
// for pricing and cross-provider inheritance.
func CanonicalModel(m string) string {
	bare := BareModel(m)
	if bare == "" {
		return ""
	}
	canon := modifierTail.ReplaceAllString(bare, "")
	switch canon {
	case "k3", "kimi-k3-auto":
		return "kimi-k3"
	case "hy4-preview-f":
		return "hy4-preview"
	}
	if canon != "" {
		return canon
	}
	return bare
}

// BareModel is a model's name without its maker or path, its version, the
// size of its context or its case: workbuddy2api/deepseek-v4.1-flash and
// 9router/workbuddy/deepseek-v4.1-flash are both deepseek-v4.1-flash, the
// one model they are sold as, and so is cn:deepseek-v4.1-flash, the model a
// region is sold in. A space in it is a "-", with the dashes or
// underscores beside it: a relay that lists its display name as the id spells
// GPT-6 Astra's id "GPT-6-Astra" elsewhere, and GPT-6 Sol's "gpt-6- sol".
func BareModel(m string) string {
	m = ContextTail.ReplaceAllString(strings.ToLower(strings.TrimSpace(m)), "")
	m = windowTail.ReplaceAllString(m, "")
	if i := strings.LastIndexByte(m, '/'); i >= 0 {
		m = m[i+1:]
	}
	// a region is no part of a model's name, and catalog is the one that
	// says what counts as one
	m = catalog.WithoutRegion(m)
	m = vendorDot.ReplaceAllString(m, "")
	if loc := versionTail.FindStringIndex(m); loc != nil && loc[0] > 0 {
		m = m[:loc[0]]
	}
	// what a tail left behind, before a space becomes a "-": "opus [1m]" is
	// opus, not "opus-"
	m = strings.TrimSpace(m)
	return sepRun.ReplaceAllString(m, "-")
}
