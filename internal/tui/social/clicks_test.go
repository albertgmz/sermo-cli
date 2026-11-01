package social

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	zone "github.com/lrstanley/bubblezone/v2"

	"github.com/albertgmz/sermo-cli/internal/tui/ui"
	"github.com/albertgmz/sermo-cli/internal/tui/ui/zonetest"
)

// withZones gives d a zone manager, so that the views mark their zones.
func withZones(t *testing.T, d *ui.Deps) *ui.Deps {
	t.Helper()
	d.Zones = zone.New()
	t.Cleanup(d.Zones.Close)
	return d
}

// click draws the screen with its markers and clicks the zone id.
func (h *harness) click(d *ui.Deps, id string) {
	h.t.Helper()
	h.send(zonetest.Click(h.t, d.Zones, h.s.View(h.w, h.h), id))
}

// clickRight clicks the last cell of the zone id.
func (h *harness) clickRight(d *ui.Deps, id string) {
	h.t.Helper()
	z := zonetest.Zone(h.t, d.Zones, h.s.View(h.w, h.h), id)
	h.send(zonetest.At(z.EndX, z.StartY))
}

// startMode starts the screen of a member with zones in the modern or
// the BBS interface.
func startMode(t *testing.T, bbs bool, f *fakeSermo, screen func(d *ui.Deps) ui.Screen) (*ui.Deps, *harness) {
	t.Helper()
	if bbs {
		d := withZones(t, bbsDeps(t, f, true))
		return d, startBBS(t, screen(d))
	}
	d := withZones(t, testDeps(t, f, true))
	return d, start(t, screen(d))
}

// openedURLs records the URLs that ui.Open gives to the browser.
func openedURLs(t *testing.T) *[]string {
	t.Helper()
	var got []string
	old := ui.OpenURL
	ui.OpenURL = func(u string) error { got = append(got, u); return nil }
	t.Cleanup(func() { ui.OpenURL = old })
	return &got
}

// twoLinkServer serves conversation 5 with one message that has two links.
func twoLinkServer(t *testing.T) *fakeSermo {
	t.Helper()
	g := newFakeSermo(t)
	g.handle("GET /api/v1/conversations/5", 200, convJSON(5, "Viaje a Varadero"))
	g.handle("GET /api/v1/conversations/5/unread", 200, map[string]any{"conversationId": 5, "messageId": 77, "cursor": nil, "page": 1})
	g.handle("GET /api/v1/conversations/5/messages", 200, map[string]any{
		"items": []any{
			message(77, 5, member(12, "Ana"), `<p>Mira <a href="https://example.com/uno">uno</a> y <a href="https://example.com/dos">dos</a></p>`),
			message(78, 5, member(7, "albert"), "<p>Claro que sí.</p>"),
		},
		"nextCursor": nil, "total": 2,
	})
	g.handle("PUT /api/v1/conversations/5/read", 200, map[string]any{})
	g.handle("GET /api/v1/conversation-messages/77/quote", 200, map[string]any{
		"messageId": 77, "author": member(12, "Ana"), "bodySource": "Mira",
	})
	g.handle("POST /api/v1/conversations/5/messages", 201, message(79, 5, member(7, "albert"), "<p>hola</p>"))
	return g
}

func TestNotificationClickSelectsThenOpens(t *testing.T) {
	f := notificationServer(t)
	d := withZones(t, testDeps(t, f, true))
	h := start(t, NewNotifications(d))
	n := h.s.(*notifications)
	h.click(d, n.list.RowZone(1))
	if n.list.Cursor() != 1 {
		t.Fatalf("the first click must select row 1, cursor = %d", n.list.Cursor())
	}
	if _, ok := msgOf[ui.OpenProfileMsg](h.msgs); ok {
		t.Fatal("the first click must not open the row")
	}
	h.click(d, n.list.RowZone(1))
	if open, ok := msgOf[ui.OpenProfileMsg](h.msgs); !ok || open.UserID != 7 {
		t.Fatalf("the second click must open the profile, got %#v", h.msgs)
	}
}

