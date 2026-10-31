package social

import (
	"net/http"
	"testing"

	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

func searchServer(t *testing.T) *fakeSermo {
	t.Helper()
	f := newFakeSermo(t)
	f.handle("GET /api/v1/search", 200, map[string]any{"items": []any{map[string]any{
		"post": map[string]any{
			"id": 478487, "position": 1, "createdAt": "2026-10-31T13:04:45Z",
			"author": member(282, "Nomura"), "excerpt": "Funciona desde <b>afuera</b> de Cuba &amp; más",
		},
		"thread": map[string]any{
			"id": 34941, "title": "Soberanía y MiIdentidad", "nodeId": 177,
			"node": map[string]any{"id": 177, "title": "Informática y Tecnología"}, "prefix": nil,
		},
	}}, "nextCursor": nil, "total": 1})
	f.handle("GET /api/v1/users", 200, map[string]any{"items": []any{
		map[string]any{"id": 12, "username": "Ana", "staff": false},
	}})
	return f
}

func TestSearchSendsQueryOnEnterAndShowsResults(t *testing.T) {
	f := searchServer(t)
	h := start(t, NewSearch(testDeps(t, f, false)))
	if h.s.CapturesInput() {
		t.Fatal("the global hotkeys must work when the screen opens")
	}
	h.key("/")
	if !h.s.CapturesInput() {
		t.Fatal("/ must give the focus to the query field")
	}
	h.typeText("cuba")
	if n := len(f.find("GET", "/api/v1/search")); n != 0 {
		t.Fatalf("typing must not search, got %d requests", n)
	}
	h.key("ctrl+t", "enter")
	calls := f.find("GET", "/api/v1/search")
	if len(calls) != 1 || calls[0].Query.Get("q") != "cuba" || calls[0].Query.Get("titlesOnly") != "true" {
		t.Fatalf("search requests = %#v", calls)
	}
	h.wantText("Soberanía y MiIdentidad", "Funciona desde afuera de Cuba & más", "Informática y Tecnología", "Nomura", "hace 1 h")
	if h.s.CapturesInput() {
		t.Fatal("the results must have the focus after a search")
	}
	h.key("enter")
	if open, ok := msgOf[ui.OpenThreadMsg](h.msgs); !ok || open != (ui.OpenThreadMsg{ThreadID: 34941, PostID: 478487}) {
		t.Fatalf("enter gave %#v", h.msgs)
	}
}

func TestSearchMembersOpensProfile(t *testing.T) {
	f := searchServer(t)
	h := start(t, NewSearch(testDeps(t, f, false)))
	h.key("ctrl+u")
	if !h.s.CapturesInput() {
		t.Fatal("ctrl+u must give the focus to the query field")
	}
	h.typeText("an")
	h.key("enter")
	calls := f.find("GET", "/api/v1/users")
	if len(calls) != 1 || calls[0].Query.Get("prefix") != "an" {
		t.Fatalf("user searches = %#v", calls)
	}
	h.wantText("Ana")
	h.key("enter")
	if open, ok := msgOf[ui.OpenProfileMsg](h.msgs); !ok || open.UserID != 12 {
		t.Fatalf("enter gave %#v", h.msgs)
	}
}

func TestSearchRateLimited(t *testing.T) {
	f := newFakeSermo(t)
	f.mux.HandleFunc("GET /api/v1/search", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "40")
		writeJSON(w, 429, errBody("rate_limited", "x"))
	})
	h := start(t, NewSearch(testDeps(t, f, false)))
	h.key("enter")
	h.typeText("cuba")
	h.key("enter")
	h.wantText("Espera 40 s")
}
