package social

import (
	"net/http"
	"strings"
	"testing"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

func TestConversationsBBSTable(t *testing.T) {
	f := newFakeSermo(t)
	f.handle("GET /api/v1/conversations", 200, map[string]any{
		"items":      []any{conversationSummary(5, "Viaje a Varadero", true), conversationSummary(6, "Libros", false)},
		"nextCursor": nil, "total": 2,
	})
	h := startBBS(t, NewConversations(bbsDeps(t, f, true)))
	h.wantText("| ## | (*) | Asunto", i18n.T("bbs.social.conv.who"), "Último", "| 01 | (*) | Viaje a Varadero", "| 02 | ( ) | Libros",
		"Ana +2", "hace 2h Pedro", "MENSAJES PRIVADOS", "Página 1 / 1")
	h.wantFit()
	h.wantHints("Mensajes", "[##] Leer", "[C] Nueva", "[<] [>] Página")
	h.capture("conversations")
	if q := f.find("GET", "/api/v1/conversations")[0].Query; q.Get("limit") != "11" {
		t.Fatalf("list query = %v, want the rows that fit: limit 11", q)
	}
	h.command("#", "2")
	push, ok := msgOf[ui.PushMsg](h.msgs)
	if !ok {
		t.Fatalf("2 must push the conversation, got %#v", h.msgs)
	}
	if c, isConv := push.Screen.(*conversation); !isConv || c.id != 6 {
		t.Fatalf("pushed %#v, want the conversation 6", push.Screen)
	}
}

func TestConversationsBBSNewSendsResolvedRecipients(t *testing.T) {
	f := newFakeSermo(t)
	f.handle("GET /api/v1/conversations", 200, map[string]any{"items": []any{}, "nextCursor": nil, "total": 0})
	f.mux.HandleFunc("GET /api/v1/users", func(w http.ResponseWriter, r *http.Request) {
		users := map[string][]any{
			"ana":  {map[string]any{"id": 15, "username": "Anabel", "staff": false}, map[string]any{"id": 12, "username": "Ana", "staff": false}},
			"luis": {map[string]any{"id": 13, "username": "Luis", "staff": false}},
		}
		writeJSON(w, 200, map[string]any{"items": users[strings.ToLower(r.URL.Query().Get("prefix"))]})
	})
	f.handle("POST /api/v1/conversations", 201, map[string]any{
		"conversation": convJSON(90, "Hola"), "message": message(900, 90, member(7, "albert"), "<p>texto</p>"),
	})
	h := startBBS(t, NewConversations(bbsDeps(t, f, true)))
	h.command("C", "")
	if !h.s.CapturesInput() {
		t.Fatal("the line editor must capture input")
	}
	h.wantText("Para:")
	h.typeText("ana, Luis")
	h.key("enter")
	h.wantText("Asunto:")
	h.typeText("Hola")
	h.key("enter")
	h.typeText("texto")
	h.key("enter")
	h.capture("conversation-new")
	h.typeText("/s")
	h.key("enter")
	posts := f.find("POST", "/api/v1/conversations")
	if len(posts) != 1 || !jsonEq(t, posts[0].Body, `{"title":"Hola","body":"texto","recipientIds":[12,13]}`) {
		t.Fatalf("create requests = %#v", posts)
	}
}

func TestConversationsBBSUnknownRecipient(t *testing.T) {
	f := newFakeSermo(t)
	f.handle("GET /api/v1/conversations", 200, map[string]any{"items": []any{}, "nextCursor": nil, "total": 0})
	f.handle("GET /api/v1/users", 200, map[string]any{"items": []any{}})
	h := startBBS(t, NewConversations(bbsDeps(t, f, true)))
	h.command("C", "")
	for _, l := range []string{"nadie", "Hola", "texto", "/S"} {
		h.typeText(l)
		h.key("enter")
	}
	if n := len(f.find("POST", "/api/v1/conversations")); n != 0 {
		t.Fatalf("an unknown recipient must stop the send, got %d posts", n)
	}
	toast, ok := msgOf[ui.ToastMsg](h.msgs)
	if !ok || !strings.Contains(toast.Text, "nadie") {
		t.Fatalf("want a toast that names the user, got %#v", h.msgs)
	}
	if !h.s.CapturesInput() {
		t.Fatal("the editor must stay open with the text")
	}
}

func TestConversationBBSReader(t *testing.T) {
	f := conversationServer(t)
	h := startBBS(t, NewConversation(bbsDeps(t, f, true), 5))
	h.wantHints("Leyendo", "Mensajes 2-2 de 2")
	h.wantText("| #2   albert", "Claro que sí.")
	h.command("#", "1")
	h.wantText("Mensajes > Viaje a Varadero", "Ana, albert", "| #1   Ana", "30-Oct-2026 10:00", "¿Vamos el sábado?",
		"[C]itar  [R]esponder", "| #2   albert", "Claro que sí.", "+====")
	h.wantFit()
	h.wantHints("Leyendo", "Mensajes 1-2 de 2", "[ENTER] Sig.", "[P]revio", "[R]esponder", "[V]olver")
	h.capture("conversation")
	if strings.ContainsAny(h.screen(), "─│┌") {
		t.Fatalf("the BBS reader must use ASCII frames:\n%s", h.screen())
	}
	if n := len(f.find("PUT", "/api/v1/conversations/5/read")); n != 1 {
		t.Fatalf("read sent %d times, want 1", n)
	}
}

func TestConversationBBSNextAndPrevious(t *testing.T) {
	f := newFakeSermo(t)
	f.handle("GET /api/v1/conversations/5", 200, convJSON(5, "Viaje a Varadero"))
	f.handle("GET /api/v1/conversations/5/unread", 200, map[string]any{"conversationId": 5, "messageId": 70, "cursor": nil, "page": 1})
	long := strings.Repeat("<p>línea</p>", 8)
	f.handle("GET /api/v1/conversations/5/messages", 200, map[string]any{
		"items": []any{message(70, 5, member(12, "Ana"), long), message(71, 5, member(13, "Luis"), long),
			message(72, 5, member(14, "Pedro"), "<p>fin</p>")},
		"nextCursor": nil, "total": 3,
	})
	f.handle("PUT /api/v1/conversations/5/read", 200, map[string]any{})
	h := startBBS(t, NewConversation(bbsDeps(t, f, true), 5))
	h.wantHints("Leyendo", "Mensajes 1-1 de 3")
	h.command("", "")
	h.wantText("| #2   Luis")
	h.wantHints("Leyendo", "Mensajes 2-")
	h.command("P", "")
	h.wantText("| #1   Ana")
	h.command("#", "3")
	h.wantText("| #3   Pedro")
}

func TestConversationBBSReplyWithLineEditor(t *testing.T) {
	f := conversationServer(t)
	h := startBBS(t, NewConversation(bbsDeps(t, f, true), 5))
	h.command("R", "")
	if !h.s.CapturesInput() {
		t.Fatal("the line editor must capture input")
	}
	h.wantText("Editor de líneas", "/S enviar", "/A abortar", "Línea 1/200", "Col 1/")
	for _, l := range []string{"uno", "dos", "tres", "/d 2", "/e 1"} {
		h.typeText(l)
		h.key("enter")
	}
	h.wantText("|   1 | uno_")
	h.key("backspace", "backspace", "backspace")
	for _, l := range []string{"UNO", "/i 2", "medio", "/i"} {
		h.typeText(l)
		h.key("enter")
	}
	h.wantText("|   1 | UNO", "|   2 | medio", "|   3 | tres")
	h.wantFit()
	h.capture("editor")
	h.typeText("/S")
	h.key("enter")
	posts := f.find("POST", "/api/v1/conversations/5/messages")
	if len(posts) != 1 || !jsonEq(t, posts[0].Body, `{"body":"UNO\nmedio\ntres"}`) {
		t.Fatalf("reply requests = %#v", posts)
	}
	if h.s.CapturesInput() {
		t.Fatal("the editor must close after the send")
	}
}

func TestLineEditorWrapsAtTheColumnLimit(t *testing.T) {
	f := conversationServer(t)
	h := startBBS(t, NewConversation(bbsDeps(t, f, true), 5))
	h.command("R", "")
	h.typeText(strings.Repeat("palabra ", 12))
	h.key("enter")
	h.typeText("/s")
	h.key("enter")
	posts := f.find("POST", "/api/v1/conversations/5/messages")
	if len(posts) != 1 || !strings.Contains(posts[0].Body, `palabra\npalabra`) {
		t.Fatalf("a long line must wrap at a space, got %#v", posts)
	}
}

func TestLineEditorQuotePreviewAndAbort(t *testing.T) {
	f := conversationServer(t)
	f.mux.HandleFunc("POST /api/v1/posts/preview", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]any{"html": "<p>vista previa</p>"})
	})
	h := startBBS(t, NewConversation(bbsDeps(t, f, true), 5))
	h.command("#", "1")
	h.command("R", "")
	h.typeText("/C")
	h.key("enter")
	h.wantText(`[QUOTE="Ana, convMessage: 77, member: 12"]¿Vamos el sábado?[/QUOTE]`)
	h.typeText("/v")
	h.key("enter")
	previews := f.find("POST", "/api/v1/posts/preview")
	if len(previews) != 1 || !strings.Contains(previews[0].Body, "QUOTE") {
		t.Fatalf("preview requests = %#v", previews)
	}
	h.wantText("vista previa")
	h.key("enter")
	h.typeText("/?")
	h.key("enter")
	h.wantText("/E n", "/D n", "/I n", "/L")
	h.key("esc")
	h.typeText("/a")
	h.key("enter")
	if h.s.CapturesInput() {
		t.Fatal("/A must close the editor")
	}
	if n := len(f.find("POST", "/api/v1/conversations/5/messages")); n != 0 {
		t.Fatalf("/A must not send, got %d posts", n)
	}
}
