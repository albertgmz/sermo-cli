package forum

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// host runs one screen as a Bubble Tea program for teatest.
type host struct{ s ui.Screen }

func (m host) Init() tea.Cmd { return m.s.Init() }

func (m host) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && k.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		msg = tea.WindowSizeMsg{Width: ws.Width, Height: ws.Height}
	}
	s, cmd := m.s.Update(msg)
	m.s = s
	return m, cmd
}

func (m host) View() tea.View { return tea.NewView(m.s.View(100, 30)) }

func TestNodesShowCategoriesForumsAndUnreadMarkers(t *testing.T) {
	f := newFakeSermo(t)
	d := testDeps(t, f, true)
	tm := teatest.NewTestModel(t, host{NewNodes(d)}, teatest.WithInitialTermSize(100, 30))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return strings.Contains(string(b), "Foro Libre") && strings.Contains(string(b), "Humanidades")
	}, teatest.WithDuration(3*time.Second))
	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))

	h := newHarness(t, NewNodes(d))
	h.wantView("Comunidad", "Humanidades, Ciencias y Artes", "Foro Libre", "│  └─")
	lines := strings.Split(h.view(), "\n")
	for _, ln := range lines {
		switch {
		case strings.Contains(ln, "Bienvenidas"):
			if !strings.Contains(ln, "●") {
				t.Errorf("unread forum has no bright marker: %q", ln)
			}
		case strings.Contains(ln, "Foro Libre"):
			if !strings.Contains(ln, "○") || strings.Contains(ln, "●") {
				t.Errorf("read forum must have the dim marker: %q", ln)
			}
		case strings.Contains(ln, "Anuncios"):
			if !strings.Contains(ln, "├─") {
				t.Errorf("forum with a later sibling has no tree glyph: %q", ln)
			}
		}
	}
}

func TestNodesEnterOpensForumAndSkipsCategories(t *testing.T) {
	f := newFakeSermo(t)
	h := newHarness(t, NewNodes(testDeps(t, f, false)))
	h.press("enter")
	if len(h.intent) != 1 || h.intent[0] != (ui.OpenNodeMsg{NodeID: 45}) {
		t.Fatalf("enter gave %#v, want OpenNodeMsg 45", h.intent)
	}
	h.press("down", "down", "down", "down", "enter")
	if got := h.intent[len(h.intent)-1]; got != (ui.OpenNodeMsg{NodeID: 68}) {
		t.Fatalf("after the category the cursor must be on 68, got %#v", got)
	}
	h.press("g", "enter")
	if got := h.intent[len(h.intent)-1]; got != (ui.OpenNodeMsg{NodeID: 45}) {
		t.Fatalf("g must go to the first forum, got %#v", got)
	}
	if n := len(f.requests("GET", "/api/v1/nodes/unread")); n != 0 {
		t.Fatalf("a guest must not ask for unread forums, got %d requests", n)
	}
}

func TestNodesMarkAllReadNeedsMember(t *testing.T) {
	f := newFakeSermo(t)
	h := newHarness(t, NewNodes(testDeps(t, f, false)))
	h.press("a")
	if h.lastToast() != i18n.T("forum.toast.loginNeeded") {
		t.Fatalf("toast = %q", h.lastToast())
	}
	if len(f.requests("PUT", "/api/v1/nodes/read")) != 0 {
		t.Fatal("a guest must send nothing")
	}

	f.set("PUT /api/v1/nodes/read", 200, `{"ok":true}`)
	m := newHarness(t, NewNodes(testDeps(t, f, true)))
	f.set("GET /api/v1/nodes/unread", 200, `{"nodeIds":[]}`)
	m.press("a")
	if len(f.requests("PUT", "/api/v1/nodes/read")) != 1 {
		t.Fatal("a must send nodes.markAllRead")
	}
	if strings.Contains(m.view(), "●") {
		t.Fatalf("all forums must show as read:\n%s", m.view())
	}
}

func TestNodesWatchForumToggles(t *testing.T) {
	f := newFakeSermo(t)
	f.set("PUT /api/v1/nodes/45/watch", 200, `{"ok":true}`)
	f.set("GET /api/v1/nodes/45", 200, strings.Replace(fixture(t, "node.json"), `"id":44,"parentId"`, `"id":45,"parentId"`, 1))
	h := newHarness(t, NewNodes(testDeps(t, f, true)))
	h.press("w")
	reqs := f.requests("PUT", "/api/v1/nodes/45/watch")
	if len(reqs) != 1 || decode(t, reqs[0].Body)["mode"] != "threads" {
		t.Fatalf("watch requests = %+v", reqs)
	}
	if h.lastToast() != i18n.T("forum.toast.watchingForum") {
		t.Fatalf("toast = %q", h.lastToast())
	}
}

func TestNodesErrorShowsRetry(t *testing.T) {
	f := newFakeSermo(t)
	f.set("GET /api/v1/nodes", 500, `{"error":{"code":"internal","message":"boom"}}`)
	h := newHarness(t, NewNodes(testDeps(t, f, false)))
	h.wantView(i18n.T("errors.server"))
	f.set("GET /api/v1/nodes", 200, fixture(t, "nodes.json"))
	h.press("r")
	h.wantView("Foro Libre")
}

func TestNodesKeysHaveHelp(t *testing.T) {
	for _, k := range NewNodes(testDeps(t, newFakeSermo(t), false)).Keys() {
		if k.Help().Desc == "" || strings.HasPrefix(k.Help().Desc, "forum.") {
			t.Fatalf("key without help text: %#v", k.Help())
		}
	}
}

func TestNodesStopAnimatingWhenIdle(t *testing.T) {
	f := newFakeSermo(t)
	d := testDeps(t, f, false)
	d.NoAnim = false
	now := d.Now()
	d.Now = func() time.Time { return now }
	h := newHarness(t, NewNodes(d))
	a, ok := h.top().(ui.Animator)
	if !ok || !a.Animating() {
		t.Fatal("the highlight must animate after the forums load")
	}
	now = now.Add(2 * time.Second)
	if a.Animating() {
		t.Fatal("an idle forum list must stop the animation after 2 s")
	}
	h.press("down")
	if !a.Animating() {
		t.Fatal("a cursor move must start the animation again")
	}
}

func TestNodesResumeReloadsOnlyUnreadMarkers(t *testing.T) {
	f := newFakeSermo(t)
	h := newHarness(t, NewNodes(testDeps(t, f, true)))
	if !strings.Contains(h.view(), "●") {
		t.Fatalf("the unread forum must show the marker:\n%s", h.view())
	}
	f.set("GET /api/v1/nodes/unread", 200, `{"nodeIds":[]}`)
	h.send(ui.ResumedMsg{})
	if strings.Contains(h.view(), "●") {
		t.Fatalf("after resume the forum must show as read:\n%s", h.view())
	}
	if n := len(f.requests("GET", "/api/v1/nodes/unread")); n != 2 {
		t.Fatalf("unread requests = %d, want 2", n)
	}
	if n := len(f.requests("GET", "/api/v1/nodes")); n != 1 {
		t.Fatalf("tree requests = %d, want 1", n)
	}

	g := newHarness(t, NewNodes(testDeps(t, f, false)))
	g.send(ui.ResumedMsg{})
	if n := len(f.requests("GET", "/api/v1/nodes/unread")); n != 2 {
		t.Fatalf("a guest must not ask for unread forums, got %d requests", n)
	}
}
