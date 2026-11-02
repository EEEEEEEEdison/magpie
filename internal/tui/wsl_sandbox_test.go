package tui

import (
	"testing"

	"github.com/yetone/magpie/internal/gateway"
)

// 测试中的网关不得监听本机的 WSL 网卡。
func TestWSLNetworksAreIsolated(t *testing.T) {
	if nets := gateway.WSLNets(); len(nets) != 0 {
		t.Fatalf("tests see the machine's WSL networks: %v", nets)
	}
}
