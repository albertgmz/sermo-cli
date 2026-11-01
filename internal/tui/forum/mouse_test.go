package forum

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone/v2"

	"github.com/albertgmz/sermo-cli/internal/api/gen"
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

// clickZone draws the top screen and clicks the zone id of that frame.
func (h *harness) clickZone(d *ui.Deps, id string) {
	h.t.Helper()
	h.send(zonetest.Click(h.t, d.Zones, h.top().View(h.w, h.h), id))
}

// openedURLs replaces the browser and returns the URLs that ui.Open gets.
func openedURLs(t *testing.T) *[]string {
	t.Helper()
	var got []string
	old := ui.OpenURL
	ui.OpenURL = func(u string) error { got = append(got, u); return nil }
	t.Cleanup(func() { ui.OpenURL = old })
	return &got
}

func nodeIndex(items []gen.Node, id int) int {
	for i, n := range items {
		if n.Id == id {
			return i
		}
	}
	return -1
}

func TestClickNodeRowSelectsThenOpens(t *testing.T) {
	f := newFakeSermo(t)
	d := withZones(t, testDeps(t, f, false))
	h := newHarness(t, NewNodes(d))
	s := h.top().(*nodes)
	i := nodeIndex(s.items, 68)
	h.clickZone(d, s.list.RowZone(i))
	if len(h.intent) != 0 || s.list.Cursor() != i {
		t.Fatalf("the first click must select: cursor %d, intent %#v", s.list.Cursor(), h.intent)
	}
	h.clickZone(d, s.list.RowZone(i))
	if len(h.intent) != 1 || h.intent[0] != (ui.OpenNodeMsg{NodeID: 68}) {
		t.Fatalf("the second click gave %#v, want OpenNodeMsg 68", h.intent)
	}
}

func TestClickBBSIndexRowSelectsThenOpens(t *testing.T) {
	f := indexFake(t)
	d := withZones(t, indexDeps(t, f, false))
	h := indexHarness(t, NewNodes(d), 80, 19)
	s := h.top().(*nodes)
	first := nodeIndex(s.items, 45)
	h.clickZone(d, s.list.RowZone(first))
	if len(h.intent) != 0 {
		t.Fatalf("a click on the first forum must only select it, intent %#v", h.intent)
	}
	i := nodeIndex(s.items, 64)
	h.clickZone(d, s.list.RowZone(i))
	if len(h.intent) != 0 || s.list.Cursor() != i {
		t.Fatalf("the first click must select: cursor %d, intent %#v", s.list.Cursor(), h.intent)
	}
	if !strings.Contains(h.top().View(h.w, h.h), d.Styles.Key.Render("03")) {
		t.Fatal("the selected row must get the Key style")
	}
	h.clickZone(d, s.list.RowZone(i))
	if len(h.intent) != 1 || h.intent[0] != (ui.OpenNodeMsg{NodeID: 64}) {
		t.Fatalf("the second click gave %#v, want OpenNodeMsg 64", h.intent)
	}
}

func TestClickThreadRowSelectsThenOpens(t *testing.T) {
	for _, bbs := range []bool{false, true} {
		f := idxThreadFake(t)
		d := testDeps(t, f, false)
		if bbs {
			d = indexDeps(t, f, false)
		}
		withZones(t, d)
		h := indexHarness(t, NewNode(d, 44), 80, 19)
		s := h.top().(*threadList)
		h.clickZone(d, s.list.RowZone(2))
		if len(h.intent) != 0 || s.list.Cursor() != 2 {
			t.Fatalf("bbs %v: the first click must select, cursor %d", bbs, s.list.Cursor())
		}
		h.clickZone(d, s.list.RowZone(2))
		want := ui.OpenThreadMsg{ThreadID: s.rows[2].Id}
		if len(h.intent) != 1 || h.intent[0] != want {
			t.Fatalf("bbs %v: the second click gave %#v, want %#v", bbs, h.intent, want)
		}
	}
}

