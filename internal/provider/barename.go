package provider

import (
	"regexp"
	"strings"
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
// catalog's bedrockGeos has them: global. and us-gov. among them.
var vendorDot = regexp.MustCompile(`^(?:(?:global|us-gov|[a-z]{2,4})\.)?(?:anthropic|amazon|meta|mistral|cohere|ai21|deepseek|qwen|openai|google|moonshotai|minimax|zai)\.`)

// ContextTail is what Claude Code writes after a model's name for the size of
// its context: claude-opus-5[1m].
var ContextTail = regexp.MustCompile(`\[[^\]]*\]$`)

// spaceRun is a run of whitespace inside a model's name. A relay that lists
// its display name as the id spells it "GPT-6 Astra" where the model's own id
// is "gpt-6-astra": the same model, and it has to be one name for a price set
// under either and one row (sameModel, which folds a run of spaces, "_" and
// "-" into one "-" for a routing group's members, is the same judgement).
var spaceRun = regexp.MustCompile(`\s+`)

// windowTail is the window magpie offers a model at beside its default
// (grok-4.7@500k, Cursor's grok-4.7@256k): the vendor answers under the
// plain name, which is the same model (#1645).
var windowTail = regexp.MustCompile(`@\d+[km]?$`)

// BareModel is a model's name without its maker or path, its version, the
// size of its context or its case: workbuddy2api/deepseek-v4.1-flash and
// 9router/workbuddy/deepseek-v4.1-flash are both deepseek-v4.1-flash, the
// one model they are sold as. A space in it is a "-": a relay that lists its
// display name as the id spells GPT-6 Astra's id "GPT-6-Astra" elsewhere.
func BareModel(m string) string {
	m = ContextTail.ReplaceAllString(strings.ToLower(strings.TrimSpace(m)), "")
	m = spaceRun.ReplaceAllString(m, "-")
	m = windowTail.ReplaceAllString(m, "")
	if i := strings.LastIndexByte(m, '/'); i >= 0 {
		m = m[i+1:]
	}
	m = vendorDot.ReplaceAllString(m, "")
	if loc := versionTail.FindStringIndex(m); loc != nil && loc[0] > 0 {
		m = m[:loc[0]]
	}
	return m
}
