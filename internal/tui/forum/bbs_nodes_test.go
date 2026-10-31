package forum

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albertgmz/sermo-cli/internal/config"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

const idxOnline = `{"items":[
 {"type":"member","card":{"id":6,"username":"Lucien","avatar":null,"frame":null,"nameColor":null,"nameIcon":null,"title":"","badges":[],"level":1,"restricted":false},"robot":null,"ip":null,"activity":{"at":"2026-10-31T14:00:00Z","hidden":false,"place":null}},
 {"type":"guest","card":null,"robot":null,"ip":null,"activity":{"at":"2026-10-31T14:00:00Z","hidden":false,"place":null}},
 {"type":"member","card":{"id":9,"username":"kz0","avatar":null,"frame":null,"nameColor":null,"nameIcon":null,"title":"","badges":[],"level":1,"restricted":false},"robot":null,"ip":null,"activity":{"at":"2026-10-31T14:00:00Z","hidden":false,"place":null}}
],"page":1,"perPage":25,"total":37}`

const idxStats = `{"threads":2758,"posts":41462,"members":4182,"latestMember":null}`

// indexFake returns the fake server with the online list, the site totals
// and a read-only forum 37.
func indexFake(t *testing.T) *fakeSermo {
	t.Helper()
	f := newFakeSermo(t)
	f.set("GET /api/v1/online", 200, idxOnline)
	f.set("GET /api/v1/site/stats", 200, idxStats)
	f.set("GET /api/v1/nodes", 200, patched(t, "nodes.json", func(v map[string]any) {
		for _, it := range v["items"].([]any) {
			n := it.(map[string]any)
			if n["id"].(float64) == 37 {
				n["settings"].(map[string]any)["isReadOnly"] = true
			}
		}
	}))
	return f
}

// indexDeps returns Deps for the fake server with the BBS interface.
func indexDeps(t *testing.T, f *fakeSermo, member bool) *ui.Deps {
	d := testDeps(t, f, member)
	d.Config = &config.Config{Interface: config.InterfaceBBS}
	return d
}

// indexHarness hosts s in the content area of an 80x24 terminal: 80x19.
func indexHarness(t *testing.T, s ui.Screen, w, h int) *harness {
	t.Helper()
	hs := &harness{t: t, w: w, h: h}
	hs.push(s)
	return hs
}

// runCmd runs the BBS command name of the top screen with arg.
func (h *harness) runCmd(name, arg string) {
	h.t.Helper()
	c, ok := h.top().(ui.Commander)
	if !ok {
		h.t.Fatalf("%T is no Commander", h.top())
	}
	for _, cmd := range c.Commands() {
		if cmd.Run != nil && strings.EqualFold(cmd.Name, name) {
			h.run(cmd.Run(arg))
			return
		}
	}
	h.t.Fatalf("no command %q", name)
}

func idxHasCommand(s ui.Screen, name string) bool {
	for _, cmd := range s.(ui.Commander).Commands() {
		if cmd.Run != nil && strings.EqualFold(cmd.Name, name) {
			return true
		}
	}
	return false
}

func idxLineWith(t *testing.T, view, text string) string {
	t.Helper()
	for _, ln := range strings.Split(view, "\n") {
		if strings.Contains(ln, text) {
			return ln
		}
	}
	t.Fatalf("no line with %q:\n%s", text, view)
	return ""
}

// wantWidth checks that each line of v is w cells wide.
func wantWidth(t *testing.T, v string, w int) {
	t.Helper()
	for _, ln := range strings.Split(v, "\n") {
		if n := len([]rune(ln)); n != w {
			t.Fatalf("line is %d cells, want %d: %q", n, w, ln)
		}
	}
}

// wantRows checks that the line with each key text starts with its value.
func wantRows(t *testing.T, v string, rows map[string]string) {
	t.Helper()
	for text, start := range rows {
		if ln := idxLineWith(t, v, text); !strings.HasPrefix(ln, start) {
			t.Errorf("row of %q = %q, want the start %q", text, ln, start)
		}
	}
}

