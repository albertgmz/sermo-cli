package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const postsJSON = `{"items":[],"nextCursor":null}`

const threadsJSON = `{"items":[],"sticky":[],"nextCursor":null,"total":0}`

// clock is a time source that a test moves by hand.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// reqLog records the method, path and query of each request.
type reqLog struct {
	mu   sync.Mutex
	reqs []string
}

func (l *reqLog) add(r *http.Request) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.reqs = append(l.reqs, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
}

func (l *reqLog) count(prefix string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, r := range l.reqs {
		if strings.HasPrefix(r, prefix) {
			n++
		}
	}
	return n
}

func (l *reqLog) all() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.reqs...)
}

// pageServer serves posts, thread lists and the writes that clear the page cache.
func pageServer(t *testing.T, h func(w http.ResponseWriter, r *http.Request) bool) (*Client, *reqLog, *clock, *sleeper) {
	t.Helper()
	l := &reqLog{}
	clk := &clock{t: time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)}
	c, s := newTestClient(t, "tok", func(w http.ResponseWriter, r *http.Request) {
		l.add(r)
		if h != nil && h(w, r) {
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/posts") && r.Method == http.MethodGet:
			writeJSON(w, http.StatusOK, postsJSON)
		case strings.HasSuffix(r.URL.Path, "/posts"):
			writeJSON(w, http.StatusCreated, `{"id":9,"threadId":1,"position":2}`)
		case strings.HasSuffix(r.URL.Path, "/threads"):
			writeJSON(w, http.StatusOK, threadsJSON)
		case strings.HasSuffix(r.URL.Path, "/read"):
			writeJSON(w, http.StatusOK, `{"readAt":"2026-10-31T10:00:00Z","forums":1,"readPosition":3}`)
		default:
			http.NotFound(w, r)
		}
	})
	c.now = clk.now
	return c, l, clk, s
}

// waitIdle waits until the prefetch queue has no job.
func waitIdle(t *testing.T, c *Client) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if c.queue.idle() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the prefetch queue did not stop")
}

func TestWarmPostsServesCacheOneTime(t *testing.T) {
	c, l, _, _ := pageServer(t, nil)
	ctx := context.Background()
	c.WarmPosts(1, PageOpts{Page: 2})
	waitIdle(t, c)
	if n := l.count("GET /api/v1/threads/1/posts?page=2"); n != 1 {
		t.Fatalf("warm sent %d requests, want 1", n)
	}
	if _, err := c.Posts(ctx, 1, PageOpts{Page: 2}); err != nil {
		t.Fatalf("Posts: %v", err)
	}
	if n := l.count("GET /api/v1/threads/1/posts"); n != 1 {
		t.Fatalf("a warmed page must come from the cache, requests = %d", n)
	}
	if _, err := c.Posts(ctx, 1, PageOpts{Page: 2}); err != nil {
		t.Fatalf("Posts: %v", err)
	}
	if n := l.count("GET /api/v1/threads/1/posts"); n != 2 {
		t.Fatalf("a cache hit must be one-shot, requests = %d", n)
	}
}

func TestWarmThreadsKeysByParams(t *testing.T) {
	c, l, _, _ := pageServer(t, nil)
	ctx := context.Background()
	c.WarmThreads(44, 0, PageOpts{Page: 1})
	waitIdle(t, c)
	if _, err := c.Threads(ctx, 44, 0, PageOpts{Page: 2}); err != nil {
		t.Fatalf("Threads: %v", err)
	}
	if _, err := c.Threads(ctx, 44, 0, PageOpts{Page: 1}); err != nil {
		t.Fatalf("Threads: %v", err)
	}
	if n := l.count("GET /api/v1/nodes/44/threads?page=1"); n != 1 {
		t.Fatalf("page 1 requests = %d, want 1 from the warm", n)
	}
	if n := l.count("GET /api/v1/nodes/44/threads?page=2"); n != 1 {
		t.Fatalf("page 2 is not warm and must be requested, requests = %d", n)
	}
}

