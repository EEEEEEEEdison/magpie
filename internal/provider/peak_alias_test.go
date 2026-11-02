package provider

import (
	"testing"
	"time"

	"github.com/yetone/magpie/internal/catalog"
	"github.com/yetone/magpie/internal/settings"
)

// 自定义价格换一种模型拼写命中时，仍须按实际供应商的峰谷规则计费。
func TestPeakFollowsCustomPriceAliases(t *testing.T) {
	useDeepSeekCatalog(t)
	peak := time.Date(2026, 10, 29, 2, 0, 0, 0, time.UTC)
	off := time.Date(2026, 10, 29, 11, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		name, model, key string
	}{
		{"direct", "deepseek-v4.1-flash", "deepseek/deepseek-v4.1-flash"},
		{"provider canonical", "deepseek/deepseek-v4.1-flash", "deepseek/deepseek-v4.1-flash"},
		{"wildcard canonical", "deepseek/deepseek-v4.1-flash", "*/deepseek-v4.1-flash"},
		{"wildcard spelling", "deepseek-v4.1-flash", "*/cn:deepseek-v4.1-flash"},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := settings.Settings{ModelPrices: map[string]settings.ModelPrice{c.key: statePrice(0.2)}}
			for _, known := range []bool{false, true} {
				p := Provider{ID: "deepseek", Chat: "https://api.deepseek.com/v1", PriceRate: 0.5}
				pr, ok := priceOf(s, p, known, c.model, time.Time{})
				if !ok || pr.Peak != catalog.PeakDeepSeek || pr.AtTime(peak).Input != 0.4 || pr.AtTime(off).Input != 0.2 {
					t.Errorf("known=%v: %+v, priced=%v; 自定义非高峰价应为 0.2，高峰价应为 0.4", known, pr, ok)
				}
			}
			// 中转商不因模型名字继承官方的峰谷；自定义价格也不乘 PriceRate。
			relay := Provider{ID: "relay", Chat: "https://relay.example/v1", PriceRate: 0.5}
			s.ModelPrices = map[string]settings.ModelPrice{"*/cn:deepseek-v4.1-flash": statePrice(0.2)}
			pr, ok := priceOf(s, relay, true, c.model, time.Time{})
			if !ok || pr.Peak != "" || pr.AtTime(peak).Input != 0.2 || pr.AtTime(off).Input != 0.2 {
				t.Errorf("中转商的自定义平价被改变: %+v, priced=%v", pr, ok)
			}
		})
	}
}
