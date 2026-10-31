package tui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

func TestSignOutStopsPrefetchOfOldClient(t *testing.T) {
	f := newFakeSermo(t)
	var mu sync.Mutex
	oldToken := 0
	started, release := make(chan struct{}, 1), make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/threads/") {
			f.Config.Handler.ServeHTTP(w, r)
			return
		}
		mu.Lock()
		if r.Header.Get("Authorization") == "Bearer "+testToken {
			oldToken++
		}
		mu.Unlock()
		if r.URL.Query().Get("page") == "1" {
			started <- struct{}{}
			<-release
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"nextCursor":null}`))
	}))
	t.Cleanup(srv.Close)
	d := testDeps(t, f, testToken)
	client, err := api.New(api.Options{Server: srv.URL, Token: testToken, UserAgent: "test"})
	if err != nil {
		t.Fatal(err)
	}
	d.Server, d.API = srv.URL, client
	m := newTestModel(t, d, StartHome)
	for p := 1; p <= 5; p++ {
		client.WarmPosts(1, api.PageOpts{Page: p})
		if p == 1 {
			<-started
		}
	}
	m.Update(ui.UnauthorizedMsg{})
	if d.API == client {
		t.Fatal("the client did not change")
	}
	close(release)
	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if oldToken != 1 {
		t.Fatalf("requests with the old token = %d, want only the one in flight", oldToken)
	}
}