func TestWarmMessagesServesCache(t *testing.T) {
	c, l, _, _ := pageServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if strings.HasSuffix(r.URL.Path, "/messages") {
			writeJSON(w, http.StatusOK, `{"items":[],"nextCursor":null,"total":0}`)
			return true
		}
		return false
	})
	c.WarmMessages(5, false, PageOpts{Page: 2, Limit: 20})
	waitIdle(t, c)
	if _, err := c.Messages(context.Background(), 5, false, PageOpts{Page: 2, Limit: 20}); err != nil {
		t.Fatalf("Messages: %v", err)
	}
	if n := l.count("GET /api/v1/conversations/5/messages"); n != 1 {
		t.Fatalf("messages requests = %d, want 1", n)
	}
}

func TestPageCacheExpires(t *testing.T) {
	c, l, clk, _ := pageServer(t, nil)
	c.WarmPosts(1, PageOpts{Page: 2})
	waitIdle(t, c)
	clk.add(pageTTL + time.Second)
	if _, err := c.Posts(context.Background(), 1, PageOpts{Page: 2}); err != nil {
		t.Fatalf("Posts: %v", err)
	}
	if n := l.count("GET /api/v1/threads/1/posts"); n != 2 {
		t.Fatalf("an old page must be requested again, requests = %d", n)
	}
}

func TestWritesClearPageCache(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name  string
		warm  func(c *Client)
		write func(c *Client) error
		read  func(c *Client) error
		path  string
	}{
		{
			name:  "reply",
			warm:  func(c *Client) { c.WarmPosts(1, PageOpts{Page: 2}) },
			write: func(c *Client) error { _, err := c.Reply(ctx, 1, "hola"); return err },
			read:  func(c *Client) error { _, err := c.Posts(ctx, 1, PageOpts{Page: 2}); return err },
			path:  "GET /api/v1/threads/1/posts",
		},
		{
			name:  "mark thread read",
			warm:  func(c *Client) { c.WarmPosts(1, PageOpts{Page: 2}) },
			write: func(c *Client) error { _, err := c.MarkThreadRead(ctx, 1, 3); return err },
			read:  func(c *Client) error { _, err := c.Posts(ctx, 1, PageOpts{Page: 2}); return err },
			path:  "GET /api/v1/threads/1/posts",
		},
		{
			name:  "mark thread read clears thread lists",
			warm:  func(c *Client) { c.WarmThreads(44, 0, PageOpts{Page: 1}) },
			write: func(c *Client) error { _, err := c.MarkThreadRead(ctx, 1, 3); return err },
			read:  func(c *Client) error { _, err := c.Threads(ctx, 44, 0, PageOpts{Page: 1}); return err },
			path:  "GET /api/v1/nodes/44/threads",
		},
		{
			name:  "mark node read",
			warm:  func(c *Client) { c.WarmThreads(44, 0, PageOpts{Page: 1}) },
			write: func(c *Client) error { return c.MarkNodeRead(ctx, 44) },
			read:  func(c *Client) error { _, err := c.Threads(ctx, 44, 0, PageOpts{Page: 1}); return err },
			path:  "GET /api/v1/nodes/44/threads",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, l, _, _ := pageServer(t, nil)
			tt.warm(c)
			waitIdle(t, c)
			if err := tt.write(c); err != nil {
				t.Fatalf("write: %v", err)
			}
			if err := tt.read(c); err != nil {
				t.Fatalf("read: %v", err)
			}
			if n := l.count(tt.path); n != 2 {
				t.Fatalf("after a write the page must be requested again, requests = %d", n)
			}
		})
	}
}

