package api

import (
	"context"
	"net/http"
	"sync"
	"testing"
)

// counter counts the requests for each method and path.
type counter struct {
	mu sync.Mutex
	n  map[string]int
}

func (c *counter) add(r *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.n == nil {
		c.n = map[string]int{}
	}
	c.n[r.Method+" "+r.URL.Path]++
}

func (c *counter) get(key string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n[key]
}

func metadataServer(t *testing.T) (*Client, *counter) {
	t.Helper()
	cnt := &counter{}
	c, _ := newTestClient(t, "tok", func(w http.ResponseWriter, r *http.Request) {
		cnt.add(r)
		switch r.URL.Path {
		case "/api/v1/site":
			writeJSON(w, http.StatusOK, siteJSON)
		case "/api/v1/nodes":
			writeJSON(w, http.StatusOK, `{"items":[{"id":1,"title":"General","type":"forum"}]}`)
		case "/api/v1/nodes/unread":
			writeJSON(w, http.StatusOK, `{"nodeIds":[1]}`)
		case "/api/v1/nodes/1/read", "/api/v1/nodes/read":
			writeJSON(w, http.StatusOK, `{"readAt":"2026-10-31T10:00:00Z","forums":1}`)
		case "/api/v1/reaction-types":
			writeJSON(w, http.StatusOK, `{"items":[{"id":1,"title":"Like","emoji":"x"}]}`)
		case "/api/v1/thread-prefixes":
			writeJSON(w, http.StatusOK, `{"items":[{"id":4,"label":"`+r.URL.Query().Get("nodeId")+`"}]}`)
		case "/api/v1/appearance/colors":
			writeJSON(w, http.StatusOK, `{"items":[{"id":2,"name":"`+r.URL.Query().Get("kind")+`"}]}`)
		default:
			http.NotFound(w, r)
		}
	})
	return c, cnt
}

func TestCacheAvoidsSecondRequest(t *testing.T) {
	c, cnt := metadataServer(t)
	ctx := context.Background()
	for range 2 {
		if _, err := c.Site(ctx); err != nil {
			t.Fatalf("Site: %v", err)
		}
		if _, err := c.Nodes(ctx); err != nil {
			t.Fatalf("Nodes: %v", err)
		}
		if _, err := c.ReactionTypes(ctx); err != nil {
			t.Fatalf("ReactionTypes: %v", err)
		}
		if _, err := c.Palette(ctx, "accent"); err != nil {
			t.Fatalf("Palette: %v", err)
		}
		if _, err := c.ThreadPrefixes(ctx, 9); err != nil {
			t.Fatalf("ThreadPrefixes: %v", err)
		}
	}
	for _, key := range []string{
		"GET /api/v1/site", "GET /api/v1/nodes", "GET /api/v1/reaction-types",
		"GET /api/v1/appearance/colors", "GET /api/v1/thread-prefixes",
	} {
		if n := cnt.get(key); n != 1 {
			t.Errorf("%s: %d requests, want 1", key, n)
		}
	}
}

func TestCacheKeepsEntriesApartByKey(t *testing.T) {
	c, cnt := metadataServer(t)
	ctx := context.Background()
	a, err := c.ThreadPrefixes(ctx, 1)
	if err != nil {
		t.Fatalf("ThreadPrefixes: %v", err)
	}
	b, err := c.ThreadPrefixes(ctx, 2)
	if err != nil {
		t.Fatalf("ThreadPrefixes: %v", err)
	}
	if a[0].Label != "1" || b[0].Label != "2" {
		t.Errorf("labels = %q %q, want 1 2", a[0].Label, b[0].Label)
	}
	if n := cnt.get("GET /api/v1/thread-prefixes"); n != 2 {
		t.Errorf("prefix requests = %d, want 2", n)
	}
	p, err := c.Palette(ctx, "name")
	if err != nil || p[0].Name != "name" {
		t.Errorf("Palette(name) = %v, %v", p, err)
	}
}

func TestMarkReadDropsCachedNodes(t *testing.T) {
	tests := []struct {
		name string
		mark func(*Client) error
	}{
		{"MarkNodeRead", func(c *Client) error { return c.MarkNodeRead(context.Background(), 1) }},
		{"MarkAllRead", func(c *Client) error { return c.MarkAllRead(context.Background()) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, cnt := metadataServer(t)
			ctx := context.Background()
			if _, err := c.Nodes(ctx); err != nil {
				t.Fatalf("Nodes: %v", err)
			}
			if err := tc.mark(c); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if _, err := c.Nodes(ctx); err != nil {
				t.Fatalf("Nodes: %v", err)
			}
			if n := cnt.get("GET /api/v1/nodes"); n != 2 {
				t.Errorf("node requests = %d, want 2", n)
			}
		})
	}
}

func TestUnreadNodesAreNotCached(t *testing.T) {
	c, cnt := metadataServer(t)
	ctx := context.Background()
	for range 2 {
		ids, err := c.UnreadNodeIDs(ctx)
		if err != nil || len(ids) != 1 || ids[0] != 1 {
			t.Fatalf("UnreadNodeIDs = %v, %v", ids, err)
		}
	}
	if n := cnt.get("GET /api/v1/nodes/unread"); n != 2 {
		t.Errorf("unread requests = %d, want 2", n)
	}
}

func TestFailedFetchIsNotCached(t *testing.T) {
	calls := 0
	c, _ := newTestClient(t, "", func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			writeJSON(w, http.StatusInternalServerError, `{"error":{"code":"internal","message":"boom"}}`)
			return
		}
		writeJSON(w, http.StatusOK, siteJSON)
	})
	if _, err := c.Site(context.Background()); err == nil {
		t.Fatal("first Site: want error")
	}
	if _, err := c.Site(context.Background()); err != nil {
		t.Fatalf("second Site: %v", err)
	}
}
