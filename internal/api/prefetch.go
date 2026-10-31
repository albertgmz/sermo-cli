package api

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/albertgmz/sermo-cli/internal/api/gen"
)

// The limits of the page cache and of the prefetch queue.
const (
	// pageTTL is the time that a prefetched page stays usable.
	pageTTL = 2 * time.Minute
	// maxPages is the number of pages in the cache. A new page removes the oldest page.
	maxPages = 50
	// maxQueue is the number of prefetch jobs that can wait.
	maxQueue = 20
	// prefetchPause is the time without prefetch after a 429.
	prefetchPause = 60 * time.Second
)

// The key prefixes of the page cache.
const (
	prefixPosts    = "posts:"
	prefixThreads  = "threads:"
	prefixMessages = "messages:"
)

// prefetchKey marks the context of a prefetch request.
type prefetchKey struct{}

// isPrefetch reports whether ctx belongs to a prefetch request.
func isPrefetch(ctx context.Context) bool {
	v, _ := ctx.Value(prefetchKey{}).(bool)
	return v
}

func postsKey(threadID int, p PageOpts) string {
	return fmt.Sprintf("%s%d:%d:%s:%d", prefixPosts, threadID, p.Page, p.Cursor, p.Limit)
}

func threadsKey(nodeID, prefixID int, p PageOpts) string {
	return fmt.Sprintf("%s%d:%d:%d:%s:%d", prefixThreads, nodeID, prefixID, p.Page, p.Cursor, p.Limit)
}

func messagesKey(conversationID int, newestFirst bool, p PageOpts) string {
	return fmt.Sprintf("%s%d:%t:%d:%s:%d", prefixMessages, conversationID, newestFirst, p.Page, p.Cursor, p.Limit)
}

// WarmPosts puts a request for one page of posts in the prefetch queue. It does not wait.
// Posts with the same threadID and p then returns the page without a request, one time.
func (c *Client) WarmPosts(threadID int, p PageOpts) {
	warm(c, postsKey(threadID, p), func(ctx context.Context) (*PostList, error) {
		return c.fetchPosts(ctx, threadID, p)
	})
}

// WarmThreads puts a request for one page of a thread list in the prefetch queue. It does not wait.
// Threads with the same arguments then returns the page without a request, one time.
func (c *Client) WarmThreads(nodeID, prefixID int, p PageOpts) {
	warm(c, threadsKey(nodeID, prefixID, p), func(ctx context.Context) (*ThreadList, error) {
		return c.fetchThreads(ctx, nodeID, prefixID, p)
	})
}

// WarmMessages puts a request for one page of messages in the prefetch queue. It does not wait.
// Messages with the same arguments then returns the page without a request, one time.
func (c *Client) WarmMessages(conversationID int, newestFirst bool, p PageOpts) {
	warm(c, messagesKey(conversationID, newestFirst, p), func(ctx context.Context) (*Page[gen.Message], error) {
		return c.fetchMessages(ctx, conversationID, newestFirst, p)
	})
}

// warm adds a job that keeps the page from fetch in the page cache.
func warm[T any](c *Client, key string, fetch func(context.Context) (*T, error)) {
	if c.ctx.Err() != nil || c.pages.has(key, c.now()) {
		return
	}
	j := job{key: key, fetch: func(ctx context.Context) (any, error) { return fetch(ctx) }}
	if c.queue.add(j, c.now()) {
		go c.drain()
	}
}

