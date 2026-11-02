package tui

import (
	"net"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/gateway"
	"github.com/yetone/magpie/internal/testenv"
)

// The tests run in a home of their own, never the user's real files, and in
// UTC, the zone the sessions fixtures' days are counted in. The zone is set
// here, before any test starts a goroutine: a test that swapped it would race
// the gateway's loops an earlier test left running, which read time.Local.
func TestMain(m *testing.M) {
	// 测试仅使用显式夹具提供的网络，不监听本机 WSL 网卡。
	gateway.WSLNets = func() []*net.IPNet { return nil }
	time.Local = time.UTC
	testenv.Main(m)
}
