package provider

import (
	"github.com/yetone/magpie/internal/lazyre"
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
var versionTail = lazyre.New(`(?:[-_@:](?:\d{4}-\d{2}-\d{2}|\d{2}-\d{2}|\d{6,8}|\d{3,4}|v\d+:\d+|latest|preview|exp))+$`)

// vVersion is a model's version written with a dash or an underscore where
// its own name has a dot: Volcengine Ark and Mistral sell
// deepseek-v4.1-flash as deepseek-v4-1-flash, and Mistral dates it -1017,
// so the usage pages' Model split must count the two as one model and a
// price set for either must cover both. Only a version after "v": Claude's
// claude-opus-5-5 is that model's own name, not claude-opus-5.5.
var vVersion = lazyre.New(`v\d+(?:[-_.]\d+)+`)

// glmVersion is GLM's version written with a dash where Z.ai's own name has
// a dot: Mistral lists glm-5.3 as zai-glm-5-3 and glm-5.3-flash as
// zai-glm-5-3-flash. Only a number after a number with the name ending
// there, so GLM's open models keep theirs: glm-4-9b, glm-4-32b-0414.
var glmVersion = lazyre.New(`glm-(\d+)-(\d+)(-|$)`)

// vendorDot is Bedrock's region and maker before a model's name
// (us.anthropic.claude-…), the geography of an inference profile as
// catalog's bedrockGeos has them: global. and us-gov. among them. The
// makers are those models.dev's bedrock list names ids by (xai.grok-4.7,
// writer.palmyra-x5, moonshot.kimi-k2-thinking, nvidia.nemotron), which
// the reply names as Anthropic's own are, without the geography.
var vendorDot = lazyre.New(`^(?:(?:global|us-gov|[a-z]{2,4})\.)?(?:anthropic|amazon|meta|mistral|cohere|ai21|deepseek|qwen|openai|google|moonshotai|moonshot|minimax|nvidia|writer|xai|zai)\.`)

// ContextTail is what Claude Code writes after a model's name for the size of
// its context: claude-opus-5[1m].
var ContextTail = lazyre.New(`\[[^\]]*\]$`)

// sepRun is a run of separators with a space in it. A relay that lists its
// display name as the id spells it "GPT-6 Astra" where the model's own id is
// "gpt-6-astra", and "gpt-6- sol" where it is "gpt-6-sol": the same model,
// and it has to be one name for a price set under either and one row. The
// dashes and underscores beside the space go with it, as sameModel folds a
// run of spaces, "_" and "-" into one "-" for a routing group's members. A
// name with no space in it is left as it is.
var sepRun = lazyre.New(`[-_]*\s+[-_\s]*`)

// windowTail is the window magpie offers a model at beside its default
// (grok-4.7@500k, Cursor's grok-4.7@256k): the vendor answers under the
// plain name, which is the same model (#1645).
var windowTail = lazyre.New(`@\d+[km]?$`)

// modifierTail matches operational modifiers, promotional tags, and runtime
// effort variants attached to a model name (e.g. "-free", "-thinking",
// "-thought", "-high", "-low", "-medium", "-minimal", "-online", "-search").
var modifierTail = lazyre.New(`(?:[-_](?:free|thinking|thought|high|low|medium|minimal|online|search))+$`)

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
// stands for a different name is not a variant here: k3 is Kimi Code's
// plan name, and its price is the plan's (PricedNameFor), not read for it
// as a variant's.
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

// UsageModel is the name the usage pages count a model's calls under and
// filter them by: its own name (BareModel), for an id with a level a
// relay pins after a versioned model's name (gemini-3.8-flash-high,
// gpt-oss-120b-high, glm-5.3-flash-free) that model's name, and for a
// name that is another model's own (usageAlias: k3 is kimi-k3) that
// model. It is not the price rules' VariantName: a word that names a
// model of its own (mistral-medium, kimi-k2-thinking, gpt-4o-search) is
// never folded, so two models never share a row.
func UsageModel(m string) string { return UsageOfBare(BareModel(m)) }

// usageLevel is a level or promotional tag after a model's name, the ones
// UsageModel folds; the model's name before it must hold a digit.
var usageLevel = lazyre.New(`^(.*\d.*?)(?:-(?:minimal|low|medium|high|xhigh|free))+$`)

// usageAlias is a name that is another model sold under a name of its own:
// Kimi Code's plan name k3 is the API's kimi-k3. The usage pages count the
// two as one model, as they count a level's id as its model's; each call
// keeps the id it was made under for its price and the Model · provider
// split. A name one vendor's plan used for a time is the user's alias
// (`magpie model alias`), not a rule here.
var usageAlias = map[string]string{
	"k3": "kimi-k3",
}

// UsageOfBare is UsageModel of a name BareModel already read.
func UsageOfBare(bare string) string {
	if m := usageLevel.FindStringSubmatch(bare); m != nil && m[1] != "" {
		bare = m[1]
	}
	if name, ok := usageAlias[bare]; ok {
		return name
	}
	return bare
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
	if id, ok := DesktopAlias(m); ok {
		m = id
	}
	m = ContextTail.ReplaceAllString(strings.ToLower(strings.TrimSpace(m)), "")
	m = windowTail.ReplaceAllString(m, "")
	if i := strings.LastIndexByte(m, '/'); i >= 0 {
		m = m[i+1:]
	}
	if rest, ok := strings.CutPrefix(m, "magpie-"); ok {
		if i := strings.Index(rest, ".anthropic."); i > 0 {
			m = rest[i+len(".anthropic."):]
		}
	}
	// a region is no part of a model's name, and catalog is the one that
	// says what counts as one
	m = catalog.WithoutRegion(m)
	m = vendorDot.ReplaceAllString(m, "")
	// Z.ai's name before a model of its own, with a dash where the maker
	// list above writes a dot: Mistral lists glm-5.3 as zai-glm-5-3. Only
	// zai: deepseek-r1 and deepseek-chat are DeepSeek's own names.
	if rest, ok := strings.CutPrefix(m, "zai-"); ok && rest != "" {
		m = rest
	}
	if loc := versionTail.FindStringIndex(m); loc != nil && loc[0] > 0 {
		m = m[:loc[0]]
	}
	// what a tail left behind, before a space becomes a "-": "opus [1m]" is
	// opus, not "opus-"
	m = strings.TrimSpace(m)
	// a version a vendor spells with a dash is the model's own dotted one
	m = glmVersion.ReplaceAllString(m, "glm-$1.$2$3")
	m = vVersion.ReplaceAllStringFunc(m, func(s string) string {
		return strings.NewReplacer("-", ".", "_", ".").Replace(s)
	})
	return sepRun.ReplaceAllString(m, "-")
}
