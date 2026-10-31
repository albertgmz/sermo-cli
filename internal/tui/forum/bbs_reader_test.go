package forum

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/albertgmz/sermo-cli/internal/config"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// bbsPostsJSON is a page with a quote, a code block, a guest author and a
// second post of the same member.
const bbsPostsJSON = `{"items":[` +
	`{"id":477999,"threadId":34898,"position":0,"author":{"kind":"member","id":68725,"username":"danipow"},"state":"visible","createdAt":"2026-10-15T14:02:00Z","editedAt":null,"bodyHtml":"<p>Hola a todos.</p>","attachmentCount":0,"attachments":[],"reactions":{"counts":{"3":2,"1":12},"total":14,"mine":null},"canEdit":false,"canReact":true},` +
	`{"id":478003,"threadId":34898,"position":1,"author":{"kind":"member","id":1825,"username":"Anierrr"},"state":"visible","createdAt":"2026-10-15T14:37:00Z","editedAt":"2026-10-15T14:40:00Z","bodyHtml":"<blockquote class=\"bb-quote\"><cite class=\"bb-quote-author\">danipow</cite>Hola a todos.</blockquote><p>Mira esto:</p><pre><code class=\"language-go\">x := 1</code></pre>","attachmentCount":0,"attachments":[],"reactions":{"counts":{},"total":0,"mine":null},"canEdit":true,"canReact":true},` +
	`{"id":478010,"threadId":34898,"position":2,"author":{"kind":"guest","name":"Pepe"},"state":"visible","createdAt":"2026-10-15T15:00:00Z","editedAt":null,"bodyHtml":"<p>Soy invitado.</p>","attachmentCount":0,"attachments":[],"reactions":{"counts":{},"total":0,"mine":null}},` +
	`{"id":478020,"threadId":34898,"position":3,"author":{"kind":"member","id":68725,"username":"danipow"},"state":"visible","createdAt":"2026-10-15T16:00:00Z","editedAt":null,"bodyHtml":"<p>Otra vez yo.</p>","attachmentCount":0,"attachments":[],"reactions":{"counts":{},"total":0,"mine":null}}` +
	`],"nextCursor":null}`

const daniProfile = `{"id":68725,"username":"danipow","groupTitle":"Registered","displayGroup":{"id":3,"title":"Veterans",` +
	`"userTitle":"Veterano","badge":""},"createdAt":"2026-03-01T10:00:00Z","postCount":812,"reactionScore":214,` +
	`"customTitle":"Hacker","signatureHtml":"<p>firma de dani</p>"}`

const anierrrProfile = `{"id":1825,"username":"Anierrr","groupTitle":"Miembro","displayGroup":null,` +
	`"createdAt":"2025-01-20T10:00:00Z","postCount":1,"reactionScore":0,"customTitle":null,"signatureHtml":null}`

func bbsTestDeps(t *testing.T, f *fakeSermo, member bool) *ui.Deps {
	t.Helper()
	d := testDeps(t, f, member)
	d.Config.Interface = config.InterfaceBBS
	return d
}

// bbsHarness hosts s in the content area of an 80x24 BBS window.
func bbsHarness(t *testing.T, s ui.Screen) *harness {
	t.Helper()
	h := &harness{t: t, w: 80, h: 19}
	h.push(s)
	return h
}

// command runs text like the BBS prompt does: digits run the command "#",
// other text runs the command of its first word.
func (h *harness) command(text string) {
	h.t.Helper()
	name, arg := "#", text
	if text == "" || strings.Trim(text, "0123456789") != "" {
		name, arg, _ = strings.Cut(text, " ")
		name = strings.ToUpper(name)
	}
	c, ok := h.top().(ui.Commander)
	if !ok {
		h.t.Fatalf("%T has no commands", h.top())
	}
	for _, cmd := range c.Commands() {
		if cmd.Run != nil && strings.EqualFold(cmd.Name, name) {
			h.run(cmd.Run(strings.TrimSpace(arg)))
			return
		}
	}
	h.t.Fatalf("%T has no command %q", h.top(), name)
}

// hints returns the labels of the commands of the top screen.
func (h *harness) hints() string {
	var out []string
	for _, c := range h.top().(ui.Commander).Commands() {
		out = append(out, c.Label)
	}
	return strings.Join(out, "  ")
}

