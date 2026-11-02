package gui

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The package refuses nonlocal dials before they can reach the network.
func TestRunsOffline(t *testing.T) {
	tr := http.DefaultTransport.(*http.Transport)
	clone := tr.Clone()
	t.Cleanup(clone.CloseIdleConnections)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // a missing guard must not send even the red check off-machine
	for _, transport := range []*http.Transport{tr, clone} {
		conn, err := transport.DialContext(ctx, "tcp", "203.0.113.1:443")
		if conn != nil {
			conn.Close()
		}
		if err == nil || !strings.Contains(err.Error(), "tests stay off the network") {
			t.Errorf("nonlocal dial = %v; want the package's offline guard", err)
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "local") }))
	defer srv.Close()
	resp, err := (&http.Client{Transport: clone}).Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || string(body) != "local" {
		t.Fatalf("loopback = %q, %v", body, err)
	}
}
