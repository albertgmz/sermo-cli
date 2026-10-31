package forum

import (
	"fmt"
	"strings"
	"testing"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

func TestReaderRendersPostsAndReactions(t *testing.T) {
	f := newFakeSermo(t)
	h := newHarness(t, NewThread(testDeps(t, f, false), 34898, 0))
	h.wantView("Ay bbs pq dejaron morir esto?", "danipow", "Anierrr", "Las leyendas nunca mueren.", "Me Encanta 1", "#2")
	if q := f.requests("GET", "/api/v1/threads/34898/posts")[0].Query; strings.Contains(q, "page=2") {
		t.Fatalf("a guest must start on page 1, query = %q", q)
	}
	if len(f.requests("GET", "/api/v1/threads/34898/unread")) != 0 || len(f.requests("PUT", "/api/v1/threads/34898/read")) != 0 {
		t.Fatal("a guest must not locate unread posts or mark the thread read")
	}
}

func TestReaderMemberMarksReadOncePerPage(t *testing.T) {
	f := newFakeSermo(t)
	h := newHarness(t, NewThread(testDeps(t, f, true), 34898, 0))
	reads := f.requests("PUT", "/api/v1/threads/34898/read")
	if len(reads) != 1 || decode(t, reads[0].Body)["position"] != float64(1) {
		t.Fatalf("mark read requests = %+v", reads)
	}
	if len(f.requests("GET", "/api/v1/threads/34898/unread")) != 1 {
		t.Fatal("a member must start at the first unread post")
	}
	h.press("j", "k", "]")
	if n := len(f.requests("PUT", "/api/v1/threads/34898/read")); n != 1 {
		t.Fatalf("mark read sent %d times for one page", n)
	}
}

func TestReaderFocusMovesBetweenPosts(t *testing.T) {
	f := newFakeSermo(t)
	h := newHarness(t, NewThread(testDeps(t, f, false), 34898, 0))
	r := h.top().(*reader)
	if r.focusedPost().Id != 477999 {
		t.Fatalf("focus = %d, want the first post", r.focusedPost().Id)
	}
	h.press("]")
	if r.focusedPost().Id != 478003 {
		t.Fatalf("] must focus the next post, focus = %d", r.focusedPost().Id)
	}
	h.press("[")
	if r.focusedPost().Id != 477999 {
		t.Fatalf("[ must focus the previous post, focus = %d", r.focusedPost().Id)
	}
}

func TestReaderGuestReplyShowsLoginToast(t *testing.T) {
	f := newFakeSermo(t)
	h := newHarness(t, NewThread(testDeps(t, f, false), 34898, 0))
	for _, k := range []string{"r", "c", "+", "*", "w"} {
		h.toasts = nil
		h.press(k)
		if h.lastToast() != i18n.T("forum.toast.loginNeeded") {
			t.Fatalf("%s: toast = %q", k, h.lastToast())
		}
	}
	if len(h.stack) != 1 {
		t.Fatalf("a guest must stay on the reader, stack = %d", len(h.stack))
	}
	for _, r := range f.reqs {
		if r.Method != "GET" {
			t.Fatalf("a guest sent %s %s", r.Method, r.Path)
		}
	}
}

func TestReaderReplySendsPost(t *testing.T) {
	f := newFakeSermo(t)
	f.set("GET /api/v1/threads/34898", 200, memberThread(t))
	f.set("POST /api/v1/threads/34898/posts", 201, `{"id":478500,"threadId":34898,"position":2,"author":{"kind":"member","id":7,"username":"albert"},"bodyHtml":"<p>Hola mundo</p>","bodySource":"Hola mundo","state":"visible","createdAt":"2026-10-31T14:05:00Z","editedAt":null,"attachmentCount":0,"attachments":[],"reactions":{"counts":{},"total":0,"mine":null}}`)
	h := newHarness(t, NewThread(testDeps(t, f, true), 34898, 0))
	h.press("r")
	if _, ok := h.top().(*composer); !ok {
		t.Fatalf("r must open the composer, top = %T", h.top())
	}
	if !h.top().CapturesInput() {
		t.Fatal("the composer must capture input")
	}
	h.typeText("Hola mundo")
	before := len(f.requests("GET", "/api/v1/threads/34898/posts"))
	h.press("ctrl+s")
	posts := f.requests("POST", "/api/v1/threads/34898/posts")
	if len(posts) != 1 || decode(t, posts[0].Body)["body"] != "Hola mundo" {
		t.Fatalf("reply requests = %+v", posts)
	}
	if _, ok := h.top().(*reader); !ok || len(h.stack) != 1 {
		t.Fatalf("after send the reader must show, top = %T", h.top())
	}
	if len(f.requests("GET", "/api/v1/threads/34898/posts")) == before {
		t.Fatal("the reader must load the page of the new post")
	}
}

func TestReaderQuotePrefillsComposer(t *testing.T) {
	f := newFakeSermo(t)
	f.set("GET /api/v1/threads/34898", 200, memberThread(t))
	f.set("GET /api/v1/posts/477999", 200, `{"id":477999,"threadId":34898,"position":0,"author":{"kind":"member","id":68725,"username":"danipow"},"bodyHtml":"<p>x</p>","bodySource":"De verdad que ya no queda nada?","state":"visible","createdAt":"2026-07-13T05:14:21.000Z","editedAt":null,"attachmentCount":0,"attachments":[],"reactions":{"counts":{},"total":0,"mine":null}}`)
	f.set("GET /api/v1/threads/34898/unread", 200, `{"threadId":34898,"postId":477999,"position":0}`)
	h := newHarness(t, NewThread(testDeps(t, f, true), 34898, 0))
	h.press("c")
	c, ok := h.top().(*composer)
	if !ok {
		t.Fatalf("c must open the composer, top = %T", h.top())
	}
	want := `[QUOTE="danipow, post: 477999, member: 68725"]De verdad que ya no queda nada?[/QUOTE]`
	if !strings.HasPrefix(c.body.Value(), want) {
		t.Fatalf("body = %q, want prefix %q", c.body.Value(), want)
	}
}

func TestReaderReactSendsPut(t *testing.T) {
	f := newFakeSermo(t)
	f.set("GET /api/v1/threads/34898/posts", 200, memberPosts(t))
	f.set("GET /api/v1/threads/34898/unread", 200, `{"threadId":34898,"postId":477999,"position":0}`)
	f.set("PUT /api/v1/reactions/post/477999", 200, `{"counts":{"1":1},"total":1,"mine":1}`)
	h := newHarness(t, NewThread(testDeps(t, f, true), 34898, 0))
	h.press("+")
	h.wantView("Like", "Me Encanta")
	h.press("enter")
	puts := f.requests("PUT", "/api/v1/reactions/post/477999")
	if len(puts) != 1 || decode(t, puts[0].Body)["reactionTypeId"] != float64(1) {
		t.Fatalf("react requests = %+v", puts)
	}
	h.wantView("Like 1 " + glyphMine)

	f.set("DELETE /api/v1/reactions/post/477999", 200, `{"counts":{},"total":0,"mine":null}`)
	h.press("+", "enter")
	if len(f.requests("DELETE", "/api/v1/reactions/post/477999")) != 1 {
		t.Fatal("the same reaction again must remove it")
	}
}

func TestReaderBookmarkAndWatch(t *testing.T) {
	f := newFakeSermo(t)
	f.set("GET /api/v1/threads/34898", 200, memberThread(t))
	f.set("PUT /api/v1/bookmarks", 200, `{"bookmarked":true}`)
	f.set("PUT /api/v1/threads/34898/watch", 200, `{"ok":true}`)
	f.set("GET /api/v1/threads/34898/unread", 200, `{"threadId":34898,"postId":477999,"position":0}`)
	h := newHarness(t, NewThread(testDeps(t, f, true), 34898, 0))
	h.press("*")
	b := f.requests("PUT", "/api/v1/bookmarks")
	if len(b) != 1 {
		t.Fatalf("bookmark requests = %+v", b)
	}
	body := decode(t, b[0].Body)
	if body["contentType"] != "post" || body["contentId"] != float64(477999) || body["bookmarked"] != true {
		t.Fatalf("bookmark body = %v", body)
	}
	h.press("w")
	if len(f.requests("PUT", "/api/v1/threads/34898/watch")) != 1 {
		t.Fatal("w must watch the thread")
	}
	h.wantView(i18n.T("forum.thread.watching"))
}

func TestReaderProfileKey(t *testing.T) {
	f := newFakeSermo(t)
	h := newHarness(t, NewThread(testDeps(t, f, false), 34898, 0))
	h.press("a")
	if len(h.intent) != 1 || h.intent[0] != (ui.OpenProfileMsg{UserID: 68725}) {
		t.Fatalf("intent = %#v", h.intent)
	}
}

func TestReaderOpensLinkByNumber(t *testing.T) {
	f := newFakeSermo(t)
	f.set("GET /api/v1/threads/34898/posts", 200, `{"items":[{"id":1,"threadId":34898,"position":0,"author":{"kind":"member","id":2,"username":"pepe"},"state":"visible","createdAt":"2026-07-13T05:14:21.000Z","editedAt":null,"bodyHtml":"<p>Mira <a href=\"https://example.com/a\">esto</a></p>","attachmentCount":0,"attachments":[],"reactions":{"counts":{},"total":0,"mine":null},"canEdit":false,"canReact":false}],"nextCursor":null}`)
	h := newHarness(t, NewThread(testDeps(t, f, false), 34898, 0))
	h.wantView("esto [1]")
	h.press("o")
	h.wantView("https://example.com/a")
	r := h.top().(*reader)
	if !r.CapturesInput() {
		t.Fatal("the link picker must capture input")
	}
	h.press("esc")
	if r.CapturesInput() || len(h.stack) != 1 {
		t.Fatal("esc must close the picker only")
	}
}

func TestReaderSpoilersShowContent(t *testing.T) {
	f := newFakeSermo(t)
	f.set("GET /api/v1/threads/34898/posts", 200, `{"items":[{"id":5,"threadId":34898,"position":0,"author":{"kind":"member","id":2,"username":"pepe"},"state":"visible","createdAt":"2026-07-13T05:14:21.000Z","editedAt":null,"bodyHtml":"<p>Hola</p><div class=\"bb-spoiler\" data-spoiler=\"0\" data-spoiler-title=\"Final\"></div>","attachmentCount":0,"attachments":[],"reactions":{"counts":{},"total":0,"mine":null}}],"nextCursor":null}`)
	f.set("GET /api/v1/spoilers", 200, `{"html":"<p>El mayordomo fue.</p>"}`)
	h := newHarness(t, NewThread(testDeps(t, f, false), 34898, 0))
	h.wantView("[spoiler: Final]")
	h.press("s")
	reqs := f.requests("GET", "/api/v1/spoilers")
	if len(reqs) != 1 || !strings.Contains(reqs[0].Query, "contentId=5") || !strings.Contains(reqs[0].Query, "position=0") {
		t.Fatalf("spoiler requests = %+v", reqs)
	}
	h.wantView("[spoiler: Final]", "El mayordomo fue.")
}

func TestReaderPagesWithHAndL(t *testing.T) {
	f := newFakeSermo(t)
	f.set("GET /api/v1/threads/34898", 200, strings.Replace(fixture(t, "thread.json"), `"replyCount":32`, `"replyCount":45`, 1))
	h := newHarness(t, NewThread(testDeps(t, f, false), 34898, 0))
	h.wantView("1/3")
	h.press("l")
	reqs := f.requests("GET", "/api/v1/threads/34898/posts")
	if q := reqs[len(reqs)-1].Query; !strings.Contains(q, "page=2") {
		t.Fatalf("query = %q, want page=2", q)
	}
	h.wantView("2/3")
}

func TestReaderOpensAtPost(t *testing.T) {
	f := newFakeSermo(t)
	f.set("GET /api/v1/threads/34898", 200, strings.Replace(fixture(t, "thread.json"), `"replyCount":32`, `"replyCount":45`, 1))
	f.set("GET /api/v1/posts/478099", 200, `{"id":478099,"threadId":34898,"position":25,"author":{"kind":"member","id":2,"username":"pepe"},"bodyHtml":"","bodySource":"","state":"visible","createdAt":"2026-07-13T05:14:21.000Z","attachments":[],"reactions":{"counts":{},"total":0,"mine":null}}`)
	newHarness(t, NewThread(testDeps(t, f, false), 34898, 478099))
	reqs := f.requests("GET", "/api/v1/threads/34898/posts")
	if len(reqs) != 1 || !strings.Contains(reqs[0].Query, "page=2") {
		t.Fatalf("posts requests = %+v, want page 2", reqs)
	}
}

func TestReaderUnauthorized(t *testing.T) {
	f := newFakeSermo(t)
	f.set("GET /api/v1/threads/34898", 401, `{"error":{"code":"unauthenticated","message":"x"}}`)
	h := newHarness(t, NewThread(testDeps(t, f, true), 34898, 0))
	if len(h.intent) != 1 || h.intent[0] != (ui.UnauthorizedMsg{}) {
		t.Fatalf("intent = %#v", h.intent)
	}
}

func TestReaderEditOwnPost(t *testing.T) {
	f := newFakeSermo(t)
	f.set("GET /api/v1/threads/34898/posts", 200, patched(t, "posts.json", func(v map[string]any) {
		v["items"].([]any)[0].(map[string]any)["canEdit"] = true
	}))
	f.set("GET /api/v1/threads/34898/unread", 200, `{"threadId":34898,"postId":477999,"position":0}`)
	f.set("GET /api/v1/posts/477999", 200, `{"id":477999,"threadId":34898,"position":0,"author":{"kind":"member","id":7,"username":"albert"},"bodyHtml":"<p>viejo</p>","bodySource":"viejo","state":"visible","createdAt":"2026-07-13T05:14:21.000Z","attachments":[],"reactions":{"counts":{},"total":0,"mine":null}}`)
	f.set("PATCH /api/v1/posts/477999", 200, `{"id":477999,"threadId":34898,"position":0,"author":{"kind":"member","id":7,"username":"albert"},"bodyHtml":"<p>viejo nuevo</p>","bodySource":"viejo nuevo","state":"visible","createdAt":"2026-07-13T05:14:21.000Z","attachments":[],"reactions":{"counts":{},"total":0,"mine":null}}`)
	h := newHarness(t, NewThread(testDeps(t, f, true), 34898, 0))
	h.press("e")
	c, ok := h.top().(*composer)
	if !ok || c.body.Value() != "viejo" {
		t.Fatalf("e must open the composer with the source, top = %T", h.top())
	}
	h.typeText(" nuevo")
	h.press("ctrl+s")
	patches := f.requests("PATCH", "/api/v1/posts/477999")
	if len(patches) != 1 || decode(t, patches[0].Body)["body"] != "viejo nuevo" {
		t.Fatalf("edit requests = %+v", patches)
	}
	if _, ok := h.top().(*reader); !ok {
		t.Fatalf("after the edit the reader must show, top = %T", h.top())
	}

	h.press("]", "e")
	if h.lastToast() != i18n.T("forum.toast.cannotEdit") {
		t.Fatalf("toast = %q", h.lastToast())
	}
}

func TestReaderPoll(t *testing.T) {
	f := newFakeSermo(t)
	f.set("GET /api/v1/threads/34898", 200, strings.Replace(memberThread(t), `"exists":false`, `"exists":true`, 1))
	poll := func(chosen bool, votes int) string {
		return fmt.Sprintf(`{"id":3,"threadId":34898,"kind":"poll","maxChoices":1,"allowChange":false,"allowCustom":false,"anonymous":false,"answerMaxLength":100,"closed":false,"closesAt":null,"createdAt":"2026-07-13T05:14:21.000Z","updatedAt":"2026-07-13T05:14:21.000Z","results":"always","resultsShown":true,"voterCount":%d,"mine":{"voted":%t,"answered":0,"completed":false,"correct":0},"permissions":{"canVote":true,"canAddAnswer":false,"canClose":false,"canDelete":false,"canEdit":false,"canViewVoters":false},"questions":[{"id":30,"text":"¿Café o té?","answered":false,"explanation":null,"image":null,"answers":[{"id":301,"text":"Café","chosen":%t,"custom":false,"correct":null,"image":null,"votes":%d},{"id":302,"text":"Té","chosen":false,"custom":false,"correct":null,"image":null,"votes":1}]}]}`, votes, chosen, chosen, votes)
	}
	f.set("GET /api/v1/threads/34898/poll", 200, `{"canCreate":false,"poll":`+poll(false, 3)+`}`)
	f.set("POST /api/v1/polls/3/vote", 200, poll(true, 4))
	h := newHarness(t, NewThread(testDeps(t, f, true), 34898, 0))
	h.wantView(i18n.T("forum.thread.poll"))
	h.press("v")
	h.wantView("¿Café o té?", "Café", "Té", "75%")
	h.press("space", "enter")
	votes := f.requests("POST", "/api/v1/polls/3/vote")
	if len(votes) != 1 {
		t.Fatalf("vote requests = %+v", votes)
	}
	body := decode(t, votes[0].Body)
	if body["questionId"] != float64(30) || fmt.Sprint(body["answerIds"]) != "[301]" {
		t.Fatalf("vote body = %v", body)
	}
	h.wantView("80%")
}