func TestNotificationBBSClickSelectsThenOpens(t *testing.T) {
	f := notificationServer(t)
	d := withZones(t, bbsDeps(t, f, true))
	h := startBBS(t, NewNotifications(d))
	n := h.s.(*notifications)
	h.click(d, n.zid+"row.1")
	if n.pg.sel != 1 {
		t.Fatalf("the first click must select row 02, sel = %d", n.pg.sel)
	}
	if _, ok := msgOf[ui.OpenProfileMsg](h.msgs); ok {
		t.Fatal("the first click must not open the row")
	}
	h.click(d, n.zid+"row.1")
	if open, ok := msgOf[ui.OpenProfileMsg](h.msgs); !ok || open.UserID != 7 {
		t.Fatalf("the second click must open the profile, got %#v", h.msgs)
	}
}

func TestBBSTablesWithZonesFitTheWidth(t *testing.T) {
	f := notificationServer(t)
	d := withZones(t, bbsDeps(t, f, true))
	h := startBBS(t, NewNotifications(d))
	if !zonetest.HasMarkers(h.s.View(h.w, h.h)) {
		t.Fatal("the view must have zone markers")
	}
	h.wantFit()
	c := newFakeSermo(t)
	c.handle("GET /api/v1/conversations", 200, map[string]any{
		"items": []any{conversationSummary(5, "Viaje a Varadero", true)}, "nextCursor": nil, "total": 30,
	})
	h = startBBS(t, NewConversations(withZones(t, bbsDeps(t, c, true))))
	h.wantFit()
}

func TestConversationsBBSClickSelectsThenOpens(t *testing.T) {
	f := newFakeSermo(t)
	f.handle("GET /api/v1/conversations", 200, map[string]any{
		"items":      []any{conversationSummary(5, "Viaje a Varadero", true), conversationSummary(6, "Libros", false)},
		"nextCursor": nil, "total": 2,
	})
	d := withZones(t, bbsDeps(t, f, true))
	h := startBBS(t, NewConversations(d))
	c := h.s.(*conversations)
	h.click(d, c.zid+"row.1")
	if _, ok := msgOf[ui.PushMsg](h.msgs); ok || c.pg.sel != 1 {
		t.Fatalf("the first click must only select row 02, sel = %d", c.pg.sel)
	}
	h.click(d, c.zid+"row.1")
	push, ok := msgOf[ui.PushMsg](h.msgs)
	if !ok {
		t.Fatalf("the second click must push the conversation, got %#v", h.msgs)
	}
	if conv, isConv := push.Screen.(*conversation); !isConv || conv.id != 6 {
		t.Fatalf("pushed %#v, want the conversation 6", push.Screen)
	}
}

func TestConversationsPageCounterClickTurnsThePage(t *testing.T) {
	for _, bbs := range []bool{false, true} {
		f := newFakeSermo(t)
		f.handle("GET /api/v1/conversations", 200, map[string]any{
			"items": []any{conversationSummary(5, "Viaje a Varadero", true)}, "nextCursor": nil, "total": 100,
		})
		d, h := startMode(t, bbs, f, func(d *ui.Deps) ui.Screen { return NewConversations(d) })
		c := h.s.(*conversations)
		h.clickRight(d, c.zid+"page")
		if c.page != 2 {
			t.Fatalf("bbs %v: a click on the right half must show page 2, page = %d", bbs, c.page)
		}
		h.click(d, c.zid+"page")
		if c.page != 1 {
			t.Fatalf("bbs %v: a click on the left half must show page 1, page = %d", bbs, c.page)
		}
	}
}