// idxCapture saves the view to the scratchpad when SERMO_BBSCAP names a
// directory.
func idxCapture(t *testing.T, h *harness, name string) {
	t.Helper()
	dir := os.Getenv("SERMO_BBSCAP")
	if dir == "" {
		return
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(h.view()+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestBBSIndexShowsTableSectionsMarkersAndRows(t *testing.T) {
	f := indexFake(t)
	h := indexHarness(t, NewNodes(indexDeps(t, f, true)), 120, 35)
	v := h.view()
	idxCapture(t, h, "forum-index-120x40.txt")
	wantWidth(t, v, 120)
	h.wantView("ÍNDICE DE FOROS", "[N]uevos: 1", "| ## | Foro", "| Hilos | Posts", "Último mensaje",
		"===  COMUNIDAD  ===", "===  HUMANIDADES, CIENCIAS Y ARTES  ===",
		"(*) mensajes nuevos", "( ) sin novedad", "(L) cerrado",
		"En línea (37): Lucien, kz0 ... +35", "Miembros: 4.182", "Temas: 2.758", "Mensajes: 41.462")
	wantRows(t, v, map[string]string{
		"Bienvenidas":             "| 01 | (*) Bienvenidas",
		"Anuncios":                "| 02 | (L) Anuncios",
		"| 03 |":                  "| 03 |   ( ) Tutor",
		"Habla de lo que quieras": "|    | Habla de lo que quieras",
	})
	foro := idxLineWith(t, v, "Foro Libre")
	for _, want := range []string{"| 04 | ( ) Foro Libre", "1.784", "29.647", "16d", "Valek"} {
		if !strings.Contains(foro, want) {
			t.Errorf("forum row has no %q: %q", want, foro)
		}
	}
}

func TestBBSIndexGuestHidesUnreadCountAndKeepsTheRowsWithoutExtras(t *testing.T) {
	f := newFakeSermo(t)
	f.set("GET /api/v1/online", 500, `{"error":{"code":"x","message":"x"}}`)
	f.set("GET /api/v1/site/stats", 500, `{"error":{"code":"x","message":"x"}}`)
	h := indexHarness(t, NewNodes(indexDeps(t, f, false)), 80, 40)
	v := h.view()
	if strings.Contains(v, "[N]uevos") || strings.Contains(v, "En línea") || strings.Contains(v, "Miembros") {
		t.Fatalf("guest view with failed extras:\n%s", v)
	}
	h.wantView("ÍNDICE DE FOROS", "Foro Libre", "(*) mensajes nuevos")
	if idxHasCommand(h.top(), "N") || idxHasCommand(h.top(), "L") {
		t.Fatal("a guest must not get N or L")
	}
}

func TestBBSIndexNumberOpensForum(t *testing.T) {
	f := indexFake(t)
	h := indexHarness(t, NewNodes(indexDeps(t, f, false)), 80, 19)
	h.runCmd("#", "03")
	if len(h.intent) != 1 || h.intent[0] != (ui.OpenNodeMsg{NodeID: 64}) {
		t.Fatalf("03 gave %#v, want OpenNodeMsg 64", h.intent)
	}
	h.runCmd("#", "9")
	if len(h.intent) != 1 || h.lastToast() == "" {
		t.Fatalf("a number with no forum must give a toast, got %#v %q", h.intent, h.lastToast())
	}
}

func TestBBSIndexNewGoesToFirstUnreadForum(t *testing.T) {
	f := indexFake(t)
	f.set("GET /api/v1/nodes/unread", 200, `{"nodeIds":[68,44]}`)
	h := indexHarness(t, NewNodes(indexDeps(t, f, true)), 80, 19)
	h.runCmd("N", "")
	if len(h.intent) != 1 || h.intent[0] != (ui.OpenNodeMsg{NodeID: 44}) {
		t.Fatalf("N gave %#v, want OpenNodeMsg 44", h.intent)
	}
	f.set("GET /api/v1/nodes/unread", 200, `{"nodeIds":[]}`)
	h.send(ui.ResumedMsg{})
	h.runCmd("N", "")
	if len(h.intent) != 1 || h.lastToast() == "" {
		t.Fatalf("N without unread forums must give a toast, got %#v", h.intent)
	}
}

func TestBBSIndexUsersAsksForTheOnlineList(t *testing.T) {
	f := indexFake(t)
	h := indexHarness(t, NewNodes(indexDeps(t, f, false)), 80, 19)
	for _, cmd := range h.top().(ui.Commander).Commands() {
		if cmd.Name == "U" {
			if msg := cmd.Run("")(); msg != (ui.OpenOnlineMsg{}) {
				t.Fatalf("U gave %#v", msg)
			}
			return
		}
	}
	t.Fatal("no U command")
}

func TestBBSIndexMarkAllReadAndRefresh(t *testing.T) {
	f := indexFake(t)
	f.set("PUT /api/v1/nodes/read", 200, `{"ok":true}`)
	h := indexHarness(t, NewNodes(indexDeps(t, f, true)), 80, 40)
	h.runCmd("L", "")
	if len(f.requests("PUT", "/api/v1/nodes/read")) != 1 {
		t.Fatal("L must send nodes.markAllRead")
	}
	if strings.Contains(h.view(), "(*) Bienvenidas") {
		t.Fatalf("all forums must show as read:\n%s", h.view())
	}
	before := len(f.requests("GET", "/api/v1/nodes"))
	h.runCmd("R", "")
	if len(f.requests("GET", "/api/v1/nodes")) != before+1 {
		t.Fatal("R must load the tree again")
	}
}

func TestBBSIndexPagesATallTree(t *testing.T) {
	f := indexFake(t)
	h := indexHarness(t, NewNodes(indexDeps(t, f, true)), 80, 19)
	idxCapture(t, h, "forum-index-80x24.txt")
	v := h.view()
	if n := strings.Count(v, "\n") + 1; n > 19 {
		t.Fatalf("view has %d lines, want 19 or less:\n%s", n, v)
	}
	h.wantView("Página 1 / 2", "===  COMUNIDAD  ===", "Bienvenidas", "En línea (37)", "Miembros: 4.182")
	if strings.Contains(v, "HUMANIDADES") {
		t.Fatalf("the second section must go to page 2:\n%s", v)
	}
	h.runCmd(">", "")
	idxCapture(t, h, "forum-index-80x24-p2.txt")
	h.wantView("Página 2 / 2", "===  HUMANIDADES, CIENCIAS Y ARTES  ===", "05 | ( ) Filosof")
	h.runCmd("<", "")
	h.wantView("Página 1 / 2")
	h.press("pgdown")
	h.wantView("Página 2 / 2")
}

func TestModernIndexHasNoBBSRequests(t *testing.T) {
	f := indexFake(t)
	h := newHarness(t, NewNodes(testDeps(t, f, true)))
	if len(f.requests("GET", "/api/v1/online")) != 0 || len(f.requests("GET", "/api/v1/site/stats")) != 0 {
		t.Fatal("the modern home must not load the BBS rows")
	}
	if strings.Contains(h.view(), "ÍNDICE DE FOROS") {
		t.Fatal("the modern home must keep its look")
	}
}