// fromCache returns the page for key from the page cache and removes it. On a miss it calls fetch.
// When a prefetch for key runs, it waits for it. A prefetch for key that waits in the queue is removed.
func fromCache[T any](c *Client, ctx context.Context, key string, fetch func(context.Context) (*T, error)) (*T, error) {
	if running := c.queue.claim(key); running != nil {
		select {
		case <-running:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if v, ok := c.pages.take(key, c.now()); ok {
		return v.(*T), nil
	}
	return fetch(ctx)
}

// drain runs the jobs of the queue one at a time until the queue is empty.
func (c *Client) drain() {
	for {
		j, ok := c.queue.next()
		if !ok {
			return
		}
		c.run(j)
	}
}

// run sends the request of one job. A 429 stops the prefetch for prefetchPause.
func (c *Client) run(j job) {
	defer c.queue.done(j.key)
	if c.ctx.Err() != nil {
		c.queue.pause(time.Time{})
		return
	}
	epoch := c.pages.current()
	v, err := j.fetch(context.WithValue(c.ctx, prefetchKey{}, true))
	switch {
	case IsRateLimited(err):
		c.queue.pause(c.now().Add(prefetchPause))
	case err == nil:
		c.pages.put(j.key, v, epoch, c.now())
	}
}

// job is one prefetch request.
type job struct {
	key   string
	fetch func(context.Context) (any, error)
}

// queue holds the prefetch jobs. One goroutine runs them in order.
type queue struct {
	mu      sync.Mutex
	jobs    []job
	keys    map[string]bool // the keys of the jobs that wait or run
	running bool
	until   time.Time // no new job before this time
	// current is the key of the job that runs. flight closes when that job is done.
	current string
	flight  chan struct{}
}

// add puts j at the end of the queue. It ignores j during a pause, when the queue is full or when
// the key waits or runs. It returns true when the caller must start a goroutine for drain.
func (q *queue) add(j job, now time.Time) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if now.Before(q.until) || len(q.jobs) >= maxQueue || q.keys[j.key] {
		return false
	}
	if q.keys == nil {
		q.keys = map[string]bool{}
	}
	q.keys[j.key] = true
	q.jobs = append(q.jobs, j)
	start := !q.running
	q.running = true
	return start
}

// next removes the first job and returns it. When the queue is empty, the goroutine of drain stops.
func (q *queue) next() (job, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.jobs) == 0 {
		q.running = false
		return job{}, false
	}
	j := q.jobs[0]
	q.jobs = q.jobs[1:]
	q.current, q.flight = j.key, make(chan struct{})
	return j, true
}

func (q *queue) done(key string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.keys, key)
	close(q.flight)
	q.current, q.flight = "", nil
}

// claim returns a channel that closes when the job for key is done, if that job runs.
// If the job for key waits, claim removes it and returns nil.
func (q *queue) claim(key string) <-chan struct{} {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.keys[key] {
		return nil
	}
	if q.current == key {
		return q.flight
	}
	delete(q.keys, key)
	q.jobs = slices.DeleteFunc(q.jobs, func(j job) bool { return j.key == key })
	return nil
}

// pause removes the jobs that wait and refuses new jobs until the time until.
func (q *queue) pause(until time.Time) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, j := range q.jobs {
		delete(q.keys, j.key)
	}
	q.jobs = nil
	q.until = until
}

// idle reports whether no job waits or runs.
func (q *queue) idle() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return !q.running && len(q.keys) == 0
}

// pageEntry is one page in the page cache.
type pageEntry struct {
	key string
	v   any
	at  time.Time
}

// pageCache keeps the prefetched pages, oldest first. A write increments epoch, so a page from a
// request that started before the write does not go into the cache.
type pageCache struct {
	mu      sync.Mutex
	entries []pageEntry
	epoch   uint64
}

func (c *pageCache) current() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.epoch
}

// put adds the page v for key. It ignores v when a write occurred after the time of epoch.
func (c *pageCache) put(key string, v any, epoch uint64, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if epoch != c.epoch {
		return
	}
	c.remove(func(e pageEntry) bool { return e.key == key || now.Sub(e.at) > pageTTL })
	c.entries = append(c.entries, pageEntry{key: key, v: v, at: now})
	if len(c.entries) > maxPages {
		c.entries = c.entries[len(c.entries)-maxPages:]
	}
}

// take removes the page for key and returns it. An old page gives a miss.
func (c *pageCache) take(key string, now time.Time) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, e := range c.entries {
		if e.key == key {
			c.entries = append(c.entries[:i], c.entries[i+1:]...)
			return e.v, now.Sub(e.at) <= pageTTL
		}
	}
	return nil, false
}

// has reports whether the cache has a usable page for key.
func (c *pageCache) has(key string, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range c.entries {
		if e.key == key {
			return now.Sub(e.at) <= pageTTL
		}
	}
	return false
}

// drop removes the pages with a key that starts with one of the prefixes.
func (c *pageCache) drop(prefixes ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.epoch++
	c.remove(func(e pageEntry) bool {
		for _, p := range prefixes {
			if strings.HasPrefix(e.key, p) {
				return true
			}
		}
		return false
	})
}

// remove deletes the entries for which fn is true. The caller holds mu.
func (c *pageCache) remove(fn func(pageEntry) bool) {
	kept := c.entries[:0]
	for _, e := range c.entries {
		if !fn(e) {
			kept = append(kept, e)
		}
	}
	c.entries = kept
}

// threadPages returns the key prefix of the pages of posts of one thread.
func threadPages(threadID int) string { return fmt.Sprintf("%s%d:", prefixPosts, threadID) }

// conversationPages returns the key prefix of the pages of messages of one conversation.
func conversationPages(conversationID int) string {
	return fmt.Sprintf("%s%d:", prefixMessages, conversationID)
}