func TestConversationClickLinkOpensIt(t *testing.T) {
	for _, bbs := range []bool{false, true} {
		got := openedURLs(t)
		f := twoLinkServer(t)
		d, h := startMode(t, bbs, f, func(d *ui.Deps) ui.Screen { return NewConversation(d, 5) })
		h.click(d, h.s.(*conversation).zid+"link.0.2")
		if len(*got) != 1 || (*got)[0] != "https://example.com/dos" {
			t.Fatalf("bbs %v: the click on [2] must open link 2, opened %q", bbs, *got)
		}
	}
}

func TestConversationClickFocusesMessage(t *testing.T) {
	f := twoLinkServer(t)
	d := withZones(t, testDeps(t, f, true))
	h := start(t, NewConversation(d, 5))
	c := h.s.(*conversation)
	h.click(d, c.zid+"msg.1")
	if c.cur != 1 {
		t.Fatalf("the click must focus message 2, cur = %d", c.cur)
	}
	h.wantText("mensaje 2 de 2, de albert")
}

func TestConversationBBSClickQuoteActsOnThatMessage(t *testing.T) {
	f := twoLinkServer(t)
	d := withZones(t, bbsDeps(t, f, true))
	h := startBBS(t, NewConversation(d, 5))
	c := h.s.(*conversation)
	c.cur = 1 // C without a number quotes message 78 now
	h.click(d, c.zid+"act.0.C")
	if n := len(f.find("GET", "/api/v1/conversation-messages/77/quote")); n != 1 {
		t.Fatalf("[C]itar of message 1 must quote message 77, got %d requests", n)
	}
	if c.led == nil {
		t.Fatal("the quote must open the line editor")
	}
}

func TestConversationBBSClickReplyOpensEditor(t *testing.T) {
	f := twoLinkServer(t)
	d := withZones(t, bbsDeps(t, f, true))
	h := startBBS(t, NewConversation(d, 5))
	c := h.s.(*conversation)
	h.click(d, c.zid+"act.0.R")
	if c.led == nil || len(c.led.lines) != 0 {
		t.Fatal("[R]esponder must open an empty line editor")
	}
}

func TestEditorHintClickSends(t *testing.T) {
	f := conversationServer(t)
	d := withZones(t, testDeps(t, f, true))
	h := start(t, NewConversation(d, 5))
	h.key("r")
	h.typeText("hola")
	c := h.s.(*conversation)
	h.click(d, c.ed.zid+"hint.0")
	posts := f.find("POST", "/api/v1/conversations/5/messages")
	if len(posts) != 1 || !jsonEq(t, posts[0].Body, `{"body":"hola"}`) {
		t.Fatalf("the click on Ctrl+S must send, requests = %#v", posts)
	}
}

func TestLineEditorHintsRunCommands(t *testing.T) {
	f := conversationServer(t)
	h := startBBS(t, NewConversation(bbsDeps(t, f, true), 5))
	h.command("R", "")
	h.typeText("hola")
	h.key("enter")
	h.typeText("abc")
	h.wantHints("Editor", "[/S] Enviar", "[/A] Abortar", "[/C] Citar", "[/V] Vista previa", "[/?] Ayuda")
	h.command("/?", "")
	if e := h.s.(*conversation).led; e.mode != viewHelp || string(e.in) != "abc" {
		t.Fatalf("/? must show the help and keep the typed line, mode %d in %q", e.mode, string(e.in))
	}
	h.key("enter")
	h.command("/S", "")
	posts := f.find("POST", "/api/v1/conversations/5/messages")
	if len(posts) != 1 || !jsonEq(t, posts[0].Body, `{"body":"hola"}`) {
		t.Fatalf("/S must send the lines, requests = %#v", posts)
	}
}

func TestLineEditorAbortHintClosesEditor(t *testing.T) {
	f := conversationServer(t)
	h := startBBS(t, NewConversation(bbsDeps(t, f, true), 5))
	h.command("R", "")
	h.command("/A", "")
	if h.s.CapturesInput() {
		t.Fatal("/A must close the line editor")
	}
}