func TestClickThreadListPageCounterTurnsPage(t *testing.T) {
	f := newFakeSermo(t)
	d := withZones(t, testDeps(t, f, false))
	h := newHarness(t, NewNode(d, 44))
	s := h.top().(*threadList)
	if s.pages < 2 {
		t.Fatalf("the fixture must have pages, got %d", s.pages)
	}
	z := zonetest.Zone(t, d.Zones, s.View(h.w, h.h), s.zid+"page")
	h.send(zonetest.At(z.EndX, z.StartY))
	reqs := f.requests("GET", "/api/v1/nodes/44/threads")
	if q := reqs[len(reqs)-1].Query; !strings.Contains(q, "page=2") {
		t.Fatalf("a click on the right half must load page 2, query %q", q)
	}
}

const twoLinkPosts = `{"items":[{"id":1,"threadId":34898,"position":0,"author":{"kind":"member","id":68725,"username":"danipow"},"state":"visible","createdAt":"2026-07-13T05:14:21.000Z","editedAt":null,"bodyHtml":"<p>Mira <a href=\"https://example.com/a\">esto</a> y <a href=\"https://example.com/b\">eso</a></p>","attachmentCount":0,"attachments":[],"reactions":{"counts":{},"total":0,"mine":null}}],"nextCursor":null}`

func TestClickLinkLabelOpensLink(t *testing.T) {
	for _, bbs := range []bool{false, true} {
		f := newFakeSermo(t)
		f.set("GET /api/v1/threads/34898/posts", 200, twoLinkPosts)
		d := testDeps(t, f, false)
		if bbs {
			d = bbsTestDeps(t, f, false)
		}
		withZones(t, d)
		got := openedURLs(t)
		h := newHarness(t, NewThread(d, 34898, 0))
		r := h.top().(*reader)
		h.clickZone(d, r.linkZone(0)+"2")
		if len(*got) != 1 || (*got)[0] != "https://example.com/b" {
			t.Fatalf("bbs %v: opened %q, want link 2", bbs, *got)
		}
	}
}

func TestClickPostFocusesIt(t *testing.T) {
	f := bbsFake(t)
	d := withZones(t, testDeps(t, f, true))
	h := newHarness(t, NewThread(d, 34898, 0))
	r := h.top().(*reader)
	if r.focus != 0 {
		t.Fatalf("focus = %d at the start", r.focus)
	}
	h.clickZone(d, r.lineZone(r.starts[2]-r.vp.YOffset()+1))
	if r.focus != 2 {
		t.Fatalf("focus = %d after a click on post 3, want 2", r.focus)
	}
}

func TestClickReaderPageCounterTurnsPage(t *testing.T) {
	f := newFakeSermo(t)
	f.set("GET /api/v1/threads/34898", 200, strings.Replace(memberThread(t), `"replyCount":32`, `"replyCount":45`, 1))
	d := withZones(t, testDeps(t, f, true))
	h := newHarness(t, NewThread(d, 34898, 0))
	r := h.top().(*reader)
	page := r.page
	z := zonetest.Zone(t, d.Zones, r.View(h.w, h.h), r.zid+"page")
	h.send(zonetest.At(z.EndX, z.StartY))
	if r.page != page+1 {
		t.Fatalf("page = %d after a click on the right half, want %d", r.page, page+1)
	}
}

func TestClickBBSFooterRepliesAndQuotesThatPost(t *testing.T) {
	f := bbsFake(t)
	f.set("GET /api/v1/posts/478003", 200, fixture(t, "post.json"))
	d := withZones(t, bbsTestDeps(t, f, true))
	h := &harness{t: t, w: 80, h: 60}
	h.push(NewThread(d, 34898, 0))
	r := h.top().(*reader)
	h.clickZone(d, r.actionZone("C", 1))
	c, ok := h.top().(*composer)
	if !ok || c.ed == nil {
		t.Fatalf("[C]itar must open the line editor, top = %T", h.top())
	}
	want := `[QUOTE="Anierrr, post: 478003, member: 1825"][B][I]Las leyendas nunca mueren.[/I][/B][/QUOTE]`
	if c.ed.lines[0] != want {
		t.Fatalf("lines = %q, want first %q", c.ed.lines, want)
	}
	h.stack = h.stack[:1]
	h.clickZone(d, r.actionZone("R", 0))
	c, ok = h.top().(*composer)
	if !ok || c.ed == nil || len(c.ed.lines) != 0 || r.focus != 0 {
		t.Fatalf("[R]esponder must open an empty line editor for post 1, top = %T", h.top())
	}
}

