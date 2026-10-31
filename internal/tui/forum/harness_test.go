package forum

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

const testToken = "tok-forum"

// request is one request that the fake server got.
type request struct {
	Method, Path, Query, Body string
	Auth                      string
}

// reply is a canned response of the fake server.
type reply struct {
	status int
	body   string
}

// fakeSermo serves fixed responses for exact "METHOD /path" keys and
// records every request.
type fakeSermo struct {
	*httptest.Server
	mu     sync.Mutex
	routes map[string]reply
	reqs   []request
}

func newFakeSermo(t *testing.T) *fakeSermo {
	t.Helper()
	f := &fakeSermo{routes: map[string]reply{}}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Close)
	f.set("GET /api/v1/nodes", 200, fixture(t, "nodes.json"))
	f.set("GET /api/v1/nodes/44", 200, fixture(t, "node.json"))
	f.set("GET /api/v1/nodes/44/threads", 200, fixture(t, "threads.json"))
	f.set("GET /api/v1/thread-prefixes", 200, `{"items":[]}`)
	f.set("GET /api/v1/threads/34898", 200, fixture(t, "thread.json"))
	f.set("GET /api/v1/threads/34898/posts", 200, fixture(t, "posts.json"))
	f.set("GET /api/v1/reaction-types", 200, fixture(t, "reaction-types.json"))
	f.set("GET /api/v1/appearance/colors", 200, fixture(t, "accent.json"))
	f.set("GET /api/v1/nodes/unread", 200, `{"nodeIds":[45]}`)
	f.set("GET /api/v1/threads/34898/unread", 200, `{"threadId":34898,"postId":478003,"position":1}`)
	f.set("PUT /api/v1/threads/34898/read", 200, `{"readPosition":1}`)
	f.set("GET /api/v1/bookmarks/status", 200, `{"bookmarked":[]}`)
	return f
}

func (f *fakeSermo) set(route string, status int, body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.routes[route] = reply{status, body}
}

