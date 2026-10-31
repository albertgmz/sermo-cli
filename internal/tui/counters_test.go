package tui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// withNotifications adds the notification list to f. A view of the list
// sets the notification counter of f to 0.
func withNotifications(t *testing.T, f *fakeSermo) *fakeSermo {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("/", f.Config.Handler)
	mux.HandleFunc("GET /api/v1/notifications", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{"items": []any{}, "nextCursor": nil, "total": nil, "unreadCount": 0})
	})
	mux.HandleFunc("PUT /api/v1/notifications/viewed", func(w http.ResponseWriter, _ *http.Request) {
		f.notifications.Store(0)
		writeJSON(w, 200, map[string]any{})
	})
	g := &fakeSermo{Server: httptest.NewServer(mux)}
	t.Cleanup(g.Close)
	return g
}

func TestViewedNotificationsResetTheCounterBaseline(t *testing.T) {
	f := newFakeSermo(t)
	f.notifications.Store(3)
	m := newTestModel(t, testDeps(t, withNotifications(t, f), testToken), StartHome)
	drive(m, m.Init(), wait)
	if m.counters.Notifications != 3 {
		t.Fatalf("start count = %d, want 3", m.counters.Notifications)
	}
	drive(m, pressKey(m, "n"), wait)
	if m.counters.Notifications != 0 {
		t.Fatalf("after the view the app must load the counters, count = %d", m.counters.Notifications)
	}
	if m.toast != nil {
		t.Fatalf("the new baseline must not toast: %q", m.toast.Text)
	}
	f.notifications.Store(2)
	drive(m, func() tea.Msg { return pollMsg{gen: m.pollGen} }, wait)
	if out := screenText(m); !strings.Contains(out, "Tienes 2 notificaciones nuevas") {
		t.Fatalf("no toast for new notifications:\n%s", out)
	}
}
