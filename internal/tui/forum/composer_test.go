package forum

import (
	"strings"
	"testing"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

const newPostJSON = `{"id":478500,"threadId":34898,"position":2,"author":{"kind":"member","id":7,"username":"albert"},"bodyHtml":"<p>x</p>","bodySource":"x","state":"visible","createdAt":"2026-10-31T14:05:00Z","editedAt":null,"attachmentCount":0,"attachments":[],"reactions":{"counts":{},"total":0,"mine":null}}`

func replyComposer(t *testing.T, f *fakeSermo) *harness {
	t.Helper()
	return newHarness(t, NewComposer(testDeps(t, f, true), ui.ComposeMsg{ThreadID: 34898}))
}

func TestComposerReplyForbiddenShowsFriendlyError(t *testing.T) {
	f := newFakeSermo(t)
	f.set("POST /api/v1/threads/34898/posts", 403, `{"error":{"code":"forbidden","message":"No."}}`)
	h := replyComposer(t, f)
	h.typeText("Hola")
	h.press("ctrl+s")
	if len(f.requests("POST", "/api/v1/threads/34898/posts")) != 1 {
		t.Fatal("ctrl+s must send the reply")
	}
	h.wantView(i18n.T("errors.forbidden"))
	if _, ok := h.top().(*composer); !ok {
		t.Fatal("the composer must stay open after an error")
	}
}

func TestComposerShowsValidationIssuesAndRateLimit(t *testing.T) {
	f := newFakeSermo(t)
	f.set("POST /api/v1/threads/34898/posts", 422, `{"error":{"code":"invalid","message":"Datos no válidos","issues":[{"path":["body"],"message":"El mensaje es muy corto."}]}}`)
	h := replyComposer(t, f)
	h.typeText("x")
	h.press("ctrl+s")
	h.wantView("El mensaje es muy corto.")
	f.set("POST /api/v1/threads/34898/posts", 429, `{"error":{"code":"rate_limited","message":"Espera"}}`)
	h.press("ctrl+s")
	h.wantView(i18n.Tf("errors.rateLimited", map[string]any{"Seconds": 1}))
}

func TestComposerPreviewCallsServer(t *testing.T) {
	f := newFakeSermo(t)
	f.set("POST /api/v1/posts/preview", 200, `{"html":"<p><strong>Negrita</strong> lista</p>"}`)
	h := replyComposer(t, f)
	h.typeText("[B]Negrita[/B] lista")
	h.press("ctrl+p")
	reqs := f.requests("POST", "/api/v1/posts/preview")
	if len(reqs) != 1 {
		t.Fatalf("preview requests = %d", len(reqs))
	}
	body := decode(t, reqs[0].Body)
	target, _ := body["target"].(map[string]any)
	if body["body"] != "[B]Negrita[/B] lista" || target["type"] != "reply" || target["threadId"] != float64(34898) {
		t.Fatalf("preview body = %v", body)
	}
	h.wantView("Negrita", "lista")
	h.press("ctrl+p")
	h.wantView("[B]Negrita[/B] lista")
}

func TestComposerWrapsWithBBCode(t *testing.T) {
	f := newFakeSermo(t)
	h := replyComposer(t, f)
	c := h.top().(*composer)
	h.press("ctrl+b")
	h.typeText("hola")
	if c.body.Value() != "[B]hola[/B]" {
		t.Fatalf("body = %q", c.body.Value())
	}
	c.body.SelectAll()
	h.press("ctrl+u")
	if c.body.Value() != "[U][B]hola[/B][/U]" {
		t.Fatalf("selection wrap gave %q", c.body.Value())
	}
}

func TestComposerCountsCharacters(t *testing.T) {
	f := newFakeSermo(t)
	h := replyComposer(t, f)
	h.typeText("ñandú")
	h.wantView("5/10000")
}

func TestComposerEscAsksBeforeDiscard(t *testing.T) {
	f := newFakeSermo(t)
	h := newHarness(t, NewNodes(testDeps(t, f, true)))
	h.push(NewComposer(testDeps(t, f, true), ui.ComposeMsg{ThreadID: 34898}))
	h.press("esc")
	if len(h.stack) != 1 {
		t.Fatal("esc on an empty composer must close it")
	}
	h.push(NewComposer(testDeps(t, f, true), ui.ComposeMsg{ThreadID: 34898}))
	h.typeText("algo")
	h.press("esc")
	if len(h.stack) != 2 {
		t.Fatal("esc with text must ask first")
	}
	h.wantView(i18n.T("forum.compose.discard"))
	h.press("x")
	if len(h.stack) != 2 || h.top().(*composer).body.Value() != "algo" {
		t.Fatal("another key must keep the text")
	}
	h.press("esc", "esc")
	if len(h.stack) != 1 {
		t.Fatal("esc twice must discard")
	}
}

func TestComposerNewThreadSendsTitleAndOpensThread(t *testing.T) {
	f := newFakeSermo(t)
	f.set("POST /api/v1/nodes/44/threads", 201, `{"thread":`+threadJSON(t)+`,"post":`+newPostJSON+`}`)
	h := newHarness(t, NewComposer(testDeps(t, f, true), ui.ComposeMsg{NodeID: 44}))
	h.typeText("Un tema")
	h.press("tab")
	h.typeText("El cuerpo")
	h.press("ctrl+s")
	reqs := f.requests("POST", "/api/v1/nodes/44/threads")
	if len(reqs) != 1 {
		t.Fatalf("create requests = %d", len(reqs))
	}
	body := decode(t, reqs[0].Body)
	if body["title"] != "Un tema" || body["body"] != "El cuerpo" {
		t.Fatalf("create body = %v", body)
	}
	if _, ok := h.top().(*reader); !ok {
		t.Fatalf("the new thread must open, top = %T", h.top())
	}
}

func TestComposerRequiredPrefix(t *testing.T) {
	f := newFakeSermo(t)
	f.set("GET /api/v1/nodes/44", 200, strings.Replace(fixture(t, "node.json"), `"requirePrefix":false`, `"requirePrefix":true`, 1))
	f.set("GET /api/v1/thread-prefixes", 200, `{"items":[{"id":5,"label":"Ayuda","color":"#60a5fa","icon":"","colorId":18,"position":0,"isActive":true,"createdAt":"2026-10-16T23:25:10.721Z","updatedAt":"2026-10-16T23:25:10.721Z"}]}`)
	f.set("POST /api/v1/nodes/44/threads", 201, `{"thread":`+threadJSON(t)+`,"post":`+newPostJSON+`}`)
	h := newHarness(t, NewComposer(testDeps(t, f, true), ui.ComposeMsg{NodeID: 44}))
	h.typeText("Un tema")
	h.press("tab")
	h.wantView(i18n.T("forum.compose.noPrefix"))
	h.press("ctrl+s")
	if len(f.requests("POST", "/api/v1/nodes/44/threads")) != 0 {
		t.Fatal("a required prefix must be chosen before send")
	}
	h.wantView(i18n.T("forum.compose.prefixRequired"))
	h.press("right")
	h.wantView("Ayuda")
	h.press("tab")
	h.typeText("cuerpo")
	h.press("ctrl+s")
	reqs := f.requests("POST", "/api/v1/nodes/44/threads")
	if len(reqs) != 1 || decode(t, reqs[0].Body)["prefixId"] != float64(5) {
		t.Fatalf("create requests = %+v", reqs)
	}
}

func threadJSON(t *testing.T) string {
	t.Helper()
	v := decode(t, fixture(t, "thread.json"))
	return mustJSON(t, v["thread"])
}
