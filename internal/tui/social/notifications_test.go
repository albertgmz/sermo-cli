package social

import (
	"bytes"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

func notificationServer(t *testing.T) *fakeSermo {
	t.Helper()
	f := newFakeSermo(t)
	conv := notification(3, "conversation.message", "conversation_message", 300, nil, []any{user(14, "Pedro")}, 1, "Viaje", false)
	conv["data"] = map[string]any{"conversationId": 44}
	f.handle("GET /api/v1/notifications", 200, map[string]any{
		"items": []any{
			notification(1, "thread.watched", "post", 55, 10, []any{user(12, "Ana")}, 3, "Elecciones", false),
			notification(2, "member.followed", "user", 7, nil, []any{user(13, "Luis")}, 1, nil, true),
			conv,
			notification(4, "something.new", "thing", 9, nil, []any{}, 0, "Cosa rara", true),
		},
		"nextCursor": nil, "total": nil, "unreadCount": 2,
	})
	f.handle("PUT /api/v1/notifications/viewed", 200, map[string]any{})
	f.handle("PUT /api/v1/notifications/read", 200, map[string]any{"unreadCount": 1})
	return f
}

func TestNotificationsShowSentencesAndMarkViewed(t *testing.T) {
	f := notificationServer(t)
	h := start(t, NewNotifications(testDeps(t, f, true)))
	h.wantText(
		"Ana y 2 más respondieron en «Elecciones»",
		"Luis empezó a seguirte",
		"Pedro te escribió en «Viaje»",
		"Novedad: Cosa rara",
		"hace 10 min",
	)
	if n := len(f.find("PUT", "/api/v1/notifications/viewed")); n != 1 {
		t.Fatalf("viewed sent %d times, want 1", n)
	}
	if q := f.find("GET", "/api/v1/notifications")[0].Query; q.Get("limit") != "20" {
		t.Fatalf("list query = %v, want limit 20", q)
	}
}

func TestNotificationEnterOpensThreadAndMarksRead(t *testing.T) {
	f := notificationServer(t)
	h := start(t, NewNotifications(testDeps(t, f, true)))
	h.key("enter")
	open, ok := msgOf[ui.OpenThreadMsg](h.msgs)
	if !ok || open != (ui.OpenThreadMsg{ThreadID: 10, PostID: 55}) {
		t.Fatalf("got %#v, want OpenThreadMsg{10, 55}", open)
	}
	reads := f.find("PUT", "/api/v1/notifications/read")
	if len(reads) != 1 || !jsonEq(t, reads[0].Body, `{"ids":[1]}`) {
		t.Fatalf("read requests = %#v", reads)
	}
}

func TestNotificationEnterOnMessageOpensConversation(t *testing.T) {
	f := notificationServer(t)
	h := start(t, NewNotifications(testDeps(t, f, true)))
	h.key("j", "j", "enter")
	open, ok := msgOf[ui.OpenConversationMsg](h.msgs)
	if !ok || open.ID != 44 {
		t.Fatalf("got %#v, want OpenConversationMsg{44}", open)
	}
}

func TestNotificationsMarkAllAndUnreadOnly(t *testing.T) {
	f := notificationServer(t)
	h := start(t, NewNotifications(testDeps(t, f, true)))
	h.key("a")
	reads := f.find("PUT", "/api/v1/notifications/read")
	if len(reads) != 1 || !jsonEq(t, reads[0].Body, `{"all":true}`) {
		t.Fatalf("read requests = %#v", reads)
	}
	h.key("u")
	lists := f.find("GET", "/api/v1/notifications")
	if last := lists[len(lists)-1]; last.Query.Get("unreadOnly") != "true" {
		t.Fatalf("u must load unread only, query %v", last.Query)
	}
}

func TestNotificationsGuestGetsLogin(t *testing.T) {
	f := notificationServer(t)
	h := start(t, NewNotifications(testDeps(t, f, false)))
	if _, ok := msgOf[ui.LoginMsg](h.msgs); !ok {
		t.Fatalf("guest must get LoginMsg, got %#v", h.msgs)
	}
	if n := len(f.find("GET", "/api/v1/notifications")); n != 0 {
		t.Fatalf("guest sent %d list requests", n)
	}
}

func TestNotificationsUnauthorized(t *testing.T) {
	f := newFakeSermo(t)
	f.handle("GET /api/v1/notifications", 401, errBody("unauthenticated", "x"))
	f.handle("PUT /api/v1/notifications/viewed", 401, errBody("unauthenticated", "x"))
	h := start(t, NewNotifications(testDeps(t, f, true)))
	if _, ok := msgOf[ui.UnauthorizedMsg](h.msgs); !ok {
		t.Fatalf("401 must give UnauthorizedMsg, got %#v", h.msgs)
	}
}

// host runs one screen as a Bubble Tea program for teatest.
type host struct {
	s    ui.Screen
	w, h int
}

func (m *host) Init() tea.Cmd { return m.s.Init() }

func (m *host) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.w, m.h = ws.Width, ws.Height
	}
	if k, ok := msg.(tea.KeyPressMsg); ok && k.String() == "ctrl+c" {
		return m, tea.Quit
	}
	s, cmd := m.s.Update(msg)
	m.s = s
	return m, cmd
}

func (m *host) View() tea.View { return tea.NewView(m.s.View(m.w, m.h)) }

func TestNotificationsInProgram(t *testing.T) {
	f := notificationServer(t)
	tm := teatest.NewTestModel(t, &host{s: NewNotifications(testDeps(t, f, true))}, teatest.WithInitialTermSize(100, 30))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains([]byte(ansi.Strip(string(b))), []byte("empezó a seguirte"))
	}, teatest.WithDuration(3*time.Second))
	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}
