package netproxy

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/yetone/magpie/internal/settings"
)

// 环境的绕过列表已经选择直连时，不能再退回系统代理；显式的用户设置仍优先。
func TestEnvironmentBypassStaysDirectBeforeSystem(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, k := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy", "NO_PROXY", "no_proxy"} {
		t.Setenv(k, "")
	}
	sysCache.Lock()
	oldAt, oldProxy := sysCache.at, sysCache.p
	sysCache.at, sysCache.p = farFuture, Proxy{URL: "http://127.0.0.1:7890"}
	sysCache.Unlock()
	t.Cleanup(func() {
		sysCache.Lock()
		sysCache.at, sysCache.p = oldAt, oldProxy
		sysCache.Unlock()
	})
	t.Setenv("ALL_PROXY", "http://127.0.0.1:7891")
	t.Setenv("NO_PROXY", ".example.invalid")
	u, _ := url.Parse("http://trace.example.invalid/cdn-cgi/trace")
	if p, err := For(u); err != nil || p != nil {
		t.Fatalf("NO_PROXY 后又退回了代理：%v %v", p, err)
	}
	t.Setenv("ALL_PROXY", "")
	if p, err := For(u); err != nil || p != nil {
		t.Fatalf("只有 NO_PROXY 时没有直连：%v %v", p, err)
	}
	other, _ := url.Parse("http://other.invalid/")
	if p, err := For(other); err != nil || p == nil || p.String() != "http://127.0.0.1:7890" {
		t.Fatalf("未匹配的地址没有使用系统代理：%v %v", p, err)
	}
	if err := settings.Save(settings.Settings{Proxy: "http://127.0.0.1:7892"}); err != nil {
		t.Fatal(err)
	}
	if p, err := For(u); err != nil || p == nil || p.String() != "http://127.0.0.1:7892" {
		t.Fatalf("显式设置没有优先：%v %v", p, err)
	}
	r, _ := http.NewRequest("GET", u.String(), nil)
	r = r.WithContext(With(r.Context(), "http://127.0.0.1:7893"))
	if p, err := Func(r); err != nil || p == nil || p.String() != "http://127.0.0.1:7893" {
		t.Fatalf("供应商自己的代理没有优先：%v %v", p, err)
	}
}
