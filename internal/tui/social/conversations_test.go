package social

import (
	"net/http"
	"strings"
	"testing"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

func TestConversationsListRenders(t *testing.T) {
	f := newFakeSermo(t)
	f.handle("GET /api/v1/conversations", 200, map[string]any{
		"items":      []any{conversationSummary(5, "Viaje a Varadero", true), conversationSummary(6, "Libros", false)},
		"nextCursor": nil, "total": 2,
	})
	h := start(t, NewConversations(testDeps(t, f, true)))
	h.wantText("Viaje a Varadero", "Libros", "Ana", i18n.Plural("social.conv.participants", 3, map[string]any{"Starter": "Ana"}), "Pedro", "hace 2 h", "●")
	if q := f.find("GET", "/api/v1/conversations")[0].Query; q.Get("page") != "1" || q.Get("limit") != "20" {
		t.Fatalf("list query = %v", q)
	}
	h.key("enter")
	if open, ok := msgOf[ui.PushMsg](h.msgs); !ok || open.Screen == nil {
		t.Fatalf("enter must push the conversation, got %#v", h.msgs)
	}
}

func TestConversationsGuestGetsLogin(t *testing.T) {
	f := newFakeSermo(t)
	h := start(t, NewConversations(testDeps(t, f, false)))
	if _, ok := msgOf[ui.LoginMsg](h.msgs); !ok {
		t.Fatalf("guest must get LoginMsg, got %#v", h.msgs)
	}
}

func TestNewConversationSendsRecipientIDs(t *testing.T) {
	f := newFakeSermo(t)
	f.handle("GET /api/v1/conversations", 200, map[string]any{"items": []any{}, "nextCursor": nil, "total": 0})
	f.handle("GET /api/v1/users", 200, map[string]any{"items": []any{
		map[string]any{"id": 12, "username": "Ana", "staff": false},
		map[string]any{"id": 15, "username": "Andrés", "staff": false},
	}})
	f.handle("POST /api/v1/conversations", 201, map[string]any{
		"conversation": convJSON(90, "Hola"), "message": message(900, 90, member(7, "albert"), "<p>texto</p>"),
	})
	h := start(t, NewConversations(testDeps(t, f, true)))
	h.key("c")
	if !h.s.CapturesInput() {
		t.Fatal("the editor must capture input")
	}
	h.typeText("a")
	if n := len(f.find("GET", "/api/v1/users")); n != 0 {
		t.Fatalf("one letter must not search, got %d searches", n)
	}
	h.typeText("n")
	searches := f.find("GET", "/api/v1/users")
	if len(searches) != 1 || searches[0].Query.Get("prefix") != "an" {
		t.Fatalf("user searches = %#v", searches)
	}
	h.wantText("Ana", "Andrés")
	h.key("down", "enter")
	h.wantText("Andrés ×")
	h.key("tab")
	h.typeText("Hola")
	h.key("tab")
	h.typeText("texto")
	h.key("ctrl+s")
	posts := f.find("POST", "/api/v1/conversations")
	if len(posts) != 1 || !jsonEq(t, posts[0].Body, `{"recipientIds":[15],"title":"Hola","body":"texto"}`) {
		t.Fatalf("create requests = %#v", posts)
	}
	if open, ok := msgOf[ui.OpenConversationMsg](h.msgs); !ok || open.ID != 90 {
		t.Fatalf("send must open the new conversation, got %#v", h.msgs)
	}
}

func conversationServer(t *testing.T) *fakeSermo {
	t.Helper()
	f := newFakeSermo(t)
	f.handle("GET /api/v1/conversations/5", 200, convJSON(5, "Viaje a Varadero"))
	f.handle("GET /api/v1/conversations/5/unread", 200, map[string]any{"conversationId": 5, "messageId": 78, "cursor": nil, "page": 1})
	f.handle("GET /api/v1/conversations/5/messages", 200, map[string]any{
		"items": []any{
			message(77, 5, member(12, "Ana"), `<p>¿Vamos el sábado? Mira <a href="https://example.com/playa">esto</a></p>`),
			message(78, 5, member(7, "albert"), "<p>Claro que sí.</p>"),
		},
		"nextCursor": nil, "total": 2,
	})
	f.handle("PUT /api/v1/conversations/5/read", 200, map[string]any{})
	f.handle("GET /api/v1/conversation-messages/77/quote", 200, map[string]any{
		"messageId": 77, "author": member(12, "Ana"), "bodySource": "¿Vamos el sábado?",
	})
	f.handle("POST /api/v1/conversations/5/messages", 201, message(79, 5, member(7, "albert"), "<p>hola</p>"))
	return f
}

func TestConversationShowsMessagesAndMarksRead(t *testing.T) {
	f := conversationServer(t)
	h := start(t, NewConversation(testDeps(t, f, true), 5))
	h.wantText("Viaje a Varadero", "Ana, albert", "¿Vamos el sábado?", "esto [1]", "Claro que sí.")
	if n := len(f.find("PUT", "/api/v1/conversations/5/read")); n != 1 {
		t.Fatalf("read sent %d times, want 1", n)
	}
	if q := f.find("GET", "/api/v1/conversations/5/messages")[0].Query; q.Get("page") != "1" {
		t.Fatalf("messages query = %v, want the page of the first unread", q)
	}
}

func TestConversationReplySendsMessage(t *testing.T) {
	f := conversationServer(t)
	h := start(t, NewConversation(testDeps(t, f, true), 5))
	h.key("r")
	h.typeText("hola")
	h.key("ctrl+s")
	posts := f.find("POST", "/api/v1/conversations/5/messages")
	if len(posts) != 1 || !jsonEq(t, posts[0].Body, `{"body":"hola"}`) {
		t.Fatalf("reply requests = %#v", posts)
	}
	if h.s.CapturesInput() {
		t.Fatal("the editor must close after the send")
	}
	if n := len(f.find("GET", "/api/v1/conversations/5/messages")); n < 2 {
		t.Fatal("the messages must load again after the send")
	}
}

func TestConversationQuoteUsesMessageReference(t *testing.T) {
	f := conversationServer(t)
	h := start(t, NewConversation(testDeps(t, f, true), 5))
	h.key("g", "c")
	h.key("ctrl+s")
	posts := f.find("POST", "/api/v1/conversations/5/messages")
	want := `{"body":"[QUOTE=\"Ana, convMessage: 77, member: 12\"]¿Vamos el sábado?[/QUOTE]\n"}`
	if len(posts) != 1 || !jsonEq(t, posts[0].Body, want) {
		t.Fatalf("reply requests = %#v", posts)
	}
}

func TestConversationEditorWrapsAndPreviews(t *testing.T) {
	f := conversationServer(t)
	f.mux.HandleFunc("POST /api/v1/posts/preview", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"html": "<p><b>negrita</b> vista previa</p>"})
	})
	h := start(t, NewConversation(testDeps(t, f, true), 5))
	h.key("r", "ctrl+b")
	h.typeText("negrita")
	h.wantText("[B]negrita[/B]")
	h.key("ctrl+p")
	previews := f.find("POST", "/api/v1/posts/preview")
	want := `{"body":"[B]negrita[/B]","target":{"type":"message","conversationId":5}}`
	if len(previews) != 1 || !jsonEq(t, previews[0].Body, want) {
		t.Fatalf("preview requests = %#v", previews)
	}
	h.wantText("vista previa")
	h.key("esc")
	if !strings.Contains(h.screen(), "[B]negrita[/B]") {
		t.Fatalf("esc must close the preview and keep the text:\n%s", h.screen())
	}
	h.key("esc")
	if h.s.CapturesInput() {
		t.Fatal("esc must close the editor")
	}
}

func TestConversationNotFound(t *testing.T) {
	f := newFakeSermo(t)
	f.handle("GET /api/v1/conversations/5", 404, errBody("not_found", "x"))
	f.handle("GET /api/v1/conversations/5/unread", 404, errBody("not_found", "x"))
	h := start(t, NewConversation(testDeps(t, f, true), 5))
	h.wantText("No encontramos lo que buscas")
}
