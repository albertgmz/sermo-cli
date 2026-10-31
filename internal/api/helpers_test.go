package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// sleeper records each wait that the transport asks for.
type sleeper struct {
	mu    sync.Mutex
	waits []time.Duration
}

func (s *sleeper) sleep(_ context.Context, d time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.waits = append(s.waits, d)
	return nil
}

func (s *sleeper) got() []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]time.Duration(nil), s.waits...)
}

// newTestClient starts a test server with h and returns a Client for it.
func newTestClient(t *testing.T, token string, h http.HandlerFunc) (*Client, *sleeper) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	s := &sleeper{}
	c, err := New(Options{Server: srv.URL, Token: token, UserAgent: "sermo-cli/test", Sleep: s.sleep})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c, s
}

// writeJSON sends body with the JSON content type and the given status.
func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

const siteJSON = `{"name":"FCuba","description":"d","adultMode":false,"analytics":null,` +
	`"palette":{"accents":[],"nameColors":[],"prefixColors":[]},` +
	`"registration":{"approval":false,"fields":[],"minimumAge":0,"open":true},` +
	`"socialLinks":{},"socialSignIn":{"providers":[]}}`
