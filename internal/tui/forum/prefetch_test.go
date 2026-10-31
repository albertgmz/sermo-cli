package forum

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/albertgmz/sermo-cli/internal/config"
)

// log returns each recorded request as "METHOD /path?query".
func (f *fakeSermo) log() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.reqs))
	for i, r := range f.reqs {
		out[i] = r.Method + " " + r.Path + "?" + r.Query
	}
	return out
}

// waitLog waits until the request log has want.
func waitLog(t *testing.T, f *fakeSermo, want string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if slices.Contains(f.log(), want) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("no request %q, log = %v", want, f.log())
}

// settle gives a background prefetch time to send a request that it must not send.
func settle() { time.Sleep(150 * time.Millisecond) }

func count(log []string, req string) int {
	n := 0
	for _, r := range log {
		if r == req {
			n++
		}
	}
	return n
}

func TestPrefetchOffReaderSendsSameRequests(t *testing.T) {
	tests := []struct {
		name   string
		member bool
		want   []string
	}{
		{"guest", false, []string{
			"GET /api/v1/threads/34898?",
			"GET /api/v1/threads/34898/posts?page=1",
			"GET /api/v1/reaction-types?",
			"GET /api/v1/appearance/colors?kind=accent",
		}},
		{"member", true, []string{
			"GET /api/v1/threads/34898?",
			"GET /api/v1/threads/34898/unread?",
			"GET /api/v1/threads/34898/posts?page=1",
			"GET /api/v1/bookmarks/status?contentType=post&ids=477999&ids=478003",
			"GET /api/v1/reaction-types?",
			"GET /api/v1/appearance/colors?kind=accent",
			"PUT /api/v1/threads/34898/read?",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeSermo(t)
			newHarness(t, NewThread(testDeps(t, f, tt.member), 34898, 0))
			settle()
			if got := f.log(); !slices.Equal(got, tt.want) {
				t.Fatalf("requests =\n%v\nwant\n%v", got, tt.want)
			}
		})
	}
}

func TestPrefetchLightWarmsNextPostsPage(t *testing.T) {
	for _, member := range []bool{false, true} {
		t.Run(fmt.Sprint("member=", member), func(t *testing.T) {
			f := newFakeSermo(t)
			d := testDeps(t, f, member)
			d.Config.Prefetch = config.PrefetchLight
			h := newHarness(t, NewThread(d, 34898, 0))
			waitLog(t, f, "GET /api/v1/threads/34898/posts?page=2")
			if member {
				checkReadBeforeWarm(t, f.log())
			}
			h.press("l")
			if h.top().(*reader).page != 2 {
				t.Fatal("l must show page 2")
			}
			settle()
			log := f.log()
			if n := count(log, "GET /api/v1/threads/34898/posts?page=2"); n != 1 {
				t.Fatalf("page 2 must come from the warm cache, log = %v", log)
			}
			if count(log, "GET /api/v1/threads/34898/posts?page=3") != 0 {
				t.Fatalf("page 2 is the last page and nothing must be warmed after it, log = %v", log)
			}
		})
	}
}

// checkReadBeforeWarm fails when the mark read of thread 34898 is missing or comes after the warm of page 2.
func checkReadBeforeWarm(t *testing.T, log []string) {
	t.Helper()
	read := slices.Index(log, "PUT /api/v1/threads/34898/read?")
	if read == -1 {
		t.Fatalf("a member must mark the thread read, log = %v", log)
	}
	if read > slices.Index(log, "GET /api/v1/threads/34898/posts?page=2") {
		t.Fatalf("the warm must start after the mark read, log = %v", log)
	}
}

func TestPrefetchLightWarmsNextThreadsPage(t *testing.T) {
	f := newFakeSermo(t)
	d := testDeps(t, f, false)
	d.Config.Prefetch = config.PrefetchLight
	h := newHarness(t, NewNode(d, 44))
	waitLog(t, f, "GET /api/v1/nodes/44/threads?page=2")
	h.press("l")
	waitLog(t, f, "GET /api/v1/nodes/44/threads?page=3")
	if n := count(f.log(), "GET /api/v1/nodes/44/threads?page=2"); n != 1 {
		t.Fatalf("page 2 must come from the warm cache, log = %v", f.log())
	}
}

func TestPrefetchOffThreadListWarmsNothing(t *testing.T) {
	f := newFakeSermo(t)
	newHarness(t, NewNode(testDeps(t, f, false), 44))
	settle()
	if n := count(f.log(), "GET /api/v1/nodes/44/threads?page=2"); n != 0 {
		t.Fatalf("prefetch off must not ask for page 2, log = %v", f.log())
	}
}

// unreadForums sets up 7 unread forums. The display order is 45, 37, 64, 44, 68, 69, 70.
func unreadForums(t *testing.T, f *fakeSermo) {
	f.set("GET /api/v1/nodes", 200, patched(t, "nodes.json", func(v map[string]any) {
		items := v["items"].([]any)
		for _, id := range []int{69, 70} {
			n := map[string]any{}
			for k, val := range items[len(items)-1].(map[string]any) {
				n[k] = val
			}
			n["id"], n["title"] = id, fmt.Sprint("Foro ", id)
			items = append(items, n)
		}
		v["items"] = items
	}))
	f.set("GET /api/v1/nodes/unread", 200, `{"nodeIds":[70,68,45,69,37,64,44]}`)
	for _, id := range []int{45, 37, 64, 44, 68, 69, 70} {
		f.set(fmt.Sprintf("GET /api/v1/nodes/%d/threads", id), 200, fixture(t, "threads.json"))
	}
}

func TestPrefetchFullWarmsUnreadForums(t *testing.T) {
	f := newFakeSermo(t)
	unreadForums(t, f)
	d := testDeps(t, f, true)
	d.Config.Prefetch = config.PrefetchFull
	newHarness(t, NewNodes(d))
	for _, id := range []int{45, 37, 64, 44, 68} {
		waitLog(t, f, fmt.Sprintf("GET /api/v1/nodes/%d/threads?page=1", id))
	}
	settle()
	for _, id := range []int{69, 70} {
		if n := len(f.requests("GET", fmt.Sprintf("/api/v1/nodes/%d/threads", id))); n != 0 {
			t.Fatalf("only 5 forums must be warmed, log = %v", f.log())
		}
	}
}

func TestPrefetchLightHomeWarmsNothing(t *testing.T) {
	f := newFakeSermo(t)
	unreadForums(t, f)
	d := testDeps(t, f, true)
	d.Config.Prefetch = config.PrefetchLight
	newHarness(t, NewNodes(d))
	settle()
	want := []string{"GET /api/v1/nodes?", "GET /api/v1/nodes/unread?"}
	if got := f.log(); !slices.Equal(got, want) {
		t.Fatalf("requests = %v, want %v", got, want)
	}
}
