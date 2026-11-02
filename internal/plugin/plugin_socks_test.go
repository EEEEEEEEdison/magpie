package plugin

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/netproxy"
)

// socksTo 把 SOCKS5 连接转发到 addr 并计数；addr 为空时明确拒绝 CONNECT。
// 使用拒绝响应，避免刚释放的端口被其他监听器占用。
func socksTo(t *testing.T, addr string) (string, *atomic.Int32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	var mu sync.Mutex
	conns := map[net.Conn]bool{}
	var serving sync.WaitGroup
	accepted := make(chan struct{})
	t.Cleanup(func() {
		ln.Close()
		cancel()
		<-accepted
		mu.Lock()
		for c := range conns {
			c.Close()
		}
		mu.Unlock()
		serving.Wait()
	})
	var n atomic.Int32
	go func() {
		defer close(accepted)
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns[c] = true
			mu.Unlock()
			serving.Add(1)
			go func(c net.Conn) {
				defer serving.Done()
				defer func() {
					c.Close()
					mu.Lock()
					delete(conns, c)
					mu.Unlock()
				}()
				var h [2]byte
				if _, err := io.ReadFull(c, h[:]); err != nil {
					return
				}
				io.CopyN(io.Discard, c, int64(h[1]))
				c.Write([]byte{5, 0})
				var r [4]byte
				if _, err := io.ReadFull(c, r[:]); err != nil {
					return
				}
				switch r[3] {
				case 1:
					io.CopyN(io.Discard, c, 4+2)
				case 3:
					var l [1]byte
					io.ReadFull(c, l[:])
					io.CopyN(io.Discard, c, int64(l[0])+2)
				}
				if addr == "" {
					c.Write([]byte{5, 5, 0, 1, 0, 0, 0, 0, 0, 0})
					return
				}
				up, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
				if err != nil {
					c.Write([]byte{5, 5, 0, 1, 0, 0, 0, 0, 0, 0})
					return
				}
				defer up.Close()
				n.Add(1)
				c.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0})
				copied := make(chan struct{})
				go func() {
					io.Copy(up, c)
					up.Close()
					close(copied)
				}()
				io.Copy(c, up)
				c.Close()
				up.Close()
				<-copied
			}(c)
		}
	}()
	return "socks5://" + ln.Addr().String(), &n
}

func closedPort(t *testing.T) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	a := ln.Addr().String()
	ln.Close()
	return a
}

// A plugin's request goes through a SOCKS5 proxy, as a built-in's does,
// though Bun can't use one itself; and a proxy that is down, SOCKS or
// HTTP, is said as Go's transport says it ("proxyconnect …"), so the
// gateway asks the next account and rests none, as for a built-in's.
func TestPluginSOCKSProxy(t *testing.T) {
	sandbox(t)
	vendor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "vendor "+r.Host)
	}))
	defer vendor.Close()
	socks, carried := socksTo(t, strings.TrimPrefix(vendor.URL, "http://"))
	t.Setenv("FAKE_BASE", "http://vendor.invalid/v1")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	abs, _ := filepath.Abs("testdata/fake/index.js")
	if _, err := Add(ctx, abs); err != nil {
		t.Fatal(err)
	}
	if _, err := Providers(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := APIKey(ctx, "fakeco", 0, nil, "k1", NewAccount); err != nil {
		t.Fatal(err)
	}
	fetch := func(proxy, url string) (int, string, error) {
		res, err := Fetch(netproxy.With(ctx, proxy), FetchRequest{Provider: "fakeco", Model: "fake-1", NPM: "@ai-sdk/openai-compatible",
			URL: url, Method: "POST", Body: []byte(`{}`)})
		if err != nil {
			return 0, "", err
		}
		defer res.Body.Close()
		b, err := io.ReadAll(res.Body)
		return res.StatusCode, string(b), err
	}
	if _, b, err := fetch(socks, "http://vendor.invalid/v1/chat/completions"); err != nil || b != "vendor vendor.invalid" || carried.Load() != 1 {
		t.Fatalf("through the SOCKS proxy: %q, %v; it carried %d", b, err, carried.Load())
	}
	// 采用上游讨论中 skygunner 的 a93c613a 补丁：拒绝由 SOCKS 服务明确返回。
	deadSOCKS, _ := socksTo(t, "")
	if netproxy.ForBun(deadSOCKS) == deadSOCKS {
		t.Fatal("the SOCKS proxy has no bridge")
	}
	// 全部监听器已启动，避免 bridge 再占用刚释放的 HTTP 代理端口。
	deadHTTP := "http://" + closedPort(t)
	for _, x := range []struct{ proxy, url, says string }{
		{deadHTTP, "http://vendor.invalid/v1/chat/completions", "proxyconnect tcp: dial tcp 127.0.0.1:"},
		{deadHTTP, "https://vendor.invalid/v1/chat/completions", "proxyconnect tcp: dial tcp 127.0.0.1:"},
	} {
		if _, b, err := fetch(x.proxy, x.url); err == nil || !strings.Contains(err.Error(), x.says) {
			t.Fatalf("%s through %s, which is down: %q, %v; want it to say %q", x.url, x.proxy, b, err, x.says)
		}
	}
	// HTTP 保留 bridge 的 502 响应；HTTPS CONNECT 也可由 Bun 报为明确的
	// proxyconnect 502 传输错误，两者都由网关归类为代理故障，不休息账户。
	for _, url := range []string{"http://vendor.invalid/v1/chat/completions", "https://vendor.invalid/v1/chat/completions"} {
		code, b, err := fetch(deadSOCKS, url)
		if err != nil {
			if !strings.HasPrefix(url, "https://") || !strings.HasPrefix(err.Error(), "proxyconnect tcp: 502 Bad Gateway") {
				t.Fatalf("%s through a SOCKS proxy that refuses: %d %q, %v", url, code, b, err)
			}
			continue
		}
		if code != 502 || !strings.HasPrefix(b, "socks connect tcp 127.0.0.1:") {
			t.Fatalf("%s through a SOCKS proxy that refuses: %d %q, %v", url, code, b, err)
		}
	}
}
