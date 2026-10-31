package tui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/zalando/go-keyring"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/auth"
	"github.com/albertgmz/sermo-cli/internal/config"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// fakeSermo is a Sermo server with the endpoints that the app core calls.
type fakeSermo struct {
	*httptest.Server
	notifications atomic.Int32
	conversations atomic.Int32
	meStatus      atomic.Int32 // 0 means 200
	counterStatus atomic.Int32 // 0 means 200
	tokenPolls    atomic.Int32
	unreadCalls   atomic.Int32 // signed-in calls of nodes.unread
}

const testToken = "tok-123"

func newFakeSermo(t *testing.T) *fakeSermo {
	t.Helper()
	f := &fakeSermo{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/site", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"name": "Foros Cuba", "description": "De Cuba para el mundo"})
	})
	mux.HandleFunc("GET /api/v1/me", f.me)
	mux.HandleFunc("GET /api/v1/me/counters", func(w http.ResponseWriter, r *http.Request) {
		if s := f.counterStatus.Load(); s != 0 {
			writeJSON(w, int(s), errBody("forbidden"))
			return
		}
		if !f.authorized(r) {
			writeJSON(w, 401, errBody("unauthenticated"))
			return
		}
		writeJSON(w, 200, map[string]any{
			"notifications": f.notifications.Load(), "conversations": f.conversations.Load(),
			"openReports": nil, "pendingApprovals": nil, "pendingSignups": nil,
		})
	})
	mux.HandleFunc("GET /api/v1/nodes", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"items": []any{}})
	})
	mux.HandleFunc("GET /api/v1/nodes/unread", func(w http.ResponseWriter, r *http.Request) {
		if f.authorized(r) {
			f.unreadCalls.Add(1)
		}
		writeJSON(w, 200, map[string]any{"nodeIds": []int{}})
	})
	mux.HandleFunc("POST /api/auth/device/code", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{
			"device_code": "dev-1", "user_code": "WXYZ-1234",
			"verification_uri": f.URL + "/device", "verification_uri_complete": f.URL + "/device?user_code=WXYZ-1234",
			"expires_in": 900, "interval": 1,
		})
	})
	mux.HandleFunc("POST /api/auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		if f.tokenPolls.Add(1) < 2 {
			writeJSON(w, 400, map[string]any{"error": "authorization_pending"})
			return
		}
		writeJSON(w, 200, map[string]any{"access_token": testToken, "token_type": "Bearer"})
	})
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

func (f *fakeSermo) authorized(r *http.Request) bool {
	return r.Header.Get("Authorization") == "Bearer "+testToken
}

func (f *fakeSermo) me(w http.ResponseWriter, r *http.Request) {
	if s := f.meStatus.Load(); s != 0 {
		writeJSON(w, int(s), errBody("unauthenticated"))
		return
	}
	user := any(nil)
	if f.authorized(r) {
		user = map[string]any{
			"id": 7, "username": "albert", "email": "a@example.com", "groupId": 2,
			"createdAt": "2024-01-02T03:04:05Z", "displayGroup": nil, "language": nil,
			"keyboardShortcuts": true, "homeTab": nil, "feedView": "cards", "theme": nil,
			"openHomeTab": "latest", "pendingAcceptance": []any{}, "awaitingApproval": false,
			"sessionId": nil, "needsUsername": false,
		}
	}
	writeJSON(w, 200, map[string]any{"user": user, "permissions": map[string]any{}, "resolvedPermissions": map[string]any{}})
}

func errBody(code string) map[string]any {
	return map[string]any{"error": map[string]any{"code": code, "message": "Debes iniciar sesión para hacer eso."}}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// testDeps returns Deps for the fake server. A token signs in as a device
// login. The config directory is a temp directory and the keyring is a mock.
func testDeps(t *testing.T, f *fakeSermo, token string) *ui.Deps {
	t.Helper()
	keyring.MockInit()
	t.Setenv("SERMO_CONFIG_DIR", t.TempDir())
	t.Setenv("SERMO_API_KEY", "")
	i18n.SetLocale("es")
	t.Cleanup(func() { i18n.SetLocale("es") })
	client, err := api.New(api.Options{Server: f.URL, Token: token, UserAgent: "test"})
	if err != nil {
		t.Fatal(err)
	}
	cred := auth.Credential{}
	if token != "" {
		cred = auth.Credential{Token: token, Source: auth.SourceDevice}
	}
	return &ui.Deps{
		API: client, Cred: cred, Store: auth.NewStore(), Server: f.URL,
		Config:  &config.Config{Effect: "plasma"},
		NoAnim:  true,
		Profile: colorprofile.TrueColor,
		Now:     func() time.Time { return time.Date(2026, 10, 31, 14, 5, 0, 0, time.UTC) },
	}
}

func noSleep(ctx context.Context, _ time.Duration) error { return ctx.Err() }

func newTestModel(t *testing.T, d *ui.Deps, start Start) *model {
	t.Helper()
	m := newModel(context.Background(), d, Options{Start: start, Sleep: noSleep})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m
}

// drive runs cmd and the commands that follow from its messages, and feeds
// each message to m. A command that blocks longer than wait, for example a
// long tick, is skipped. It returns the messages in order.
func drive(m *model, cmd tea.Cmd, wait time.Duration) []tea.Msg {
	var out []tea.Msg
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 && len(out) < 200 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		msg, ok := run(c, wait)
		if !ok || msg == nil {
			continue
		}
		if batch, isBatch := msg.(tea.BatchMsg); isBatch {
			queue = append(queue, batch...)
			continue
		}
		if o, owned := msg.(ownedMsg); owned {
			out = append(out, o.msg)
		} else {
			out = append(out, msg)
		}
		_, next := m.Update(msg)
		queue = append(queue, next)
	}
	return out
}

func run(c tea.Cmd, wait time.Duration) (tea.Msg, bool) {
	ch := make(chan tea.Msg, 1)
	go func() { ch <- c() }()
	select {
	case msg := <-ch:
		return msg, true
	case <-time.After(wait):
		return nil, false
	}
}

func screenText(m *model) string { return ansi.Strip(m.View().Content) }

func pressKey(m *model, s string) tea.Cmd {
	var k tea.KeyPressMsg
	switch s {
	case "esc":
		k = tea.KeyPressMsg{Code: tea.KeyEscape}
	case "enter":
		k = tea.KeyPressMsg{Code: tea.KeyEnter}
	case "right":
		k = tea.KeyPressMsg{Code: tea.KeyRight}
	case "down":
		k = tea.KeyPressMsg{Code: tea.KeyDown}
	default:
		k = tea.KeyPressMsg{Code: []rune(strings.ToLower(s))[0], Text: s}
	}
	_, cmd := m.Update(k)
	return cmd
}

func hasMsg[T any](msgs []tea.Msg) bool {
	for _, m := range msgs {
		if _, ok := m.(T); ok {
			return true
		}
	}
	return false
}
