package social

import (
	"testing"
	"time"

	"github.com/albertgmz/sermo-cli/internal/config"
)

// pageRequests counts the message requests for page.
func pageRequests(f *fakeSermo, page string) int {
	n := 0
	for _, c := range f.find("GET", "/api/v1/conversations/5/messages") {
		if c.Query.Get("page") == page {
			n++
		}
	}
	return n
}

// longConversation serves a conversation with 45 messages, so 3 pages.
func longConversation(t *testing.T) *fakeSermo {
	t.Helper()
	f := newFakeSermo(t)
	f.handle("GET /api/v1/conversations/5", 200, convJSON(5, "Viaje a Varadero"))
	f.handle("GET /api/v1/conversations/5/unread", 200, map[string]any{"conversationId": 5, "messageId": 77, "cursor": nil, "page": 1})
	f.handle("GET /api/v1/conversations/5/messages", 200, map[string]any{
		"items": []any{message(77, 5, member(12, "Ana"), "<p>Hola</p>")}, "nextCursor": "c2", "total": 45,
	})
	f.handle("PUT /api/v1/conversations/5/read", 200, map[string]any{})
	return f
}

func TestPrefetchLightWarmsNextMessagesPage(t *testing.T) {
	f := longConversation(t)
	d := testDeps(t, f, true)
	d.Config.Prefetch = config.PrefetchLight
	h := start(t, NewConversation(d, 5))
	deadline := time.Now().Add(3 * time.Second)
	for pageRequests(f, "2") == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if pageRequests(f, "2") != 1 {
		t.Fatalf("light prefetch must ask for page 2 one time, calls = %d", pageRequests(f, "2"))
	}
	h.key("l")
	time.Sleep(150 * time.Millisecond)
	if n := pageRequests(f, "2"); n != 1 {
		t.Fatalf("page 2 must come from the warm cache, requests = %d", n)
	}
}

func TestPrefetchOffConversationWarmsNothing(t *testing.T) {
	f := longConversation(t)
	start(t, NewConversation(testDeps(t, f, true), 5))
	time.Sleep(150 * time.Millisecond)
	if n := pageRequests(f, "2"); n != 0 {
		t.Fatalf("prefetch off must not ask for page 2, requests = %d", n)
	}
}
