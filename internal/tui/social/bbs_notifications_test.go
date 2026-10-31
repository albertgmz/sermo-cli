package social

import (
	"testing"

	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

func TestNotificationsBBSTable(t *testing.T) {
	f := notificationServer(t)
	h := startBBS(t, NewNotifications(bbsDeps(t, f, true)))
	h.wantText("| ## | (*) | Aviso", "Cuándo", "| 01 | (*) | Ana y 2 más respondieron en «Elecciones»",
		"| 02 | ( ) | Luis empezó a seguirte", "hace 10m", "(*) sin leer", "AVISOS")
	h.wantFit()
	h.wantHints("Avisos", "[##] Abrir", "[L]eer todo", "[U] Solo sin leer", "[<] [>] Página")
	h.capture("notifications")
	if q := f.find("GET", "/api/v1/notifications")[0].Query; q.Get("limit") != "10" {
		t.Fatalf("list query = %v, want the rows that fit: limit 10", q)
	}
	if n := len(f.find("PUT", "/api/v1/notifications/viewed")); n != 1 {
		t.Fatalf("viewed sent %d times, want 1", n)
	}
}

func TestNotificationsBBSNumberOpensAndMarksRead(t *testing.T) {
	f := notificationServer(t)
	h := startBBS(t, NewNotifications(bbsDeps(t, f, true)))
	h.command("#", "3")
	if open, ok := msgOf[ui.OpenConversationMsg](h.msgs); !ok || open.ID != 44 {
		t.Fatalf("3 must open the conversation 44, got %#v", h.msgs)
	}
	reads := f.find("PUT", "/api/v1/notifications/read")
	if len(reads) != 1 || !jsonEq(t, reads[0].Body, `{"ids":[3]}`) {
		t.Fatalf("read requests = %#v", reads)
	}
	h.wantText("| 03 | ( ) |")
}

func TestNotificationsBBSReadAllAndUnreadOnly(t *testing.T) {
	f := notificationServer(t)
	h := startBBS(t, NewNotifications(bbsDeps(t, f, true)))
	h.command("L", "")
	reads := f.find("PUT", "/api/v1/notifications/read")
	if len(reads) != 1 || !jsonEq(t, reads[0].Body, `{"all":true}`) {
		t.Fatalf("read all requests = %#v", reads)
	}
	h.wantText("| 01 | ( ) |")
	h.command("U", "")
	lists := f.find("GET", "/api/v1/notifications")
	if len(lists) != 2 || lists[1].Query.Get("unreadOnly") != "true" {
		t.Fatalf("U must load the unread ones, got %#v", lists)
	}
	h.wantText("solo sin leer")
}

func TestNotificationsBBSNextPageUsesCursor(t *testing.T) {
	f := newFakeSermo(t)
	var items []any
	for i := 1; i <= 10; i++ {
		items = append(items, notification(i, "member.followed", "user", i, nil, []any{user(13, "Luis")}, 1, nil, true))
	}
	f.handle("GET /api/v1/notifications", 200, map[string]any{"items": items, "nextCursor": "c2", "total": nil, "unreadCount": 0})
	f.handle("PUT /api/v1/notifications/viewed", 200, map[string]any{})
	h := startBBS(t, NewNotifications(bbsDeps(t, f, true)))
	h.command(">", "")
	lists := f.find("GET", "/api/v1/notifications")
	if len(lists) != 2 || lists[1].Query.Get("cursor") != "c2" {
		t.Fatalf("> must load the next page with the cursor, got %#v", lists)
	}
	h.command("<", "")
	if n := len(f.find("GET", "/api/v1/notifications")); n != 2 {
		t.Fatalf("< must show the loaded rows again, got %d requests", n)
	}
}
