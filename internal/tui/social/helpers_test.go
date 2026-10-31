package social

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/api/gen"
	"github.com/albertgmz/sermo-cli/internal/auth"
	"github.com/albertgmz/sermo-cli/internal/config"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/render"
	"github.com/albertgmz/sermo-cli/internal/theme"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// call is one request that the fake server got.
type call struct {
	Method, Path string
	Query        url.Values
	Body         string
}

// fakeSermo is a Sermo server. Each test adds the routes it needs.
type fakeSermo struct {
	*httptest.Server
	mux   *http.ServeMux
	mu    sync.Mutex
	calls []call
}

func newFakeSermo(t *testing.T) *fakeSermo {
	t.Helper()
	f := &fakeSermo{mux: http.NewServeMux()}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.calls = append(f.calls, call{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(), Body: string(body)})
		f.mu.Unlock()
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		f.mux.ServeHTTP(w, r)
	}))
	t.Cleanup(f.Close)
	return f
}

// handle serves v as JSON with the status for the pattern.
func (f *fakeSermo) handle(pattern string, status int, v any) {
	f.mux.HandleFunc(pattern, func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, status, v) })
}

// find returns the requests with the method and the path.
func (f *fakeSermo) find(method, path string) []call {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []call
	for _, c := range f.calls {
		if c.Method == method && c.Path == path {
			out = append(out, c)
		}
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func errBody(code, message string) map[string]any {
	return map[string]any{"error": map[string]any{"code": code, "message": message}}
}

var testNow = time.Date(2026, 10, 31, 14, 5, 0, 0, time.UTC)

// testDeps returns Deps for the fake server. A member signs in as albert
// with the id 7. A guest has no user.
func testDeps(t *testing.T, f *fakeSermo, member bool) *ui.Deps {
	t.Helper()
	i18n.SetLocale("es")
	t.Cleanup(func() { i18n.SetLocale("es") })
	token := ""
	if member {
		token = "tok"
	}
	client, err := api.New(api.Options{Server: f.URL, Token: token, UserAgent: "test"})
	if err != nil {
		t.Fatal(err)
	}
	th := theme.ByID("")
	d := &ui.Deps{
		API: client, Server: f.URL, Config: &config.Config{}, Theme: th, Styles: ui.NewStyles(th),
		Render: render.New(f.URL), NoAnim: true, Profile: colorprofile.TrueColor, Glamour: "notty",
		Now: func() time.Time { return testNow },
	}
	if member {
		d.Cred = auth.Credential{Token: token, Source: auth.SourceDevice}
		d.Me = &gen.SelfUser{Id: 7, Username: "albert"}
	}
	return d
}

// harness runs a screen like the app does. It feeds the messages of the
// commands back to the screen and keeps all of them.
type harness struct {
	t    *testing.T
	s    ui.Screen
	msgs []tea.Msg
	w, h int
}

const cmdWait = 1500 * time.Millisecond

func start(t *testing.T, s ui.Screen) *harness {
	t.Helper()
	h := &harness{t: t, s: s, w: 100, h: 30}
	h.send(tea.WindowSizeMsg{Width: h.w, Height: h.h})
	h.run(s.Init())
	return h
}

func (h *harness) send(msg tea.Msg) {
	s, cmd := h.s.Update(msg)
	h.s = s
	h.run(cmd)
}

func (h *harness) run(cmd tea.Cmd) {
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 && len(h.msgs) < 300 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		msg, ok := runCmd(c)
		if !ok || msg == nil {
			continue
		}
		if cmds, isList := cmdList(msg); isList {
			queue = append(queue, cmds...)
			continue
		}
		h.msgs = append(h.msgs, msg)
		s, next := h.s.Update(msg)
		h.s = s
		queue = append(queue, next)
	}
}

// cmdList returns the commands of a batch or a sequence message.
func cmdList(msg tea.Msg) ([]tea.Cmd, bool) {
	v := reflect.ValueOf(msg)
	if v.Kind() != reflect.Slice || v.Type().Elem() != reflect.TypeFor[tea.Cmd]() {
		return nil, false
	}
	out := make([]tea.Cmd, v.Len())
	for i := range out {
		out[i] = v.Index(i).Interface().(tea.Cmd)
	}
	return out, true
}

func runCmd(c tea.Cmd) (tea.Msg, bool) {
	ch := make(chan tea.Msg, 1)
	go func() { ch <- c() }()
	select {
	case msg := <-ch:
		return msg, true
	case <-time.After(cmdWait):
		return nil, false
	}
}

func (h *harness) key(keys ...string) {
	for _, k := range keys {
		h.send(keyMsg(k))
	}
}

// typeText sends each rune of s as a key press.
func (h *harness) typeText(s string) {
	for _, r := range s {
		h.send(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func keyMsg(s string) tea.KeyPressMsg {
	named := map[string]rune{
		"enter": tea.KeyEnter, "esc": tea.KeyEscape, "tab": tea.KeyTab, "down": tea.KeyDown,
		"up": tea.KeyUp, "backspace": tea.KeyBackspace,
	}
	if c, ok := named[s]; ok {
		return tea.KeyPressMsg{Code: c}
	}
	if ch, ok := strings.CutPrefix(s, "ctrl+"); ok {
		return tea.KeyPressMsg{Code: []rune(ch)[0], Mod: tea.ModCtrl}
	}
	return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
}

func (h *harness) screen() string { return ansi.Strip(h.s.View(h.w, h.h)) }

func (h *harness) wantText(parts ...string) {
	h.t.Helper()
	out := h.screen()
	for _, p := range parts {
		if !strings.Contains(out, p) {
			h.t.Errorf("screen misses %q:\n%s", p, out)
		}
	}
}

func msgOf[T any](msgs []tea.Msg) (T, bool) {
	for _, m := range msgs {
		if v, ok := m.(T); ok {
			return v, true
		}
	}
	var zero T
	return zero, false
}

// jsonEq reports whether the JSON texts a and b hold the same value.
func jsonEq(t *testing.T, a, b string) bool {
	t.Helper()
	var va, vb any
	if err := json.Unmarshal([]byte(a), &va); err != nil {
		t.Fatalf("bad JSON %q: %v", a, err)
	}
	if err := json.Unmarshal([]byte(b), &vb); err != nil {
		t.Fatalf("bad JSON %q: %v", b, err)
	}
	return reflect.DeepEqual(va, vb)
}

// Fixtures with the shapes of api/openapi.json.

func member(id int, name string) map[string]any {
	return map[string]any{"kind": "member", "id": id, "username": name}
}

func user(id int, name string) map[string]any {
	return map[string]any{"id": id, "username": name}
}

func reactions() map[string]any {
	return map[string]any{"counts": map[string]any{}, "total": 0, "mine": nil}
}

func notification(id int, typ, contentType string, contentID int, threadID any, actors []any, count int, title any, read bool) map[string]any {
	return map[string]any{
		"id": id, "type": typ, "contentType": contentType, "contentId": contentID, "threadId": threadID,
		"actors": actors, "actorCount": count, "contentTitle": title, "url": "/posts/" + itoa(contentID),
		"data": map[string]any{}, "createdAt": "2026-10-31T13:55:00Z", "updatedAt": "2026-10-31T13:55:00Z", "read": read,
	}
}

func message(id, convID int, author map[string]any, html string) map[string]any {
	return map[string]any{
		"id": id, "conversationId": convID, "author": author, "bodyHtml": html, "state": "visible",
		"createdAt": "2026-10-30T10:00:00Z", "editedAt": nil, "attachments": []any{}, "attachmentCount": 0,
		"reactions": reactions(), "canEdit": false, "canDelete": false, "canHardDelete": false, "canReport": true,
		"deletion": nil,
	}
}

func lastMessage(id int, author map[string]any) map[string]any {
	return map[string]any{"messageId": id, "sentAt": "2026-10-31T12:05:00Z", "user": author}
}

func conversationSummary(id int, title string, unread bool) map[string]any {
	return map[string]any{
		"id": id, "title": title, "isUnread": unread, "createdAt": "2026-10-20T10:00:00Z",
		"starter": user(12, "Ana"), "participantCount": 3, "messageCount": 4,
		"lastMessage": lastMessage(80, member(14, "Pedro")),
	}
}

func convJSON(id int, title string) map[string]any {
	c := conversationSummary(id, title, true)
	c["participants"] = []any{
		map[string]any{"user": user(12, "Ana"), "state": "active"},
		map[string]any{"user": user(7, "albert"), "state": "active"},
	}
	for _, k := range []string{"canInvite", "canLock", "canRename", "isLocked"} {
		c[k] = false
	}
	c["canReply"] = true
	return c
}

func profilePost(id int, author map[string]any, html string, comments int) map[string]any {
	return map[string]any{
		"id": id, "profileUserId": 9, "author": author, "state": "visible", "createdAt": "2026-10-31T11:05:00Z",
		"editedAt": nil, "bodyHtml": html, "attachmentCount": 0, "attachments": []any{}, "reactions": reactions(),
		"commentCount": comments, "latestComments": []any{}, "canEdit": false, "canDelete": false,
		"canReport": false, "canComment": true, "deletion": nil,
	}
}

func profileJSON(id int, name string) map[string]any {
	return map[string]any{
		"id": id, "username": name, "groupTitle": "Miembro", "displayGroup": nil,
		"createdAt": "2021-03-04T05:06:07Z", "about": "Me gusta el béisbol.", "avatar": nil, "cover": nil,
		"postCount": 321, "reactionScore": 45, "canPostOnWall": true,
		"card": map[string]any{
			"id": id, "username": name, "avatar": nil, "frame": nil, "nameColor": "#ff8800", "nameIcon": nil,
			"title": "Habanero", "badges": []any{}, "level": 3, "restricted": false,
		},
		"customTitle": nil, "signatureHtml": nil, "location": "La Habana", "website": "", "birthday": nil,
		"socials": map[string]any{"discord": "", "telegram": "", "x": ""}, "followerCount": 1, "followingCount": 2,
		"credits": 0, "awardXp": 0, "level": map[string]any{"level": 3, "currentLevelXp": 0, "nextLevelXp": 10},
		"awardSlots": 0, "referralCount": 0, "lastActivityAt": nil, "device": nil,
		"viewer": map[string]any{"isFollowing": false, "isBlocking": false}, "isAdult": false,
		"seo": map[string]any{"title": "", "description": "", "canonical": "", "redirect": false, "robots": "index,follow", "jsonLd": []any{}},
	}
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
