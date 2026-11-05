package gateway

import (
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/provider"
)

const antigravityRegionRefusal = `{"error":{"code":400,"message":"User location is not supported for the API use.","status":"FAILED_PRECONDITION"}}`

func TestAntigravityRegionRetry(t *testing.T) {
	old := antigravityRegionDelay
	antigravityRegionDelay = time.Millisecond
	t.Cleanup(func() { antigravityRegionDelay = old })

	tests := []struct {
		name       string
		status     int
		body       string
		wantCalls  int32
		wantStatus int
	}{
		{"location refused is asked again", http.StatusBadRequest, antigravityRegionRefusal, 3, http.StatusOK},
		{"account not eligible is not", http.StatusForbidden, `{"error":{"code":403,"message":"Your current account is not eligible for Gemini Code Assist for individuals because it is not currently available in your location."}}`, 1, http.StatusForbidden},
		{"another 400 is not", http.StatusBadRequest, `{"error":{"code":400,"message":"bad request"}}`, 1, http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			s := New()
			s.client = &http.Client{Transport: countTransport(func(r *http.Request) (*http.Response, error) {
				n := calls.Add(1)
				status, body := tc.status, tc.body
				if tc.wantStatus == http.StatusOK && n >= 3 {
					status, body = http.StatusOK, `{}`
				}
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			p := provider.Provider{
				ID:        "antigravity",
				Responses: "http://up.example",
				Account:   &provider.Account{Agent: "antigravity"},
			}
			res, err := s.forward(t.Context(), p, provider.Responses, "/v1/responses", []byte(`{"model":"gemini-3.8-flash","input":"hi"}`), http.Header{})
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != tc.wantStatus {
				t.Errorf("status = %d, want %d", res.StatusCode, tc.wantStatus)
			}
			if got := calls.Load(); got != tc.wantCalls {
				t.Errorf("asked %d times, want %d", got, tc.wantCalls)
			}
		})
	}
}

func TestAntigravityRegionRetryStops(t *testing.T) {
	old := antigravityRegionDelay
	antigravityRegionDelay = time.Millisecond
	t.Cleanup(func() { antigravityRegionDelay = old })

	var calls atomic.Int32
	s := New()
	s.client = &http.Client{Transport: countTransport(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: http.StatusBadRequest, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(antigravityRegionRefusal))}, nil
	})}
	p := provider.Provider{
		ID:        "antigravity",
		Responses: "http://up.example",
		Account:   &provider.Account{Agent: "antigravity"},
	}
	res, err := s.forward(t.Context(), p, provider.Responses, "/v1/responses", []byte(`{"model":"gemini-3.8-flash","input":"hi"}`), http.Header{})
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", res.StatusCode)
	}
	if got := calls.Load(); got != antigravityRegionTries {
		t.Errorf("asked %d times, want %d", got, antigravityRegionTries)
	}
}
