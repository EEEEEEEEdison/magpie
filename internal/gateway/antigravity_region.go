package gateway

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/yetone/magpie/internal/netproxy"
	"github.com/yetone/magpie/internal/provider"
)

// Antigravity's egress may choose a node per connection. Google refuses a
// request with a 400 "User location is not supported for the API use."
// when that node's address is outside the regions it serves; asking on a
// fresh connection can land on another node, as CPA's retry proxy does.
const antigravityRegionTries = 6

// antigravityRegionDelay is the pause between those tries.
var antigravityRegionDelay = 250 * time.Millisecond

// retryAntigravityRegion asks an Antigravity request refused for its
// location again on new connections, at most antigravityRegionTries in
// all. A refusal of another kind, or a 403, is passed on as it came.
func (s *Server) retryAntigravityRegion(ctx context.Context, p provider.Provider, to provider.Protocol, path string, body []byte, in http.Header, res *http.Response) (*http.Response, error) {
	if !p.RegionRetries() || res == nil || res.StatusCode != http.StatusBadRequest {
		return res, nil
	}
	var err error
	for attempt := 1; attempt < antigravityRegionTries; attempt++ {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		res.Body = io.NopCloser(bytes.NewReader(b))
		if !regionRefused(res.StatusCode, b) {
			return res, nil
		}
		if s.debug {
			log.Printf("antigravity: location refused, trying a new connection (%d/%d)", attempt+1, antigravityRegionTries)
		}
		select {
		case <-ctx.Done():
			return res, nil
		case <-time.After(antigravityRegionDelay):
		}
		res, err = s.forwardOnce(netproxy.Fresh(ctx), p, to, path, body, in)
		if err != nil || res.StatusCode != http.StatusBadRequest {
			return res, err
		}
	}
	return res, nil
}