func TestClickBBSChoiceRowSelectsThenChooses(t *testing.T) {
	f := bbsFake(t)
	f.set("PUT /api/v1/reactions/post/478003", 200, `{"counts":{"3":1},"total":1,"mine":3}`)
	d := withZones(t, bbsTestDeps(t, f, true))
	h := bbsHarness(t, NewThread(d, 34898, 0))
	h.command("+ 2")
	r := h.top().(*reader)
	h.clickZone(d, r.picker.list.RowZone(1))
	if r.picker == nil || r.picker.list.Cursor() != 1 {
		t.Fatal("the first click must select the row")
	}
	h.clickZone(d, r.picker.list.RowZone(1))
	puts := f.requests("PUT", "/api/v1/reactions/post/478003")
	if len(puts) != 1 || decode(t, puts[0].Body)["reactionTypeId"] != float64(3) {
		t.Fatalf("react requests = %+v", puts)
	}
}

func TestClickComposerHintSends(t *testing.T) {
	f := newFakeSermo(t)
	f.set("POST /api/v1/threads/34898/posts", 201, newPostJSON)
	d := withZones(t, testDeps(t, f, true))
	h := newHarness(t, NewComposer(d, ui.ComposeMsg{ThreadID: 34898}))
	h.typeText("Hola")
	c := h.top().(*composer)
	h.clickZone(d, c.zid+"hint.0")
	if posts := f.requests("POST", "/api/v1/threads/34898/posts"); len(posts) != 1 {
		t.Fatalf("a click on the send hint must send, requests %+v", posts)
	}
}

func TestBBSEditorHintCommandsRun(t *testing.T) {
	f := newFakeSermo(t)
	f.set("POST /api/v1/threads/34898/posts", 201, newPostJSON)
	h, c := bbsEditor(t, f)
	h.enterLine("Hola")
	h.typeText("medio")
	runHint(t, h, c, "/?")
	if c.ed.panel != panelHelp || c.ed.cur != "medio" {
		t.Fatalf("/? must show the help and keep the typed line, panel %d cur %q", c.ed.panel, c.ed.cur)
	}
	runHint(t, h, c, "/S")
	posts := f.requests("POST", "/api/v1/threads/34898/posts")
	if len(posts) != 1 || decode(t, posts[0].Body)["body"] != "Hola" {
		t.Fatalf("/S must send the lines, requests %+v", posts)
	}
}

// runHint runs the hint command name like a click on the hint line does.
func runHint(t *testing.T, h *harness, c *composer, name string) {
	t.Helper()
	for _, cmd := range c.Commands() {
		if cmd.Name == name {
			if cmd.Run == nil {
				t.Fatalf("the hint %s has no Run", name)
			}
			h.run(cmd.Run(""))
			return
		}
	}
	t.Fatalf("no hint %s", name)
}

func TestBBSIndexWidthWithZones(t *testing.T) {
	f := indexFake(t)
	d := withZones(t, indexDeps(t, f, true))
	h := indexHarness(t, NewNodes(d), 80, 19)
	for i, l := range strings.Split(h.top().View(80, 19), "\n") {
		if w := ansi.StringWidth(l); w != 80 {
			t.Fatalf("line %d is %d cells wide with zones: %q", i, w, ansi.Strip(l))
		}
	}
}