func TestPrefetchRunsOneRequestAtATime(t *testing.T) {
	var cur, peak atomic.Int32
	release := make(chan struct{})
	started := make(chan struct{}, 1+maxQueue)
	c, l, _, _ := pageServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		started <- struct{}{}
		n := cur.Add(1)
		defer cur.Add(-1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		<-release
		time.Sleep(5 * time.Millisecond)
		return false
	})
	c.WarmPosts(1, PageOpts{Page: 1})
	<-started
	for p := 2; p <= 25; p++ {
		c.WarmPosts(1, PageOpts{Page: p})
	}
	c.WarmPosts(1, PageOpts{Page: 25})
	close(release)
	waitIdle(t, c)
	if peak.Load() != 1 {
		t.Fatalf("prefetch sent %d requests at the same time, want 1", peak.Load())
	}
	// One job runs and the queue holds maxQueue more. The repeated page is ignored.
	if n := l.count("GET /api/v1/threads/1/posts"); n != 1+maxQueue {
		t.Fatalf("prefetch requests = %d, want %d", n, 1+maxQueue)
	}
}

func TestPrefetchIgnoresDuplicates(t *testing.T) {
	release := make(chan struct{})
	c, l, _, _ := pageServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		<-release
		return false
	})
	c.WarmPosts(1, PageOpts{Page: 2})
	c.WarmPosts(1, PageOpts{Page: 2})
	c.WarmPosts(1, PageOpts{Page: 3})
	c.WarmPosts(1, PageOpts{Page: 3})
	close(release)
	waitIdle(t, c)
	c.WarmPosts(1, PageOpts{Page: 2})
	waitIdle(t, c)
	if n := l.count("GET /api/v1/threads/1/posts"); n != 2 {
		t.Fatalf("requests = %d, want 2: a queued or cached page must not be warmed again", n)
	}
}

func TestPrefetchRateLimitPausesQueue(t *testing.T) {
	release := make(chan struct{})
	c, l, clk, s := pageServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Query().Get("page") == "2" {
			<-release
			w.Header().Set("Retry-After", "1")
			writeJSON(w, http.StatusTooManyRequests, `{"error":{"code":"rate_limited","message":"slow"}}`)
			return true
		}
		return false
	})
	c.WarmPosts(1, PageOpts{Page: 2})
	c.WarmPosts(1, PageOpts{Page: 3})
	close(release)
	waitIdle(t, c)
	if n := l.count("GET /api/v1/threads/1/posts"); n != 1 {
		t.Fatalf("requests = %v, want only the first: no retry and the queue is dropped", l.all())
	}
	if w := s.got(); len(w) != 0 {
		t.Fatalf("a prefetch must not wait for a retry, waits = %v", w)
	}
	c.WarmPosts(1, PageOpts{Page: 4})
	waitIdle(t, c)
	if n := l.count("GET /api/v1/threads/1/posts"); n != 1 {
		t.Fatalf("the queue must pause after a 429, requests = %v", l.all())
	}
	clk.add(prefetchPause + time.Second)
	c.WarmPosts(1, PageOpts{Page: 4})
	waitIdle(t, c)
	if n := l.count("GET /api/v1/threads/1/posts?page=4"); n != 1 {
		t.Fatalf("the queue must run again after the pause, requests = %v", l.all())
	}
}

func TestPrefetchCallsOnlyListOperations(t *testing.T) {
	c, l, _, _ := pageServer(t, nil)
	c.WarmPosts(1, PageOpts{Page: 2})
	c.WarmThreads(44, 0, PageOpts{Page: 1})
	waitIdle(t, c)
	for _, r := range l.all() {
		path := strings.SplitN(r, "?", 2)[0]
		if path != "GET /api/v1/threads/1/posts" && path != "GET /api/v1/nodes/44/threads" {
			t.Fatalf("prefetch sent %s", r)
		}
	}
}

func TestPrefetchStopsWithClientContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	l := &reqLog{}
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		l.add(r)
		writeJSON(w, http.StatusOK, postsJSON)
	})
	c, err := New(Options{Server: srv, Context: ctx})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	cancel()
	c.WarmPosts(1, PageOpts{Page: 2})
	waitIdle(t, c)
	if n := l.count("GET"); n != 0 {
		t.Fatalf("prefetch after the end of the context sent %v", l.all())
	}
}