func bbsFake(t *testing.T) *fakeSermo {
	f := newFakeSermo(t)
	f.set("GET /api/v1/threads/34898", 200, memberThread(t))
	f.set("GET /api/v1/threads/34898/posts", 200, bbsPostsJSON)
	f.set("GET /api/v1/threads/34898/unread", 200, `{"threadId":34898,"postId":477999,"position":0}`)
	f.set("GET /api/v1/users/68725", 200, daniProfile)
	f.set("GET /api/v1/users/1825", 200, anierrrProfile)
	return f
}

// stamp is the BBS date of an UTC time in the local zone.
func stamp(utc string) string {
	tm, _ := time.Parse(time.RFC3339, utc)
	tm = tm.Local()
	months := strings.Split(i18n.T("bbs.months"), ",")
	return fmt.Sprintf("%02d-%s-%d  %s", tm.Day(), months[tm.Month()-1], tm.Year(), tm.Format("15:04"))
}

func lineIndex(lines []string, part string) int {
	for i, l := range lines {
		if strings.Contains(l, part) {
			return i
		}
	}
	return -1
}

func TestBBSReaderHeaderAndPrompt(t *testing.T) {
	f := bbsFake(t)
	h := bbsHarness(t, NewThread(bbsTestDeps(t, f, true), 34898, 0))
	lines := strings.Split(h.view(), "\n")
	if len(lines) != 19 {
		t.Fatalf("view has %d lines, want 19:\n%s", len(lines), h.view())
	}
	for i, l := range lines {
		if ansi.StringWidth(l) != 80 {
			t.Fatalf("line %d is %d cells wide: %q", i, ansi.StringWidth(l), l)
		}
	}
	wantAt(t, lines, 0, "| Foro Libre > #34898 ", " |")
	wantAt(t, lines, 1, "| Ay bbs pq dejaron morir esto? ", " |")
	wantAt(t, lines, 2, "| 32 respuestas . 1.124 vistas . Etiquetas: reencuentro ", " |")
	wantAt(t, lines, 3, ruleEq, ruleEq)
	wantAt(t, lines, 4, "| #1   danipow ", " |")
	r := h.top().(*reader)
	if r.Prompt() != "Leyendo" {
		t.Fatalf("prompt = %q", r.Prompt())
	}
	want := "Mensajes 1-4 de 33   [ENTER] Sig.  [P]revio  [##] Ir a  [R]esponder  [V]olver"
	if !strings.HasPrefix(h.hints(), want) {
		t.Fatalf("hints = %q, want prefix %q", h.hints(), want)
	}
}

// memberContent opens the BBS reader as a member and returns all lines of
// the posts. Each line must be 80 cells wide.
func memberContent(t *testing.T) []string {
	t.Helper()
	f := bbsFake(t)
	h := bbsHarness(t, NewThread(bbsTestDeps(t, f, true), 34898, 0))
	content := ansi.Strip(h.top().(*reader).vp.GetContent())
	lines := strings.Split(content, "\n")
	for i, l := range lines {
		if ansi.StringWidth(l) != 80 {
			t.Fatalf("line %d is %d cells wide: %q", i, ansi.StringWidth(l), l)
		}
	}
	return lines
}

// wantAt fails when line i of lines does not start with prefix and end
// with suffix.
func wantAt(t *testing.T, lines []string, i int, prefix, suffix string) {
	t.Helper()
	if i < 0 || i >= len(lines) || !strings.HasPrefix(lines[i], prefix) || !strings.HasSuffix(lines[i], suffix) {
		t.Fatalf("line %d: want %q ... %q:\n%s", i, prefix, suffix, strings.Join(lines, "\n"))
	}
}

var (
	ruleEq   = "+" + strings.Repeat("=", 78) + "+"
	ruleDash = "+" + strings.Repeat("-", 78) + "+"
)

