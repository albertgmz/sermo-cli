package social

import (
	"strings"
	"testing"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

func TestProfileBBSCardAndWall(t *testing.T) {
	f := profileServer(t)
	h := startBBS(t, NewProfile(bbsDeps(t, f, true), 9))
	h.wantText("+=====", "Yuniel", "Habanero", "Miembro", "Desde: 4-Mar-2021", "Mensajes: 321", "Reacciones: 45",
		"Ubicación: La Habana", "| ## | "+i18n.T("bbs.social.profile.author"), "Mensaje", "Cuándo", "| 01 | Ana", "¡Feliz cumpleaños, Yuniel!",
		"| 02 | Luis", "hace 3h", "MURO")
	h.wantFit()
	h.wantHints("Yuniel", "[##] Abrir", "[W] Escribir", "[C] n Comentar")
	if h.hasCommand("F") || h.hasCommand("S") {
		t.Fatal("the profile of another member must not have F and S")
	}
	h.capture("profile")
	h.command("#", "1")
	if n := len(f.find("GET", "/api/v1/profile-posts/501/comments")); n != 1 {
		t.Fatalf("1 must load the comments once, got %d requests", n)
	}
	h.wantText("Ana", "¡Feliz cumpleaños, Yuniel!", "Yuniel", "¡Gracias, Ana!")
	h.wantFit()
	h.wantHints("Yuniel", "[C]omentar", "[V]olver")
	h.capture("profile-post")
	h.command("V", "")
	h.wantText("| 02 | Luis")
}

func TestProfileBBSWriteAndComment(t *testing.T) {
	f := profileServer(t)
	h := startBBS(t, NewProfile(bbsDeps(t, f, true), 9))
	h.command("W", "")
	h.wantText("Editor de líneas", "Yuniel")
	h.typeText("hola")
	h.key("enter")
	h.typeText("/s")
	h.key("enter")
	posts := f.find("POST", "/api/v1/users/9/profile-posts")
	if len(posts) != 1 || !jsonEq(t, posts[0].Body, `{"body":"hola"}`) {
		t.Fatalf("profile post requests = %#v", posts)
	}
	h.command("C", "1")
	h.typeText("bien")
	h.key("enter")
	h.typeText("/S")
	h.key("enter")
	comments := f.find("POST", "/api/v1/profile-posts/501/comments")
	if len(comments) != 1 || !jsonEq(t, comments[0].Body, `{"body":"bien"}`) {
		t.Fatalf("comment requests = %#v", comments)
	}
}

func TestProfileBBSGuestCannotWrite(t *testing.T) {
	f := profileServer(t)
	h := startBBS(t, NewProfile(bbsDeps(t, f, false), 9))
	h.command("W", "")
	if h.s.CapturesInput() {
		t.Fatal("a guest must not get the editor")
	}
	if _, ok := msgOf[ui.ToastMsg](h.msgs); !ok {
		t.Fatalf("guest must get a toast, got %#v", h.msgs)
	}
}

func TestOwnProfileBBSBookmarksAndWatches(t *testing.T) {
	f := ownServer(t)
	h := startBBS(t, NewProfile(bbsDeps(t, f, true), 0))
	h.wantHints("albert", "[F]avoritos", "[S]eguidos")
	h.wantText("Nadie escribió en este perfil todavía.")
	h.command("F", "")
	h.wantText("FAVORITOS", "| ## | Título", "| 01 | Elecciones", "Ana", "29-Oct")
	h.wantFit()
	h.capture("profile-bookmarks")
	if q := f.find("GET", "/api/v1/bookmarks")[0].Query; q.Get("limit") == "20" {
		t.Fatalf("bookmarks must ask for the rows that fit, query %v", q)
	}
	h.command("#", "1")
	if open, ok := msgOf[ui.OpenThreadMsg](h.msgs); !ok || open != (ui.OpenThreadMsg{ThreadID: 10, PostID: 55}) {
		t.Fatalf("bookmark 1 gave %#v", h.msgs)
	}
	h.command("S", "")
	h.wantText("SEGUIDOS", "| 01 | Béisbol cubano", "hace 1h")
	h.wantHints("albert", "[X] n Dejar de seguir")
	h.capture("profile-watched")
	h.command("X", "1")
	if n := len(f.find("DELETE", "/api/v1/threads/33/watch")); n != 1 {
		t.Fatalf("X 1 sent %d unwatch requests, want 1", n)
	}
	if strings.Contains(h.screen(), "Béisbol cubano") {
		t.Fatalf("the unwatched thread must leave the list:\n%s", h.screen())
	}
	h.command("S", "")
	h.wantText("MURO")
}
