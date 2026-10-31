package forum

import (
	"net/url"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/i18n"

	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// idxThreads gives the threads fixture with an unread, a hot, a locked and
// a prefixed thread.
func idxThreads(t *testing.T) string {
	return patched(t, "threads.json", func(v map[string]any) {
		items := v["items"].([]any)
		unread, locked := items[0].(map[string]any), items[1].(map[string]any)
		unread["isUnread"] = true
		locked["isLocked"] = true
		hot := map[string]any{}
		for k, val := range locked {
			hot[k] = val
		}
		hot["id"], hot["title"], hot["replyCount"], hot["isLocked"] = 34700, "Tema caliente", 134, false
		hot["prefix"] = map[string]any{"id": 3, "label": "RESUELTO", "color": "#00ff00"}
		v["items"] = append(items, hot)
	})
}

func idxThreadFake(t *testing.T) *fakeSermo {
	t.Helper()
	f := indexFake(t)
	f.set("GET /api/v1/nodes/44/threads", 200, idxThreads(t))
	f.set("GET /api/v1/thread-prefixes", 200,
		`{"items":[{"id":3,"label":"RESUELTO","color":"#00ff00","nodeIds":[44]},{"id":5,"label":"AYUDA","color":"#ff0000","nodeIds":[44]}]}`)
	return f
}

func TestBBSThreadListShowsStickyBlockMarkersAndHeader(t *testing.T) {
	f := idxThreadFake(t)
	h := indexHarness(t, NewNode(indexDeps(t, f, true), 44), 80, 19)
	idxCapture(t, h, "forum-threads-80x24.txt")
	v := h.view()
	wantWidth(t, v, 80)
	h.wantView("Índice > Comunidad > Foro Libre", "Página 1 / 5", "| #", "| Tema", "| "+i18n.T("bbs.forum.col.author"),
		"| Resp", "| Último", "*  nuevo", "#  caliente (+100)", "[!] fijado", "x  cerrado", "|  134 |")
	wantRows(t, v, map[string]string{
		"Comic del Foro": "| [!]",
		"Ay bbs":         "| 34898*",
		"Las flipantes":  "| 34822x",
		"Tema caliente":  "| 34700# | [RESUELTO] Tema caliente",
	})
	sticky, normal := strings.Index(v, "Comic del Foro"), strings.Index(v, "Ay bbs")
	if between := v[sticky:normal]; !strings.Contains(between, "+------") {
		t.Errorf("the sticky block must have its own rule:\n%s", v)
	}
	reqs := f.requests("GET", "/api/v1/nodes/44/threads")
	if len(reqs) != 1 || !strings.Contains(reqs[0].Query, "limit=10") {
		t.Fatalf("the page size must follow the height: %+v", reqs)
	}
	if h.top().(ui.Commander).Prompt() != "Foro Libre" {
		t.Fatalf("prompt = %q", h.top().(ui.Commander).Prompt())
	}
}

func TestBBSThreadListNumberOpensThread(t *testing.T) {
	f := idxThreadFake(t)
	h := indexHarness(t, NewNode(indexDeps(t, f, false), 44), 80, 19)
	h.runCmd("#", "34822")
	if len(h.intent) != 1 || h.intent[0] != (ui.OpenThreadMsg{ThreadID: 34822}) {
		t.Fatalf("34822 gave %#v", h.intent)
	}
}

func TestBBSThreadListPagesAndFilter(t *testing.T) {
	f := idxThreadFake(t)
	h := indexHarness(t, NewNode(indexDeps(t, f, true), 44), 120, 35)
	idxCapture(t, h, "forum-threads-120x40.txt")
	h.runCmd(">", "")
	reqs := f.requests("GET", "/api/v1/nodes/44/threads")
	if last := reqs[len(reqs)-1].Query; !strings.Contains(last, "page=2") || !strings.Contains(last, "limit=25") {
		t.Fatalf("> must ask for page 2, got %q", last)
	}
	h.wantView("Página 2 / 2")
	h.runCmd("<", "")
	reqs = f.requests("GET", "/api/v1/nodes/44/threads")
	if last := reqs[len(reqs)-1].Query; !strings.Contains(last, "page=1") {
		t.Fatalf("< must ask for page 1, got %q", last)
	}

	h.runCmd("F", "")
	idxCapture(t, h, "forum-threads-filter-120x40.txt")
	h.wantView("01", "RESUELTO", "02", "AYUDA")
	h.runCmd("#", "2")
	reqs = f.requests("GET", "/api/v1/nodes/44/threads")
	if last := reqs[len(reqs)-1].Query; !strings.Contains(last, "prefixId=5") || !strings.Contains(last, "page=1") {
		t.Fatalf("F 2 must filter by prefix 5, got %q", last)
	}
	h.wantView("Prefijo: AYUDA")
	h.runCmd("F", "0")
	reqs = f.requests("GET", "/api/v1/nodes/44/threads")
	if last := reqs[len(reqs)-1].Query; strings.Contains(last, "prefixId") {
		t.Fatalf("F 0 must clear the filter, got %q", last)
	}
}

func TestBBSThreadListNewThreadNeedsCanPost(t *testing.T) {
	f := idxThreadFake(t)
	h := indexHarness(t, NewNode(indexDeps(t, f, true), 44), 80, 19)
	if idxHasCommand(h.top(), "N") {
		t.Fatal("N needs canPost")
	}
	f.set("GET /api/v1/nodes/44", 200, strings.Replace(fixture(t, "node.json"), `"canPost":false`, `"canPost":true`, 1))
	h = indexHarness(t, NewNode(indexDeps(t, f, true), 44), 80, 19)
	if !idxHasCommand(h.top(), "N") {
		t.Fatal("a member with canPost must get N")
	}
}

func TestBBSThreadListMarkReadAndIndex(t *testing.T) {
	f := idxThreadFake(t)
	f.set("PUT /api/v1/nodes/44/read", 200, `{"ok":true}`)
	h := indexHarness(t, NewNode(indexDeps(t, f, true), 44), 80, 19)
	h.runCmd("L", "")
	if len(f.requests("PUT", "/api/v1/nodes/44/read")) != 1 {
		t.Fatal("L must mark the forum read")
	}
	if strings.Contains(h.view(), "34898*") {
		t.Fatalf("the thread must show as read:\n%s", h.view())
	}
	h.runCmd("I", "")
	if len(h.stack) != 0 {
		t.Fatal("I must go back to the index")
	}
}

func TestBBSThreadListNewThreadHasNoCount(t *testing.T) {
	f := idxThreadFake(t)
	f.set("GET /api/v1/nodes/44", 200, strings.Replace(fixture(t, "node.json"), `"canPost":false`, `"canPost":true`, 1))
	h := indexHarness(t, NewNode(indexDeps(t, f, true), 44), 80, 19)
	for _, c := range h.top().(ui.Commander).Commands() {
		if c.Name == "N" && !c.NoCount {
			t.Fatal("the hint of N must not show the count of notifications")
		}
	}
	if !idxHasCommand(h.top(), "N") {
		t.Fatal("a member with canPost must get N")
	}
}

// shownRange returns the first and the end index of the threads that the
// last thread request asks for.
func shownRange(t *testing.T, f *fakeSermo) (int, int) {
	t.Helper()
	reqs := f.requests("GET", "/api/v1/nodes/44/threads")
	q, err := url.ParseQuery(reqs[len(reqs)-1].Query)
	if err != nil {
		t.Fatal(err)
	}
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	if page < 1 || limit < 1 {
		t.Fatalf("query = %q", reqs[len(reqs)-1].Query)
	}
	return (page - 1) * limit, page * limit
}

func TestBBSThreadListResizeKeepsThePosition(t *testing.T) {
	f := idxThreadFake(t)
	h := indexHarness(t, NewNode(indexDeps(t, f, true), 44), 80, 19)
	h.runCmd(">", "")
	first, _ := shownRange(t, f)
	h.w, h.h = 120, 35
	h.send(tea.WindowSizeMsg{Width: h.w, Height: h.h})
	lo, hi := shownRange(t, f)
	if first < lo || first >= hi {
		t.Fatalf("after the resize the page must show thread %d, it shows %d to %d", first, lo, hi)
	}
	h.runCmd(">", "")
	next, _ := shownRange(t, f)
	if next != hi {
		t.Fatalf("> after the resize must start at thread %d, it starts at %d", hi, next)
	}
	h.wantView("Página 2 / 2")
}