func TestProfileClickTabSwitches(t *testing.T) {
	f := ownServer(t)
	d := withZones(t, testDeps(t, f, true))
	h := start(t, NewProfile(d, 0))
	pr := h.s.(*profile)
	h.click(d, pr.zid+"tab.1")
	if pr.tab != 1 {
		t.Fatalf("the click must open tab 2, tab = %d", pr.tab)
	}
	h.wantText("Elecciones")
	bm := pr.tabs[1].(*bookmarksTab)
	h.click(d, bm.list.RowZone(0))
	if open, ok := msgOf[ui.OpenThreadMsg](h.msgs); !ok || open != (ui.OpenThreadMsg{ThreadID: 10, PostID: 55}) {
		t.Fatalf("a click on the selected bookmark must open it, got %#v", h.msgs)
	}
}

func TestProfileBBSClickRowOpensPost(t *testing.T) {
	f := profileServer(t)
	d := withZones(t, bbsDeps(t, f, true))
	h := startBBS(t, NewProfile(d, 9))
	posts := h.s.(*profile).posts
	h.click(d, posts.zid+"row.0")
	if posts.open != 0 {
		t.Fatalf("a click on the selected row must open the post, open = %d", posts.open)
	}
}

func TestProfilePostClickLinkOpensIt(t *testing.T) {
	got := openedURLs(t)
	f := newFakeSermo(t)
	f.handle("GET /api/v1/users/9", 200, profileJSON(9, "Yuniel"))
	f.handle("GET /api/v1/users/9/profile-posts", 200, map[string]any{
		"items":      []any{profilePost(501, member(12, "Ana"), `<p><a href="https://example.com/a">a</a> <a href="https://example.com/b">b</a></p>`, 0)},
		"nextCursor": nil,
	})
	d := withZones(t, testDeps(t, f, true))
	h := start(t, NewProfile(d, 9))
	h.key("enter")
	h.click(d, h.s.(*profile).posts.zid+"link.2")
	if len(*got) != 1 || (*got)[0] != "https://example.com/b" {
		t.Fatalf("the click on [2] must open link 2, opened %q", *got)
	}
}

func TestLinkPickerClickOpensLink(t *testing.T) {
	got := openedURLs(t)
	f := twoLinkServer(t)
	d := withZones(t, testDeps(t, f, true))
	h := start(t, NewConversation(d, 5))
	h.key("o")
	c := h.s.(*conversation)
	h.click(d, c.picker.list.RowZone(1))
	h.click(d, c.picker.list.RowZone(1))
	if len(*got) != 1 || (*got)[0] != "https://example.com/dos" {
		t.Fatalf("two clicks on row 2 must open link 2, opened %q", *got)
	}
}

func TestSearchAndOnlineClickOpens(t *testing.T) {
	f := searchServer(t)
	d := withZones(t, testDeps(t, f, false))
	h := start(t, NewSearch(d))
	h.key("/")
	h.typeText("cuba")
	h.key("enter")
	h.click(d, h.s.(*search).list.RowZone(0))
	if open, ok := msgOf[ui.OpenThreadMsg](h.msgs); !ok || open.ThreadID != 34941 {
		t.Fatalf("a click on the selected result must open it, got %#v", h.msgs)
	}
	o := onlineServer(t)
	d = withZones(t, bbsDeps(t, o, false))
	h = startBBS(t, NewOnline(d))
	h.click(d, h.s.(*online).zid+"row.0")
	if open, ok := msgOf[ui.OpenProfileMsg](h.msgs); !ok || open.UserID != 12 {
		t.Fatalf("a click on the selected visitor must open the profile, got %#v", h.msgs)
	}
}

func TestClickWithoutZonesDoesNothing(t *testing.T) {
	f := notificationServer(t)
	h := start(t, NewNotifications(testDeps(t, f, true)))
	before := len(h.msgs)
	h.send(tea.MouseReleaseMsg{X: 3, Y: 3, Button: tea.MouseLeft})
	if len(h.msgs) != before {
		t.Fatalf("a click without zones must do nothing, got %#v", h.msgs[before:])
	}
}
