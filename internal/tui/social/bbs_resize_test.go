package social

import (
	"strconv"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// lastRange returns the first and the end index of the items that the
// last request to path asks for.
func lastRange(t *testing.T, f *fakeSermo, path string) (int, int) {
	t.Helper()
	calls := f.find("GET", path)
	if len(calls) == 0 {
		t.Fatalf("no request to %s", path)
	}
	q := calls[len(calls)-1].Query
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	if page < 1 || limit < 1 {
		t.Fatalf("query of %s = %v", path, q)
	}
	return (page - 1) * limit, page * limit
}

// wantResizeKeepsPosition goes to page 2, makes the window higher and
// goes to the next page. No item may be left out or show two times.
func wantResizeKeepsPosition(t *testing.T, h *harness, f *fakeSermo, path string) {
	t.Helper()
	h.command(">", "")
	first, _ := lastRange(t, f, path)
	if first == 0 {
		t.Fatalf("> must go to page 2")
	}
	h.w, h.h = 120, 35
	h.send(tea.WindowSizeMsg{Width: h.w, Height: h.h})
	lo, hi := lastRange(t, f, path)
	if first < lo || first >= hi {
		t.Fatalf("after the resize the page must show item %d, it shows %d to %d", first, lo, hi)
	}
	h.command(">", "")
	if next, _ := lastRange(t, f, path); next != hi {
		t.Fatalf("> after the resize must start at item %d, it starts at %d", hi, next)
	}
}

func TestSearchBBSResizeKeepsThePosition(t *testing.T) {
	f := newFakeSermo(t)
	f.handle("GET /api/v1/search", 200, map[string]any{"items": []any{}, "nextCursor": nil, "total": 100})
	h := startBBS(t, NewSearch(bbsDeps(t, f, false)))
	h.command("B", "cuba")
	wantResizeKeepsPosition(t, h, f, "/api/v1/search")
}

func TestOwnProfileBBSBookmarksResizeKeepsThePosition(t *testing.T) {
	f := newFakeSermo(t)
	f.handle("GET /api/v1/users/7", 200, profileJSON(7, "albert"))
	f.handle("GET /api/v1/users/7/profile-posts", 200, map[string]any{"items": []any{}, "nextCursor": nil})
	f.handle("GET /api/v1/bookmarks", 200, map[string]any{"items": []any{}, "nextCursor": nil, "total": 100})
	h := startBBS(t, NewProfile(bbsDeps(t, f, true), 0))
	h.command("F", "")
	wantResizeKeepsPosition(t, h, f, "/api/v1/bookmarks")
}