func TestBBSReaderPostRows(t *testing.T) {
	lines := memberContent(t)
	head := lineIndex(lines, "| #1   danipow  <<VETERANO>>")
	if head != 0 {
		t.Fatalf("post header at %d:\n%s", head, strings.Join(lines, "\n"))
	}
	wantAt(t, lines, lineIndex(lines, "| #2   Anierrr")-1, ruleEq, ruleEq)
	wantAt(t, lines, head, "| #1   danipow  <<VETERANO>> ", stamp("2026-10-15T14:02:00Z")+" |")
	wantAt(t, lines, head+1, "|      Hacker . 812 posts . Desde Mar-2026 . Rep +214 ", " |")
	wantAt(t, lines, head+2, ruleDash, ruleDash)
	wantAt(t, lines, head+4, "|  Hola a todos. ", " |")
	sig := lineIndex(lines, "|  -- ")
	wantAt(t, lines, sig+1, "|  firma de dani ", " |")
	foot := lineIndex(lines, "[Like 12] [Me Encanta 2]")
	wantAt(t, lines, foot, "| [Like 12] [Me Encanta 2] ", " [C]itar  [R]esponder |")
	wantAt(t, lines, lineIndex(lines, "| #2   Anierrr")+1, "|      Miembro . 1 post . Desde Ene-2025 . Rep +0 ", " |")
}

func TestBBSReaderPostBoxes(t *testing.T) {
	lines := memberContent(t)
	quote := lineIndex(lines, ".-- danipow escribió: ")
	wantAt(t, lines, quote, "|  .-- danipow escribió: ", "-.  |")
	wantAt(t, lines, quote+1, "|  | Hola a todos.", " |  |")
	wantAt(t, lines, lineIndex(lines, "+-[ go ]")+1, "|  | x := 1 ", " |  |")
	wantAt(t, lines, lineIndex(lines, "** EDITADO **"), "| ", " [C]itar  [R]esponder  [E]ditar |")
	wantAt(t, lines, lineIndex(lines, "| #3   Pepe ")+1, ruleDash, ruleDash)
	wantAt(t, lines, len(lines)-1, ruleEq, ruleEq)
}

func TestBBSReaderLoadsEachCardOnce(t *testing.T) {
	f := bbsFake(t)
	bbsHarness(t, NewThread(bbsTestDeps(t, f, false), 34898, 0))
	if n := len(f.requests("GET", "/api/v1/users/68725")); n != 1 {
		t.Fatalf("card requests for danipow = %d, want 1", n)
	}
	if n := len(f.requests("GET", "/api/v1/users/1825")); n != 1 {
		t.Fatalf("card requests for Anierrr = %d, want 1", n)
	}
	for _, r := range f.reqs {
		if strings.HasPrefix(r.Path, "/api/v1/users/") && r.Path != "/api/v1/users/68725" && r.Path != "/api/v1/users/1825" {
			t.Fatalf("unexpected card request %s", r.Path)
		}
	}
}

func TestBBSReaderWithoutCardsShowsPosts(t *testing.T) {
	f := bbsFake(t)
	f.set("GET /api/v1/users/68725", 500, `{"error":{"code":"internal","message":"x"}}`)
	h := bbsHarness(t, NewThread(bbsTestDeps(t, f, false), 34898, 0))
	content := ansi.Strip(h.top().(*reader).vp.GetContent())
	lines := strings.Split(content, "\n")
	head := lineIndex(lines, "| #1   danipow ")
	if head < 0 || lines[head+1] != "+"+strings.Repeat("-", 78)+"+" {
		t.Fatalf("a post without a card has no card row:\n%s", content)
	}
	if len(h.toasts) != 0 {
		t.Fatalf("a missing card must not show a toast: %+v", h.toasts)
	}
}

func TestModernReaderLoadsNoCards(t *testing.T) {
	f := bbsFake(t)
	h := newHarness(t, NewThread(testDeps(t, f, false), 34898, 0))
	for _, r := range f.reqs {
		if strings.HasPrefix(r.Path, "/api/v1/users/") {
			t.Fatalf("the modern reader asked for %s", r.Path)
		}
	}
	if _, ok := any(h.top()).(ui.Commander); !ok {
		t.Fatal("the reader must give BBS commands")
	}
	h.wantView("─")
}

