package forum

import (
	"strings"
	"testing"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

func TestThreadListShowsThreadsAndBreadcrumbs(t *testing.T) {
	f := newFakeSermo(t)
	h := newHarness(t, NewNode(testDeps(t, f, false), 44))
	h.wantView("Comunidad", "Foro Libre", "Comic del Foro", "Ay bbs pq dejaron morir esto?", "1/3")
	if h.top().Title() != "Foro Libre" {
		t.Fatalf("title = %q", h.top().Title())
	}
	for _, ln := range strings.Split(h.view(), "\n") {
		if strings.Contains(ln, "Comic del Foro") && !strings.Contains(ln, glyphSticky) {
			t.Fatalf("sticky thread has no glyph: %q", ln)
		}
	}
}

func TestThreadListNextPageSendsPage2(t *testing.T) {
	f := newFakeSermo(t)
	h := newHarness(t, NewNode(testDeps(t, f, false), 44))
	h.press("l")
	reqs := f.requests("GET", "/api/v1/nodes/44/threads")
	last := reqs[len(reqs)-1]
	if !strings.Contains(last.Query, "page=2") {
		t.Fatalf("query = %q, want page=2", last.Query)
	}
	h.wantView("2/3")
	h.press("h")
	reqs = f.requests("GET", "/api/v1/nodes/44/threads")
	if q := reqs[len(reqs)-1].Query; strings.Contains(q, "page=2") {
		t.Fatalf("h must go back to page 1, query = %q", q)
	}
	h.press("h")
	if n := len(f.requests("GET", "/api/v1/nodes/44/threads")); n != len(reqs) {
		t.Fatal("h on page 1 must not send a request")
	}
}

func TestThreadListEnterOpensThread(t *testing.T) {
	f := newFakeSermo(t)
	h := newHarness(t, NewNode(testDeps(t, f, false), 44))
	h.press("down", "enter")
	if len(h.intent) != 1 || h.intent[0] != (ui.OpenThreadMsg{ThreadID: 34898}) {
		t.Fatalf("intent = %#v", h.intent)
	}
}

func TestThreadListPrefixFilter(t *testing.T) {
	f := newFakeSermo(t)
	f.set("GET /api/v1/thread-prefixes", 200, `{"items":[{"id":5,"label":"Ayuda","color":"#60a5fa","icon":"","colorId":18,"position":0,"isActive":true,"createdAt":"2026-10-16T23:25:10.721Z","updatedAt":"2026-10-16T23:25:10.721Z"}]}`)
	h := newHarness(t, NewNode(testDeps(t, f, false), 44))
	h.press("x")
	h.wantView("Ayuda")
	h.press("down", "enter")
	reqs := f.requests("GET", "/api/v1/nodes/44/threads")
	if q := reqs[len(reqs)-1].Query; !strings.Contains(q, "prefixId=5") {
		t.Fatalf("query = %q, want prefixId=5", q)
	}
}

func TestThreadListNewThreadNeedsPermission(t *testing.T) {
	f := newFakeSermo(t)
	h := newHarness(t, NewNode(testDeps(t, f, false), 44))
	h.press("t")
	if h.lastToast() != i18n.T("forum.toast.loginNeeded") {
		t.Fatalf("guest toast = %q", h.lastToast())
	}
	m := newHarness(t, NewNode(testDeps(t, f, true), 44))
	m.press("t")
	if m.lastToast() != i18n.T("forum.toast.cannotPost") {
		t.Fatalf("member toast = %q", m.lastToast())
	}
	f.set("GET /api/v1/nodes/44", 200, strings.Replace(fixture(t, "node.json"), `"canPost":false`, `"canPost":true`, 1))
	p := newHarness(t, NewNode(testDeps(t, f, true), 44))
	p.press("t")
	if _, ok := p.top().(*composer); !ok {
		t.Fatalf("t must open the composer, top = %T", p.top())
	}
}

func TestThreadListMarkReadAndWatch(t *testing.T) {
	f := newFakeSermo(t)
	f.set("PUT /api/v1/nodes/44/read", 200, `{"ok":true}`)
	f.set("PUT /api/v1/nodes/44/watch", 200, `{"ok":true}`)
	h := newHarness(t, NewNode(testDeps(t, f, true), 44))
	h.press("a")
	if len(f.requests("PUT", "/api/v1/nodes/44/read")) != 1 {
		t.Fatal("a must mark the forum read")
	}
	h.press("w")
	if len(f.requests("PUT", "/api/v1/nodes/44/watch")) != 1 {
		t.Fatal("w must watch the forum")
	}
	f.set("DELETE /api/v1/nodes/44/watch", 200, `{"ok":true}`)
	h.press("w")
	if len(f.requests("DELETE", "/api/v1/nodes/44/watch")) != 1 {
		t.Fatal("second w must stop the watch")
	}
}

func TestThreadListNotFound(t *testing.T) {
	f := newFakeSermo(t)
	h := newHarness(t, NewNode(testDeps(t, f, false), 999))
	h.wantView(i18n.T("errors.notFound"))
}

func TestThreadListIgnoresThreadWatchResult(t *testing.T) {
	f := newFakeSermo(t)
	h := newHarness(t, NewNode(testDeps(t, f, true), 44))
	h.send(watchedMsg{on: true, forum: false})
	if strings.Contains(h.view(), i18n.T("forum.node.watching")) {
		t.Fatalf("a thread watch result must not change the forum watch state:\n%s", h.view())
	}
	h.send(watchedMsg{on: true, forum: true})
	h.wantView(i18n.T("forum.node.watching"))
}

func TestThreadListResumeReloadsPageAndKeepsCursor(t *testing.T) {
	f := newFakeSermo(t)
	h := newHarness(t, NewNode(testDeps(t, f, true), 44))
	h.press("l")
	h.press("down")
	before := len(f.requests("GET", "/api/v1/nodes/44/threads"))
	h.send(ui.ResumedMsg{})
	reqs := f.requests("GET", "/api/v1/nodes/44/threads")
	if len(reqs) != before+1 || !strings.Contains(reqs[len(reqs)-1].Query, "page=2") {
		t.Fatalf("resume must reload the current page, requests %+v", reqs)
	}
	h.press("enter")
	if got := h.intent[len(h.intent)-1]; got != (ui.OpenThreadMsg{ThreadID: 34898}) {
		t.Fatalf("resume must keep the cursor, enter gave %#v", got)
	}
}
