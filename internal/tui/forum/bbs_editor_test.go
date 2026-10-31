package forum

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// enterLine types text in the line editor and presses Enter.
func (h *harness) enterLine(text string) {
	h.typeText(text)
	h.press("enter")
}

func bbsEditor(t *testing.T, f *fakeSermo) (*harness, *composer) {
	t.Helper()
	h := bbsHarness(t, NewComposer(bbsTestDeps(t, f, true), ui.ComposeMsg{ThreadID: 34898}))
	c := h.top().(*composer)
	if c.ed == nil {
		t.Fatal("the BBS composer must be a line editor")
	}
	return h, c
}

func TestBBSEditorLayout(t *testing.T) {
	f := newFakeSermo(t)
	h, _ := bbsEditor(t, f)
	h.enterLine("Primera línea")
	h.enterLine("")
	h.typeText("[code]")
	lines := strings.Split(h.view(), "\n")
	if len(lines) != 19 {
		t.Fatalf("view has %d lines:\n%s", len(lines), h.view())
	}
	for i, l := range lines {
		if ansi.StringWidth(l) != 80 {
			t.Fatalf("line %d is %d cells wide: %q", i, ansi.StringWidth(l), l)
		}
	}
	h.wantView("| RESPONDER  >  #34898", "| Editor de líneas    /S enviar  /A abortar  /C citar  /V vista previa  /?",
		"|   1 | Primera línea ", "|   2 |  ", "|   3 | [code]_ ", "| Línea 3/200   Col 7/72   [b]negrita[/b]")
	if !strings.HasPrefix(lines[len(lines)-1], "+---") {
		t.Fatalf("the table must end the view:\n%s", h.view())
	}
}

func TestBBSEditorSendsJoinedLines(t *testing.T) {
	f := newFakeSermo(t)
	f.set("POST /api/v1/threads/34898/posts", 201, newPostJSON)
	h, _ := bbsEditor(t, f)
	h.enterLine("Hola")
	h.enterLine("mundo")
	h.enterLine("/S")
	posts := f.requests("POST", "/api/v1/threads/34898/posts")
	if len(posts) != 1 || decode(t, posts[0].Body)["body"] != "Hola\nmundo" {
		t.Fatalf("reply requests = %+v", posts)
	}
}

func TestBBSEditorShowsServerIssues(t *testing.T) {
	f := newFakeSermo(t)
	f.set("POST /api/v1/threads/34898/posts", 422, `{"error":{"code":"invalid","message":"Datos no válidos","issues":[{"path":["body"],"message":"El mensaje es muy corto."}]}}`)
	h, _ := bbsEditor(t, f)
	h.enterLine("x")
	h.enterLine("/s")
	h.wantView("El mensaje es muy corto.")
	f.set("POST /api/v1/threads/34898/posts", 429, `{"error":{"code":"rate_limited","message":"Espera"}}`)
	h.enterLine("/S")
	h.wantView(i18n.Tf("errors.rateLimited", map[string]any{"Seconds": 1}))
}

func TestBBSEditorAbortAsksWhenChanged(t *testing.T) {
	f := newFakeSermo(t)
	h, _ := bbsEditor(t, f)
	h.enterLine("algo")
	h.enterLine("/A")
	if len(h.stack) != 1 {
		t.Fatal("/A with changes must ask first")
	}
	h.wantView(i18n.T("bbs.editor.discard"))
	h.enterLine("/A")
	if len(h.stack) != 0 {
		t.Fatal("a second /A must close the editor")
	}

	h2, _ := bbsEditor(t, f)
	h2.enterLine("/a")
	if len(h2.stack) != 0 {
		t.Fatal("/A without changes must close the editor at once")
	}
}

