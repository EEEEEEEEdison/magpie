package gateway

import (
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/yetone/magpie/internal/usage"
)

// 长流式推理后接短答案仍有整轮速度；不能仅按答案的短窗口误判 held。
func TestStreamedReasoningWithShortAnswerKeepsSpeed(t *testing.T) {
	for _, tc := range []struct {
		name, thought, answer string
	}{
		{"chat", `data: {"choices":[{"delta":{"reasoning_content":"` + strings.Repeat("x", 80) + `"}}]}`, `data: {"choices":[{"delta":{"content":"ok"}}]}`},
		{"anthropic", `data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"` + strings.Repeat("x", 80) + `"}}`, `data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"ok"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				f := firstToken{start: time.Now()}
				for i := 0; i < 40; i++ {
					time.Sleep(200 * time.Millisecond)
					f.see([]byte(tc.thought + "\n\n"))
				}
				time.Sleep(200 * time.Millisecond)
				f.see([]byte(tc.answer + "\n\n"))
				ttft, text := f.ms()
				flow := f.flowFor(390)
				n, ms := usage.DecodeOf(400, 390, 8200, ttft, text, flow)
				if n != 400 || ms != 8200 {
					t.Fatalf("streamed reasoning lost whole-turn speed: %d tokens/%d ms, flow %d, ttft %d, first text %d; want 400/8200", n, ms, flow, ttft, text)
				}
			})
		})
	}
}
