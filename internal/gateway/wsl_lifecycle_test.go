package gateway

import (
	"context"
	"net"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/yetone/magpie/internal/filememo"
)

// closeHeldListener 关闭后仍把 Accept 留在收尾处，模拟 Serve 尚未返回。
type closeHeldListener struct {
	addr    *net.TCPAddr
	started chan struct{}
	closed  chan struct{}
	leaving chan struct{}
	release chan struct{}
	left    chan struct{}
	once    sync.Once
}

func (l *closeHeldListener) Addr() net.Addr { return l.addr }
func (l *closeHeldListener) Close() error {
	l.once.Do(func() { close(l.closed) })
	return nil
}
func (l *closeHeldListener) Accept() (net.Conn, error) {
	close(l.started)
	<-l.closed
	close(l.leaving)
	<-l.release
	close(l.left)
	return nil, net.ErrClosed
}

// watcher 返回时，每个 Serve 都必须已经退出，不能继续使用下一轮测试的状态。
func TestServeWSLWaitsForEveryListener(t *testing.T) {
	fresh(t)
	t.Setenv("MAGPIE_ADDR", "127.0.0.1:3425")
	oldListen, oldEverywhere := wslListen, wslServeEverywhere
	t.Cleanup(func() { wslListen, wslServeEverywhere = oldListen, oldEverywhere })
	wslServeEverywhere = true
	fakeWSLNet(t, "127.0.0.2/32")
	synctest.Test(t, func(t *testing.T) {
		ln := &closeHeldListener{
			addr:    &net.TCPAddr{IP: net.ParseIP("127.0.0.2"), Port: 3425},
			started: make(chan struct{}), closed: make(chan struct{}),
			leaving: make(chan struct{}), release: make(chan struct{}), left: make(chan struct{}),
		}
		letGo := sync.OnceFunc(func() { close(ln.release) })
		defer letGo()
		wslListen = func(network, addr string) (net.Listener, error) {
			if network != "tcp" || addr != "127.0.0.2:3425" {
				t.Errorf("监听越过夹具：%s %s", network, addr)
			}
			return ln, nil
		}
		main := &closeHeldListener{addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 3425}}
		s := &Server{ln: main}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan struct{})
		go func() { defer close(done); s.serveWSL(ctx, &http.Server{}) }()
		<-ln.started
		cancel()
		<-ln.leaving
		synctest.Wait()
		select {
		case <-done:
			t.Error("WSL watcher 在 Serve 仍被屏障挂起时已经返回")
		default:
		}
		letGo()
		<-done
		select {
		case <-ln.left:
		default:
			t.Error("WSL watcher 返回后还有 listener 没有退出")
		}
	})
}

// 主网关的后台循环等待全局 filememo 通知，不能放进 synctest bubble。
// listener 仍为内存夹具；栈屏障确认主调用确实停在 watcher 的收尾处。
func TestGatewayWaitsForWSLWatcher(t *testing.T) {
	fresh(t)
	t.Setenv("MAGPIE_ADDR", "127.0.0.1:3425")
	oldListen, oldEverywhere := wslListen, wslServeEverywhere
	t.Cleanup(func() { wslListen, wslServeEverywhere = oldListen, oldEverywhere })
	wslServeEverywhere = true
	fakeWSLNet(t, "127.0.0.2/32")
	asked, release := make(chan struct{}), make(chan struct{})
	letGo := sync.OnceFunc(func() { close(release) })
	wslListen = func(network, addr string) (net.Listener, error) {
		if network != "tcp" || addr != "127.0.0.2:3425" {
			t.Errorf("监听越过夹具：%s %s", network, addr)
		}
		close(asked)
		<-release
		return nil, net.ErrClosed
	}
	main := &closeHeldListener{
		addr:    &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 3425},
		started: make(chan struct{}), closed: make(chan struct{}),
		leaving: make(chan struct{}), release: make(chan struct{}), left: make(chan struct{}),
	}
	close(main.release)
	Inherit(main, nil)
	s := New()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var err error
	go func() { defer close(done); err = s.ListenAndServe(ctx) }()
	t.Cleanup(func() {
		cancel()
		letGo()
		main.Close()
		<-done
	})
	<-main.started
	<-asked
	cancel()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	buf := make([]byte, 1<<20)
waiting:
	for {
		select {
		case <-done:
			t.Fatal("主网关在 WSL watcher 结束前已经返回")
		case <-deadline.C:
			t.Fatal("主网关没有到达 WSL watcher 的收尾屏障")
		default:
		}
		for _, stack := range strings.Split(string(buf[:runtime.Stack(buf, true)]), "\n\n") {
			if strings.Contains(stack, "[chan receive]") &&
				strings.Contains(stack, "gateway.(*Server).ListenAndServe.func") &&
				strings.Contains(stack, "\ngithub.com/yetone/magpie/internal/gateway.(*Server).ListenAndServe(") {
				break waiting
			}
		}
		runtime.Gosched()
	}
	letGo()
	<-done
	if err != nil {
		t.Fatal(err)
	}
	// 下一轮写必须安全：此前 bubble 内的 Written 把全局通道留给了下一例。
	filememo.Forget()
}
