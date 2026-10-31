package social

import (
	"strings"
	"testing"

	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

func profileServer(t *testing.T) *fakeSermo {
	t.Helper()
	f := newFakeSermo(t)
	f.handle("GET /api/v1/users/9", 200, profileJSON(9, "Yuniel"))
	f.handle("GET /api/v1/users/9/profile-posts", 200, map[string]any{
		"items": []any{
			profilePost(501, member(12, "Ana"), "<p>¡Feliz cumpleaños, Yuniel!</p>", 1),
			profilePost(502, member(13, "Luis"), "<p>Nos vemos en el Malecón.</p>", 0),
		},
		"nextCursor": nil,
	})
	f.handle("GET /api/v1/profile-posts/501/comments", 200, map[string]any{
		"items": []any{map[string]any{
			"id": 601, "profilePostId": 501, "author": member(9, "Yuniel"), "state": "visible",
			"createdAt": "2026-10-31T12:00:00Z", "editedAt": nil, "bodyHtml": "<p>¡Gracias, Ana!</p>",
			"attachmentCount": 0, "attachments": []any{}, "reactions": reactions(), "canEdit": false,
			"canDelete": false, "canHardDelete": false, "canReport": false, "deletion": nil,
		}},
		"nextCursor": nil,
	})
	f.handle("POST /api/v1/users/9/profile-posts", 201, profilePost(503, member(7, "albert"), "<p>hola</p>", 0))
	f.handle("POST /api/v1/profile-posts/501/comments", 201, map[string]any{"id": 602})
	return f
}

func TestProfileShowsFactsAndPosts(t *testing.T) {
	f := profileServer(t)
	h := start(t, NewProfile(testDeps(t, f, false), 9))
	h.wantText("Yuniel", "Habanero", "321", "45", "Me gusta el béisbol.", "¡Feliz cumpleaños, Yuniel!", "Luis", "Nos vemos en el Malecón.")
	if n := len(f.find("GET", "/api/v1/profile-posts/501/comments")); n != 0 {
		t.Fatalf("comments must load only on enter, got %d requests", n)
	}
	h.key("enter")
	if n := len(f.find("GET", "/api/v1/profile-posts/501/comments")); n != 1 {
		t.Fatalf("enter must load the comments once, got %d requests", n)
	}
	h.wantText("¡Gracias, Ana!")
}

func TestProfileNotFound(t *testing.T) {
	f := newFakeSermo(t)
	f.handle("GET /api/v1/users/9", 404, errBody("not_found", "x"))
	f.handle("GET /api/v1/users/9/profile-posts", 404, errBody("not_found", "x"))
	h := start(t, NewProfile(testDeps(t, f, false), 9))
	h.wantText("No encontramos lo que buscas")
}

func TestProfileOwnForGuestGetsLogin(t *testing.T) {
	f := newFakeSermo(t)
	h := start(t, NewProfile(testDeps(t, f, false), 0))
	if _, ok := msgOf[ui.LoginMsg](h.msgs); !ok {
		t.Fatalf("guest must get LoginMsg, got %#v", h.msgs)
	}
}

func TestProfileGuestCannotWrite(t *testing.T) {
	f := profileServer(t)
	h := start(t, NewProfile(testDeps(t, f, false), 9))
	h.key("w")
	toast, ok := msgOf[ui.ToastMsg](h.msgs)
	if !ok || toast.Text != "Tienes que entrar para hacer eso." {
		t.Fatalf("guest must get a toast, got %#v", h.msgs)
	}
	if h.s.CapturesInput() {
		t.Fatal("a guest must not get the editor")
	}
}

func TestProfileWriteAndComment(t *testing.T) {
	f := profileServer(t)
	h := start(t, NewProfile(testDeps(t, f, true), 9))
	h.key("w")
	h.typeText("hola")
	h.key("ctrl+s")
	posts := f.find("POST", "/api/v1/users/9/profile-posts")
	if len(posts) != 1 || !jsonEq(t, posts[0].Body, `{"body":"hola"}`) {
		t.Fatalf("profile post requests = %#v", posts)
	}
	h.key("c")
	h.typeText("bien")
	h.key("ctrl+s")
	comments := f.find("POST", "/api/v1/profile-posts/501/comments")
	if len(comments) != 1 || !jsonEq(t, comments[0].Body, `{"body":"bien"}`) {
		t.Fatalf("comment requests = %#v", comments)
	}
}

func ownServer(t *testing.T) *fakeSermo {
	t.Helper()
	f := newFakeSermo(t)
	f.handle("GET /api/v1/users/7", 200, profileJSON(7, "albert"))
	f.handle("GET /api/v1/users/7/profile-posts", 200, map[string]any{"items": []any{}, "nextCursor": nil})
	f.handle("GET /api/v1/bookmarks", 200, map[string]any{"items": []any{map[string]any{
		"id": 1, "contentType": "post", "contentId": 55, "threadId": 10, "postPosition": 3,
		"title": "Elecciones", "excerpt": "Un mensaje guardado", "author": member(12, "Ana"),
		"createdAt": "2026-10-29T10:00:00Z",
	}}, "nextCursor": nil, "total": 1})
	f.handle("GET /api/v1/watches/threads", 200, map[string]any{"items": []any{map[string]any{
		"threadId": 33, "title": "Béisbol cubano", "nodeId": 4, "email": false,
		"createdAt": "2026-10-01T10:00:00Z", "lastPostAt": "2026-10-31T13:05:00Z",
	}}, "nextCursor": nil})
	f.handle("DELETE /api/v1/threads/33/watch", 200, map[string]any{})
	return f
}

func TestOwnProfileListsBookmarksAndWatches(t *testing.T) {
	f := ownServer(t)
	h := start(t, NewProfile(testDeps(t, f, true), 0))
	h.wantText("albert", "Marcadores", "Seguidos")
	if n := len(f.find("GET", "/api/v1/bookmarks")); n != 0 {
		t.Fatal("bookmarks must load only when the tab opens")
	}
	h.key("2")
	h.wantText("Elecciones", "Un mensaje guardado")
	h.key("enter")
	if open, ok := msgOf[ui.OpenThreadMsg](h.msgs); !ok || open != (ui.OpenThreadMsg{ThreadID: 10, PostID: 55}) {
		t.Fatalf("bookmark enter gave %#v", h.msgs)
	}
	h.key("3")
	h.wantText("Béisbol cubano")
	h.key("x")
	if n := len(f.find("DELETE", "/api/v1/threads/33/watch")); n != 1 {
		t.Fatalf("x sent %d unwatch requests, want 1", n)
	}
	if strings.Contains(h.screen(), "Béisbol cubano") {
		t.Fatalf("the unwatched thread must leave the list:\n%s", h.screen())
	}
}
