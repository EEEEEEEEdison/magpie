package usage

import (
	"testing"
	"time"
)

// The Requests page's search box asks the models alone: the one the ledger
// recorded and the one the upstream said it served. A call the client asked
// for by one name and another answered belongs to the model that answered
// it, so searching "claude" finds no call a relay answered with gpt-6.1-sol,
// and the row's provider, host, session, account and key names are searched
// by their own filters, not by this box.
func TestSearchAsksTheModelsAlone(t *testing.T) {
	swapped := Record{Agent: "claude-desktop", Requested: "claude-sonnet-5", Model: "gpt-6.1-sol",
		Provider: "cpa", Host: "relay.example", Session: "s-1234", Effort: "high",
		SessionAccount: "work@example.com", ProviderKeyID: "k1", ProviderKeyName: "Team",
		CallerKeyID: "ck1", CallerKeyName: "Laptop"}
	served := Record{Model: "mimo-v2.6-pro", Served: "gemini-3.8-flash-high", Provider: "cpa"}
	cases := []struct {
		query string
		r     Record
		want  bool
	}{
		{"claude", swapped, false}, // the name asked for, not the one served
		{"gpt-6.1", swapped, true}, // the model recorded
		{"GPT-6.1-SOL", swapped, true},
		{"6.1-sol", swapped, true},        // a part of the name
		{"cpa", swapped, false},           // the provider
		{"relay.example", swapped, false}, // the host
		{"s-1234", swapped, false},        // the session
		{"work@example.com", swapped, false},
		{"Team", swapped, false},   // the provider key's name
		{"Laptop", swapped, false}, // the gateway key's name
		{"high", swapped, false},   // the effort
		{"gemini", served, true},   // the served model, which the table shows
		{"mimo", served, true},
	}
	for _, c := range cases {
		if got := (Filter{Query: c.query}).keeps(c.r); got != c.want {
			t.Errorf("Query %q over %s/%s: %v, want %v", c.query, c.r.Model, c.r.Served, got, c.want)
		}
	}
	// and the ledger's own rows keep the same promise
	if rows, _, _ := ledger(time.Time{}, Filter{Query: "claude"}, []Record{swapped, served}); len(rows) != 0 {
		t.Fatalf("the name a client asked for found a row: %+v", rows)
	}
}