func TestBBSEditorPreview(t *testing.T) {
	f := newFakeSermo(t)
	f.set("POST /api/v1/posts/preview", 200, `{"html":"<p><strong>Negrita</strong> lista</p><pre><code>x := 1</code></pre>"}`)
	h, c := bbsEditor(t, f)
	h.enterLine("[B]Negrita[/B] lista")
	h.enterLine("/V")
	reqs := f.requests("POST", "/api/v1/posts/preview")
	if len(reqs) != 1 || decode(t, reqs[0].Body)["body"] != "[B]Negrita[/B] lista" {
		t.Fatalf("preview requests = %+v", reqs)
	}
	h.wantView("|  **Negrita** lista ", "|  | x := 1 ", i18n.T("bbs.editor.previewHint"))
	h.typeText("q")
	if c.preview || c.ed.cur != "" {
		t.Fatalf("a key must close the preview only, cur = %q", c.ed.cur)
	}
	h.wantView("[B]Negrita[/B] lista")
}

func TestBBSEditorEditsLines(t *testing.T) {
	f := newFakeSermo(t)
	h, c := bbsEditor(t, f)
	for _, l := range []string{"uno", "dos", "tres"} {
		h.enterLine(l)
	}
	h.enterLine("/E 2")
	if c.ed.cur != "dos" {
		t.Fatalf("/E 2 must load line 2, cur = %q", c.ed.cur)
	}
	h.press("backspace", "backspace", "backspace")
	h.enterLine("DOS")
	h.enterLine("/D 1")
	h.enterLine("/I 1")
	h.enterLine("cero")
	if got := strings.Join(c.ed.lines, "|"); got != "cero|DOS|tres" {
		t.Fatalf("lines = %q", got)
	}
	h.enterLine("/D 9")
	if h.lastToast() != i18n.T("bbs.editor.noLine") {
		t.Fatalf("toast = %q", h.lastToast())
	}
	h.enterLine("//S es texto")
	if got := strings.Join(c.ed.lines, "|"); got != "cero|/S es texto|DOS|tres" {
		t.Fatalf("// must give a line that starts with /, lines = %q", got)
	}
}

func TestBBSEditorWrapsAt72Columns(t *testing.T) {
	f := newFakeSermo(t)
	h, c := bbsEditor(t, f)
	text := strings.Repeat("palabra ", 11) // 88 cells
	h.typeText(text)
	if len(c.ed.lines) != 1 || ansi.StringWidth(c.ed.lines[0]) > 72 {
		t.Fatalf("lines = %q", c.ed.lines)
	}
	if c.ed.lines[0] != strings.TrimSpace(strings.Repeat("palabra ", 9)) || c.ed.cur != "palabra palabra " {
		t.Fatalf("wrap gave %q and cur %q", c.ed.lines, c.ed.cur)
	}
	h.typeText(strings.Repeat("x", 80))
	if ansi.StringWidth(c.ed.cur) > 72 || ansi.StringWidth(c.ed.lines[len(c.ed.lines)-1]) > 72 {
		t.Fatalf("a long word must break at 72, lines %q cur %q", c.ed.lines, c.ed.cur)
	}
}

func TestBBSEditorLineLimit(t *testing.T) {
	f := newFakeSermo(t)
	h, c := bbsEditor(t, f)
	for range maxLines {
		h.press("enter")
	}
	h.enterLine("una más")
	if len(c.ed.lines) != maxLines || h.lastToast() != i18n.T("bbs.editor.full") {
		t.Fatalf("lines %d, toast %q", len(c.ed.lines), h.lastToast())
	}
}

func TestBBSEditorQuoteCommand(t *testing.T) {
	f := bbsFake(t)
	f.set("GET /api/v1/posts/478003", 200, fixture(t, "post.json"))
	h := bbsHarness(t, NewThread(bbsTestDeps(t, f, true), 34898, 0))
	h.command("R")
	c := h.top().(*composer)
	h.enterLine("Hola")
	h.enterLine("/C 2")
	want := `[QUOTE="Anierrr, post: 478003, member: 1825"][B][I]Las leyendas nunca mueren.[/I][/B][/QUOTE]`
	if len(c.ed.lines) != 2 || c.ed.lines[1] != want {
		t.Fatalf("lines = %q", c.ed.lines)
	}
}