func TestBBSReaderNumberJumpsToPost(t *testing.T) {
	f := bbsFake(t)
	f.set("GET /api/v1/threads/34898", 200, strings.Replace(memberThread(t), `"replyCount":32`, `"replyCount":45`, 1))
	h := bbsHarness(t, NewThread(bbsTestDeps(t, f, false), 34898, 0))
	r := h.top().(*reader)
	before := len(f.requests("GET", "/api/v1/threads/34898/posts"))
	h.command("2")
	if r.focusedPost().Position != 1 || len(f.requests("GET", "/api/v1/threads/34898/posts")) != before {
		t.Fatalf("2 must focus post #2 on the page, focus = %d", r.focusedPost().Position)
	}
	if r.vp.YOffset() != r.starts[1] {
		t.Fatalf("offset = %d, want %d", r.vp.YOffset(), r.starts[1])
	}
	f.set("GET /api/v1/threads/34898/posts", 200, strings.NewReplacer(`"position":2,`, `"position":24,`, `"position":3,`, `"position":25,`).Replace(bbsPostsJSON))
	h.command("25")
	reqs := f.requests("GET", "/api/v1/threads/34898/posts")
	if q := reqs[len(reqs)-1].Query; !strings.Contains(q, "page=2") {
		t.Fatalf("query = %q, want page=2", q)
	}
	if r.page != 2 || r.focusedPost().Position != 24 {
		t.Fatalf("page %d focus %d, want page 2 at #25", r.page, r.focusedPost().Position)
	}
	h.command("99")
	if h.lastToast() != i18n.Tf("bbs.reader.noPost", map[string]any{"N": 99}) {
		t.Fatalf("toast = %q", h.lastToast())
	}
}

func TestBBSReaderEnterScrollsThenTurnsPage(t *testing.T) {
	f := bbsFake(t)
	f.set("GET /api/v1/threads/34898", 200, strings.Replace(memberThread(t), `"replyCount":32`, `"replyCount":45`, 1))
	h := bbsHarness(t, NewThread(bbsTestDeps(t, f, false), 34898, 0))
	r := h.top().(*reader)
	h.command("")
	if r.vp.YOffset() == 0 || r.page != 1 {
		t.Fatalf("ENTER must scroll first, offset %d page %d", r.vp.YOffset(), r.page)
	}
	for i := 0; i < 20 && r.page == 1; i++ {
		h.command("")
	}
	if r.page != 2 {
		t.Fatalf("ENTER at the end must load the next page, page %d", r.page)
	}
	h.command("P")
	if r.page != 1 {
		t.Fatalf("P must load the previous page, page %d", r.page)
	}
}

func TestBBSReaderQuoteByNumberOpensLineEditor(t *testing.T) {
	f := bbsFake(t)
	f.set("GET /api/v1/posts/478003", 200, fixture(t, "post.json"))
	h := bbsHarness(t, NewThread(bbsTestDeps(t, f, true), 34898, 0))
	h.command("C 2")
	c, ok := h.top().(*composer)
	if !ok || c.ed == nil {
		t.Fatalf("C 2 must open the line editor, top = %T", h.top())
	}
	want := `[QUOTE="Anierrr, post: 478003, member: 1825"][B][I]Las leyendas nunca mueren.[/I][/B][/QUOTE]`
	if c.ed.lines[0] != want {
		t.Fatalf("lines = %q, want first %q", c.ed.lines, want)
	}
}

func TestBBSReaderReplyOpensLineEditor(t *testing.T) {
	f := bbsFake(t)
	h := bbsHarness(t, NewThread(bbsTestDeps(t, f, true), 34898, 0))
	h.command("R")
	c, ok := h.top().(*composer)
	if !ok || c.ed == nil || !c.CapturesInput() {
		t.Fatalf("R must open the line editor, top = %T", h.top())
	}
	h.wantView(`RESPONDER  >  #34898  "Ay bbs pq dejaron morir esto?"`, "/S enviar  /A abortar  /C citar  /V vista previa  /?")
}

func TestBBSReaderGuestReplyShowsLoginToast(t *testing.T) {
	f := bbsFake(t)
	h := bbsHarness(t, NewThread(bbsTestDeps(t, f, false), 34898, 0))
	for _, c := range []string{"R", "C", "C 2", "+", "*", "W"} {
		h.toasts = nil
		h.command(c)
		if h.lastToast() != i18n.T("forum.toast.loginNeeded") {
			t.Fatalf("%s: toast = %q", c, h.lastToast())
		}
	}
	if len(h.stack) != 1 {
		t.Fatalf("a guest must stay on the reader, stack = %d", len(h.stack))
	}
}

