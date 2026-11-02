package gui

import (
	"testing"

	"github.com/yetone/magpie/internal/gateway"
)

// A gateway started by a GUI test must not listen on the machine's WSL adapters.
func TestWSLNetworksAreIsolated(t *testing.T) {
	if nets := gateway.WSLNets(); len(nets) != 0 {
		t.Fatalf("tests see the machine's WSL networks: %v", nets)
	}
}