func TestBBSEditorNewThread(t *testing.T) {
	f := newFakeSermo(t)
	f.set("GET /api/v1/thread-prefixes", 200, `{"items":[{"id":5,"label":"Ayuda","color":"#ff0000","colorId":1,"icon":"","isActive":true,"position":1,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}]}`)
	f.set("POST /api/v1/nodes/44/threads", 201, `{"thread":{"id":900},"post":{"id":901,"position":0}}`)
	h := bbsHarness(t, NewComposer(bbsTestDeps(t, f, true), ui.ComposeMsg{NodeID: 44}))
	h.wantView("NUEVO TEMA  >  Foro Libre")
	h.enterLine("/T Mi tema")
	h.enterLine("/P")
	h.wantView("01", "Ayuda")
	h.enterLine("/P 1")
	h.wantView("Mi tema", "[Ayuda]")
	h.enterLine("Cuerpo")
	h.enterLine("/S")
	reqs := f.requests("POST", "/api/v1/nodes/44/threads")
	if len(reqs) != 1 {
		t.Fatalf("thread requests = %+v", reqs)
	}
	body := decode(t, reqs[0].Body)
	if body["title"] != "Mi tema" || body["body"] != "Cuerpo" || body["prefixId"] != float64(5) {
		t.Fatalf("thread body = %v", body)
	}
}

func TestBBSEditorEditHeader(t *testing.T) {
	f := bbsFake(t)
	f.set("GET /api/v1/posts/478003", 200, fixture(t, "post.json"))
	h := bbsHarness(t, NewThread(bbsTestDeps(t, f, true), 34898, 0))
	h.command("E 2")
	c, ok := h.top().(*composer)
	if !ok || c.mode != modeEdit {
		t.Fatalf("E 2 must edit post #2, top = %T", h.top())
	}
	h.wantView("EDITAR  >  #2", "[B][I]Las leyendas nunca mueren.[/I][/B]")
	h.toasts = nil
	h.press("esc")
	if len(h.stack) != 1 {
		t.Fatal("esc without changes must close the editor")
	}
	h.command("E 1")
	if h.lastToast() != i18n.T("forum.toast.cannotEdit") {
		t.Fatalf("toast = %q", h.lastToast())
	}
}

func TestBBSEditorInsertKeepsLineNumbers(t *testing.T) {
	f := newFakeSermo(t)
	h, c := bbsEditor(t, f)
	for _, l := range []string{"a", "b", "c"} {
		h.enterLine(l)
	}
	h.enterLine("/I 2")
	h.wantView("|   1 | a ", "|   > | _ ", "|   2 | b ", "|   3 | c ")
	if strings.Contains(h.view(), "|   4 |") {
		t.Fatalf("an insert must not change the numbers of the lines:\n%s", h.view())
	}
	h.enterLine("/D 3")
	if got := strings.Join(c.ed.lines, "|"); got != "a|b" {
		t.Fatalf("/D 3 must delete the line shown as 3, lines = %q", got)
	}
	h.enterLine("/L 2")
	h.wantView("|   2 | b ")
	if strings.Contains(h.view(), "|   1 | a ") {
		t.Fatalf("/L 2 must list from the line shown as 2:\n%s", h.view())
	}
}

func TestBBSEditorEscAsksForTheTypedLine(t *testing.T) {
	f := newFakeSermo(t)
	h, _ := bbsEditor(t, f)
	h.typeText("sin Enter")
	h.press("esc")
	if len(h.stack) != 1 {
		t.Fatal("esc with a typed line must ask first")
	}
	h.wantView(i18n.T("bbs.editor.discard"))
	h.press("esc")
	if len(h.stack) != 0 {
		t.Fatal("a second esc must close the editor")
	}
}

func TestBBSEditorEscAfterEditCommand(t *testing.T) {
	f := bbsFake(t)
	f.set("GET /api/v1/posts/478003", 200, fixture(t, "post.json"))
	h := bbsHarness(t, NewThread(bbsTestDeps(t, f, true), 34898, 0))
	h.command("E 2")
	h.enterLine("/E 1")
	h.press("esc")
	if len(h.stack) != 1 {
		t.Fatal("esc after /E without a change must close the editor at once")
	}
	h.command("E 2")
	h.enterLine("/E 1")
	h.typeText("!")
	h.press("esc")
	if len(h.stack) != 2 {
		t.Fatal("esc with a changed line under edit must ask first")
	}
}
