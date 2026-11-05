package netproxy

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// Fresh sends a request on a new connection even when the transport has an
// idle one for the host.
func TestFreshDialsAgain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))
	defer srv.Close()

	var dials atomic.Int32
	base := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			dials.Add(1)
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
	}
	c := &http.Client{Transport: Dispatch(base)}
	defer c.CloseIdleConnections()

	get := func(ctx context.Context) {
		t.Helper()
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
		res, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
	}

	get(context.Background())
	get(context.Background())
	if got := dials.Load(); got != 1 {
		t.Fatalf("two requests dialled %d times, want 1", got)
	}
	get(Fresh(context.Background()))
	if got := dials.Load(); got != 2 {
		t.Fatalf("a fresh request dialled %d times, want 2", got)
	}
}