func TestBBSReaderReactByNumber(t *testing.T) {
	f := bbsFake(t)
	f.set("PUT /api/v1/reactions/post/478003", 200, `{"counts":{"3":1},"total":1,"mine":3}`)
	h := bbsHarness(t, NewThread(bbsTestDeps(t, f, true), 34898, 0))
	h.command("+ 2")
	h.wantView("01", "Like", "02", "Me Encanta")
	if h.top().CapturesInput() {
		t.Fatal("the BBS choice must leave the keys to the prompt")
	}
	if !strings.HasPrefix(h.hints(), "[##] Elegir") {
		t.Fatalf("hints = %q", h.hints())
	}
	h.command("2")
	puts := f.requests("PUT", "/api/v1/reactions/post/478003")
	if len(puts) != 1 || decode(t, puts[0].Body)["reactionTypeId"] != float64(3) {
		t.Fatalf("react requests = %+v", puts)
	}
	content := ansi.Strip(h.top().(*reader).vp.GetContent())
	if !strings.Contains(content, "[Me Encanta 1*]") {
		t.Fatalf("own reaction not shown:\n%s", content)
	}
}

func TestBBSReaderOpensLinkByNumber(t *testing.T) {
	f := bbsFake(t)
	f.set("GET /api/v1/threads/34898/posts", 200, `{"items":[{"id":1,"threadId":34898,"position":0,"author":{"kind":"member","id":68725,"username":"danipow"},"state":"visible","createdAt":"2026-07-13T05:14:21.000Z","editedAt":null,"bodyHtml":"<p>Mira <a href=\"https://example.com/a\">esto</a> y <a href=\"https://example.com/b\">eso</a></p>","attachmentCount":0,"attachments":[],"reactions":{"counts":{},"total":0,"mine":null}}],"nextCursor":null}`)
	h := bbsHarness(t, NewThread(bbsTestDeps(t, f, false), 34898, 0))
	h.command("O")
	h.wantView("01", "https://example.com/a", "02", "https://example.com/b")
	h.command("V")
	if h.top().(*reader).picker != nil {
		t.Fatal("V must close the choice")
	}
	h.command("O 3")
	if h.lastToast() != i18n.T("bbs.reader.noLink") {
		t.Fatalf("toast = %q", h.lastToast())
	}
}

func TestBBSPollVotesByNumber(t *testing.T) {
	f := bbsFake(t)
	f.set("GET /api/v1/threads/34898", 200, strings.Replace(memberThread(t), `"exists":false`, `"exists":true`, 1))
	poll := func(chosen bool, votes int) string {
		return fmt.Sprintf(`{"id":3,"threadId":34898,"kind":"poll","maxChoices":1,"allowChange":false,"allowCustom":false,"anonymous":false,"answerMaxLength":100,"closed":false,"closesAt":null,"createdAt":"2026-07-13T05:14:21.000Z","updatedAt":"2026-07-13T05:14:21.000Z","results":"always","resultsShown":true,"voterCount":%d,"mine":{"voted":%t,"answered":0,"completed":false,"correct":0},"permissions":{"canVote":true,"canAddAnswer":false,"canClose":false,"canDelete":false,"canEdit":false,"canViewVoters":false},"questions":[{"id":30,"text":"¿Café o té?","answered":false,"explanation":null,"image":null,"answers":[{"id":301,"text":"Café","chosen":%t,"custom":false,"correct":null,"image":null,"votes":%d},{"id":302,"text":"Té","chosen":false,"custom":false,"correct":null,"image":null,"votes":1}]}]}`, votes, chosen, chosen, votes)
	}
	f.set("GET /api/v1/threads/34898/poll", 200, `{"canCreate":false,"poll":`+poll(false, 3)+`}`)
	f.set("POST /api/v1/polls/3/vote", 200, poll(true, 4))
	h := bbsHarness(t, NewThread(bbsTestDeps(t, f, true), 34898, 0))
	h.command("T")
	if _, ok := h.top().(*pollScreen); !ok {
		t.Fatalf("T must open the poll, top = %T", h.top())
	}
	h.wantView("¿Café o té?", "01", "Café", "###", "75%", "02", "Té", "25%")
	if strings.ContainsAny(h.view(), "█░▶") {
		t.Fatalf("the BBS poll must be ASCII:\n%s", h.view())
	}
	h.command("1")
	h.command("")
	votes := f.requests("POST", "/api/v1/polls/3/vote")
	if len(votes) != 1 || fmt.Sprint(decode(t, votes[0].Body)["answerIds"]) != "[301]" {
		t.Fatalf("vote requests = %+v", votes)
	}
	h.wantView("80%", "[x]")
}