func (f *fakeSermo) serve(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.reqs = append(f.reqs, request{r.Method, r.URL.Path, r.URL.RawQuery, string(b), r.Header.Get("Authorization")})
	rep, ok := f.routes[r.Method+" "+r.URL.Path]
	f.mu.Unlock()
	if !ok {
		rep = reply{404, `{"error":{"code":"not_found","message":"No se encontró la ruta pedida."}}`}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(rep.status)
	_, _ = io.WriteString(w, rep.body)
}

// requests returns the recorded requests with the method and path.
func (f *fakeSermo) requests(method, path string) []request {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []request
	for _, r := range f.reqs {
		if r.Method == method && r.Path == path {
			out = append(out, r)
		}
	}
	return out
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// patched returns the JSON of a fixture after fn changes the decoded value.
func patched(t *testing.T, name string, fn func(v map[string]any)) string {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal([]byte(fixture(t, name)), &v); err != nil {
		t.Fatal(err)
	}
	fn(v)
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// memberPosts gives the posts fixture with the permissions of a member.
func memberPosts(t *testing.T) string {
	return patched(t, "posts.json", func(v map[string]any) {
		for _, it := range v["items"].([]any) {
			p := it.(map[string]any)
			p["canReact"] = true
		}
	})
}

// memberThread gives the thread fixture where the viewer can reply.
func memberThread(t *testing.T) string {
	return patched(t, "thread.json", func(v map[string]any) {
		v["permissions"] = map[string]any{"canReply": true, "canWatch": true}
	})
}

func decode(t *testing.T, s string) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("decode %q: %v", s, err)
	}
	return v
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// testDeps returns Deps for the fake server. A member has a token and a
// signed-in user.
func testDeps(t *testing.T, f *fakeSermo, member bool) *ui.Deps {
	t.Helper()
	i18n.SetLocale("es")
	t.Cleanup(func() { i18n.SetLocale("es") })
	token := ""
	if member {
		token = testToken
	}
	client, err := api.New(api.Options{Server: f.URL, Token: token, UserAgent: "test"})
	if err != nil {
		t.Fatal(err)
	}
	th := theme.ByID("")
	d := &ui.Deps{
		API: client, Server: f.URL, Config: &config.Config{}, Theme: th, Styles: ui.NewStyles(th),
		Render: render.New(f.URL), NoAnim: true, Profile: colorprofile.TrueColor, Glamour: "notty",
		Now: func() time.Time { return time.Date(2026, 10, 31, 14, 5, 0, 0, time.UTC) },
	}
	if member {
		d.Cred = auth.Credential{Token: token, Source: auth.SourceDevice}
		d.Me = &gen.SelfUser{Id: 7, Username: "albert"}
	}
	return d
}

// harness hosts a stack of screens like the app does. It runs each Cmd
// and feeds the messages back. It records toasts and intent messages.
type harness struct {
	t      *testing.T
	stack  []ui.Screen
	toasts []ui.ToastMsg
	intent []tea.Msg
	w, h   int
	steps  int
}

func newHarness(t *testing.T, s ui.Screen) *harness {
	t.Helper()
	h := &harness{t: t, w: 96, h: 40}
	h.push(s)
	return h
}

func (h *harness) top() ui.Screen { return h.stack[len(h.stack)-1] }

func (h *harness) push(s ui.Screen) {
	h.stack = append(h.stack, s)
	s, cmd := s.Update(tea.WindowSizeMsg{Width: h.w, Height: h.h})
	h.stack[len(h.stack)-1] = s
	h.run(cmd)
	h.run(s.Init())
}

func (h *harness) send(msg tea.Msg) {
	s, cmd := h.top().Update(msg)
	h.stack[len(h.stack)-1] = s
	h.run(cmd)
}

func (h *harness) press(keys ...string) {
	for _, k := range keys {
		h.send(keyMsg(k))
	}
}

func (h *harness) typeText(s string) {
	for _, r := range s {
		h.send(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

// run waits a short time for cmd. A Cmd that waits longer, for example a
// cursor blink, is skipped.
func (h *harness) run(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	h.steps++
	if h.steps > 500 {
		h.t.Fatal("too many steps")
	}
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()
	select {
	case msg := <-ch:
		h.handle(msg)
	case <-time.After(400 * time.Millisecond):
	}
}

var cmdType = reflect.TypeOf(tea.Cmd(nil))

func (h *harness) handle(msg tea.Msg) {
	if h.handleNav(msg) {
		return
	}
	switch m := msg.(type) {
	case nil:
	case tea.BatchMsg:
		for _, c := range m {
			h.run(c)
		}
	default:
		if v := reflect.ValueOf(msg); v.Kind() == reflect.Slice && v.Type().Elem() == cmdType {
			for i := range v.Len() {
				h.run(v.Index(i).Interface().(tea.Cmd))
			}
			return
		}
		h.send(msg)
	}
}

// handleNav handles the messages that the app handles. It returns false
// for other messages.
func (h *harness) handleNav(msg tea.Msg) bool {
	switch m := msg.(type) {
	case ui.PushMsg:
		h.push(m.Screen)
	case ui.PopMsg:
		h.stack = h.stack[:len(h.stack)-1]
	case ui.ReplaceMsg:
		h.stack = h.stack[:len(h.stack)-1]
		h.push(m.Screen)
	case ui.ToastMsg:
		h.toasts = append(h.toasts, m)
	case ui.OpenNodeMsg, ui.OpenThreadMsg, ui.OpenProfileMsg, ui.LoginMsg, ui.UnauthorizedMsg:
		h.intent = append(h.intent, m)
	default:
		return false
	}
	return true
}

func (h *harness) view() string { return ansi.Strip(h.top().View(h.w, h.h)) }

func (h *harness) lastToast() string {
	if len(h.toasts) == 0 {
		return ""
	}
	return h.toasts[len(h.toasts)-1].Text
}

func (h *harness) wantView(want ...string) {
	h.t.Helper()
	v := h.view()
	for _, w := range want {
		if !strings.Contains(v, w) {
			h.t.Fatalf("view has no %q:\n%s", w, v)
		}
	}
}

func keyMsg(k string) tea.KeyPressMsg {
	named := map[string]rune{
		"enter": tea.KeyEnter, "esc": tea.KeyEscape, "tab": tea.KeyTab, "space": tea.KeySpace,
		"up": tea.KeyUp, "down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight,
		"pgup": tea.KeyPgUp, "pgdown": tea.KeyPgDown, "backspace": tea.KeyBackspace,
	}
	if c, ok := named[k]; ok {
		m := tea.KeyPressMsg{Code: c}
		if c == tea.KeySpace {
			m.Text = " "
		}
		return m
	}
	if rest, ok := strings.CutPrefix(k, "ctrl+"); ok {
		return tea.KeyPressMsg{Code: []rune(rest)[0], Mod: tea.ModCtrl}
	}
	return tea.KeyPressMsg{Code: []rune(k)[0], Text: k}
}
