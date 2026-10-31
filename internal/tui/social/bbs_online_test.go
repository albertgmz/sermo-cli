package social

import (
	"testing"

	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

func visitor(typ string, card, robot any, at string, place any) map[string]any {
	return map[string]any{
		"type": typ, "card": card, "robot": robot, "ip": nil,
		"activity": map[string]any{"at": at, "hidden": false, "place": place},
	}
}

func memberCard(id int, name string) map[string]any {
	return map[string]any{
		"id": id, "username": name, "avatar": nil, "frame": nil, "nameColor": nil, "nameIcon": nil,
		"title": "Habanero", "badges": []any{}, "level": 1, "restricted": false,
	}
}

func onlineServer(t *testing.T) *fakeSermo {
	t.Helper()
	f := newFakeSermo(t)
	f.handle("GET /api/v1/online", 200, map[string]any{
		"items": []any{
			visitor("member", memberCard(12, "Ana"), nil, "2026-10-31T14:02:00Z",
				map[string]any{"kind": "thread", "target": map[string]any{"id": 10, "title": "Elecciones"}}),
			visitor("guest", nil, nil, "2026-10-31T13:50:00Z", map[string]any{"kind": "forumList", "target": nil}),
			visitor("robot", nil, map[string]any{"name": "Googlebot", "url": nil}, "2026-10-30T20:00:00Z", nil),
		},
		"page": 1, "perPage": 25, "total": 3,
	})
	f.handle("GET /api/v1/site/stats", 200, map[string]any{"members": 4182, "posts": 31808, "threads": 2015, "latestMember": nil})
	return f
}

func TestOnlineBBSShowsVisitorsAndOpensProfile(t *testing.T) {
	f := onlineServer(t)
	h := startBBS(t, NewOnline(bbsDeps(t, f, false)))
	h.wantText("| ## |", "Usuario", "Visto", "Dónde", "| 01 |", "Ana", "hace 3m", "Tema: Elecciones",
		"Invitado", "hace 15m", "Índice de foros", "Googlebot", "ayer", "EN LÍNEA (3)", "Miembros: 4.182")
	h.wantFit()
	h.wantHints("Usuarios", "[##] Perfil", "[V]olver")
	h.capture("online")
	h.command("#", "1")
	if open, ok := msgOf[ui.OpenProfileMsg](h.msgs); !ok || open.UserID != 12 {
		t.Fatalf("1 must open the profile of Ana, got %#v", h.msgs)
	}
}

func TestOnlineBBSGuestRowHasNoProfile(t *testing.T) {
	f := onlineServer(t)
	h := startBBS(t, NewOnline(bbsDeps(t, f, false)))
	h.command("#", "2")
	if _, ok := msgOf[ui.OpenProfileMsg](h.msgs); ok {
		t.Fatal("a guest row must not open a profile")
	}
	if toast, ok := msgOf[ui.ToastMsg](h.msgs); !ok || toast.Kind != ui.ToastError {
		t.Fatalf("want an error toast, got %#v", h.msgs)
	}
	h.command("#", "9")
	if n := len(h.msgs); n < 2 {
		t.Fatalf("a missing row must give a toast, got %#v", h.msgs)
	}
}

func TestOnlineModernListsVisitors(t *testing.T) {
	f := onlineServer(t)
	h := start(t, NewOnline(testDeps(t, f, false)))
	h.wantText("Ana", "Tema: Elecciones", "Invitado", "Googlebot")
	h.key("enter")
	if open, ok := msgOf[ui.OpenProfileMsg](h.msgs); !ok || open.UserID != 12 {
		t.Fatalf("enter must open the profile of Ana, got %#v", h.msgs)
	}
}
