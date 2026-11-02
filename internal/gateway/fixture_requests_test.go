package gateway

import (
	"fmt"
	"net"
	"net/http"
	"testing"
)

// 这些 fixture 的目录、登录和上游都在本机；缺一份响应也不能交给公网。
func localFixtureRequests(t *testing.T) {
	t.Helper()
	old := http.DefaultClient
	client := *old
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	client.Transport = countTransport(func(r *http.Request) (*http.Response, error) {
		host := r.URL.Hostname()
		if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			t.Errorf("本地 fixture 发起了外部请求：%s %s", r.Method, r.URL.Host)
			return nil, fmt.Errorf("fixture 只允许本机请求：%s", r.URL.Host)
		}
		return base.RoundTrip(r)
	})
	http.DefaultClient = &client
	t.Cleanup(func() { http.DefaultClient = old })
}