func TestClickPollAnswerSelectsThenMarks(t *testing.T) {
	f := bbsFake(t)
	poll := `{"id":3,"threadId":34898,"kind":"poll","maxChoices":1,"allowChange":false,"allowCustom":false,"anonymous":false,"answerMaxLength":100,"closed":false,"closesAt":null,"createdAt":"2026-07-13T05:14:21.000Z","updatedAt":"2026-07-13T05:14:21.000Z","results":"always","resultsShown":true,"voterCount":3,"mine":{"voted":false,"answered":0,"completed":false,"correct":0},"permissions":{"canVote":true,"canAddAnswer":false,"canClose":false,"canDelete":false,"canEdit":false,"canViewVoters":false},"questions":[{"id":30,"text":"Cafe o te","answered":false,"explanation":null,"image":null,"answers":[{"id":301,"text":"Cafe","chosen":false,"custom":false,"correct":null,"image":null,"votes":2},{"id":302,"text":"Te","chosen":false,"custom":false,"correct":null,"image":null,"votes":1}]}]}`
	f.set("GET /api/v1/threads/34898/poll", 200, `{"canCreate":false,"poll":`+poll+`}`)
	for _, bbs := range []bool{false, true} {
		d := testDeps(t, f, true)
		if bbs {
			d = bbsTestDeps(t, f, true)
		}
		withZones(t, d)
		h := newHarness(t, newPoll(d, 34898))
		s := h.top().(*pollScreen)
		h.clickZone(d, s.list.RowZone(1))
		if s.list.Cursor() != 1 || s.chosen[302] {
			t.Fatalf("bbs %v: the first click must only select", bbs)
		}
		h.clickZone(d, s.list.RowZone(1))
		if !s.chosen[302] {
			t.Fatalf("bbs %v: a click on the selected answer must mark it", bbs)
		}
	}
}

func TestClickBBSIndexPageCounterTurnsPage(t *testing.T) {
	f := indexFake(t)
	d := withZones(t, indexDeps(t, f, true))
	h := indexHarness(t, NewNodes(d), 80, 19)
	s := h.top().(*nodes)
	z := zonetest.Zone(t, d.Zones, s.View(h.w, h.h), s.zid+"page")
	h.send(zonetest.At(z.EndX, z.StartY))
	h.wantView("Página 2 / 2")
	z = zonetest.Zone(t, d.Zones, s.View(h.w, h.h), s.zid+"page")
	h.send(zonetest.At(z.StartX, z.StartY))
	h.wantView("Página 1 / 2")
}

func TestClickBBSThreadListPageCounterTurnsPage(t *testing.T) {
	f := idxThreadFake(t)
	d := withZones(t, indexDeps(t, f, false))
	h := indexHarness(t, NewNode(d, 44), 80, 19)
	s := h.top().(*threadList)
	z := zonetest.Zone(t, d.Zones, s.View(h.w, h.h), s.zid+"page")
	h.send(zonetest.At(z.EndX, z.StartY))
	reqs := f.requests("GET", "/api/v1/nodes/44/threads")
	if q := reqs[len(reqs)-1].Query; !strings.Contains(q, "page=2") {
		t.Fatalf("a click on the right half must load page 2, query %q", q)
	}
}

func TestEmptyLinesBelowLastPostHaveNoZone(t *testing.T) {
	f := newFakeSermo(t)
	f.set("GET /api/v1/threads/34898/posts", 200, twoLinkPosts)
	d := withZones(t, testDeps(t, f, false))
	h := newHarness(t, NewThread(d, 34898, 0))
	r := h.top().(*reader)
	last := r.vp.TotalLineCount() - r.vp.YOffset()
	lines := strings.Split(r.markBody(r.vp.View()), "\n")
	if last >= len(lines) {
		t.Fatalf("the post must leave empty lines, %d of %d", last, len(lines))
	}
	if !zonetest.HasMarkers(lines[last-1]) || zonetest.HasMarkers(lines[last]) {
		t.Fatalf("only the lines of the posts must have zones: %q %q", lines[last-1], lines[last])
	}
}