// newTestServer starts a test server with h and returns its URL.
func newTestServer(t *testing.T, h http.HandlerFunc) string {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestReadWaitsForRunningPrefetch(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, 4)
	c, l, _, _ := pageServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		started <- struct{}{}
		<-release
		return false
	})
	c.WarmPosts(1, PageOpts{Page: 2})
	<-started
	c.WarmPosts(1, PageOpts{Page: 3})
	got := make(chan error, 2)
	for _, p := range []int{2, 3} {
		go func() { _, err := c.Posts(context.Background(), 1, PageOpts{Page: p}); got <- err }()
	}
	time.Sleep(20 * time.Millisecond)
	close(release)
	for range 2 {
		if err := <-got; err != nil {
			t.Fatalf("Posts: %v", err)
		}
	}
	waitIdle(t, c)
	if n := l.count("GET /api/v1/threads/1/posts?page=2"); n != 1 {
		t.Fatalf("a read must wait for the running prefetch of the page, requests = %v", l.all())
	}
	if n := l.count("GET /api/v1/threads/1/posts?page=3"); n != 1 {
		t.Fatalf("a read must remove the waiting prefetch of the page, requests = %v", l.all())
	}
}

func TestPrefetchInFlightDuringWriteIsNotStored(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	var gets atomic.Int32
	c, l, _, _ := pageServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodGet && gets.Add(1) == 1 {
			started <- struct{}{}
			<-release
		}
		return false
	})
	ctx := context.Background()
	c.WarmPosts(1, PageOpts{Page: 2})
	<-started
	if _, err := c.Reply(ctx, 1, "hola"); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	close(release)
	waitIdle(t, c)
	if _, err := c.Posts(ctx, 1, PageOpts{Page: 2}); err != nil {
		t.Fatalf("Posts: %v", err)
	}
	if n := l.count("GET /api/v1/threads/1/posts?page=2"); n != 2 {
		t.Fatalf("a page from a prefetch that started before a write must not be used, requests = %v", l.all())
	}
}

// blockedClient returns a client for a server that holds each request for page 2 until release closes.
// started gets a value when the request for page 2 arrives.
func blockedClient(t *testing.T, ctx context.Context) (c *Client, l *reqLog, started, release chan struct{}) {
	t.Helper()
	l = &reqLog{}
	started, release = make(chan struct{}, 1), make(chan struct{})
	srv := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		l.add(r)
		if r.URL.Query().Get("page") == "2" {
			started <- struct{}{}
			<-release
		}
		writeJSON(w, http.StatusOK, postsJSON)
	})
	c, err := New(Options{Server: srv, Context: ctx})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c, l, started, release
}

func TestPrefetchQueuedJobStopsWithClientContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c, l, started, release := blockedClient(t, ctx)
	c.WarmPosts(1, PageOpts{Page: 2})
	<-started
	c.WarmPosts(1, PageOpts{Page: 3})
	cancel()
	close(release)
	waitIdle(t, c)
	if n := l.count("GET /api/v1/threads/1/posts?page=3"); n != 0 {
		t.Fatalf("a queued prefetch must not run after the end of the context, requests = %v", l.all())
	}
}

func TestCloseStopsQueuedPrefetch(t *testing.T) {
	c, l, started, release := blockedClient(t, context.Background())
	c.WarmPosts(1, PageOpts{Page: 2})
	<-started
	c.WarmPosts(1, PageOpts{Page: 3})
	c.Close()
	close(release)
	waitIdle(t, c)
	c.WarmPosts(1, PageOpts{Page: 4})
	waitIdle(t, c)
	if n := l.count("GET"); n != 1 {
		t.Fatalf("after Close only the request in flight may reach the server, requests = %v", l.all())
	}
}
