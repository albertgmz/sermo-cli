package social

import (
	"testing"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

func TestSearchBBSQueryFromThePrompt(t *testing.T) {
	f := searchServer(t)
	h := startBBS(t, NewSearch(bbsDeps(t, f, false)))
	h.wantHints("Buscar", "[B] texto Buscar", "[T] Solo títulos", "[U] nombre Miembros")
	h.wantText("Escribe B")
	if h.s.CapturesInput() {
		t.Fatal("the BBS search must leave the keys to the prompt")
	}
	h.command("B", "")
	if n := len(f.find("GET", "/api/v1/search")); n != 0 {
		t.Fatalf("B without text must not search, got %d requests", n)
	}
	h.command("T", "")
	h.command("B", "cuba libre")
	calls := f.find("GET", "/api/v1/search")
	if len(calls) != 1 || calls[0].Query.Get("q") != "cuba libre" || calls[0].Query.Get("titlesOnly") != "true" {
		t.Fatalf("search requests = %#v", calls)
	}
	if l := calls[0].Query.Get("limit"); l == "20" || l == "" {
		t.Fatalf("the search must ask for the rows that fit, limit %q", l)
	}
	h.wantText("| ## | Tema", "Foro", i18n.T("bbs.social.profile.author"), "Fecha", "| 01 | Soberanía y MiIdentidad", "Informática y",
		"Nomura", "hace 1h", "Funciona desde afuera de Cuba & más", "«cuba libre»", "[x]")
	h.wantFit()
	h.capture("search")
	wantOpenAndRetoggle(t, h, f)
}

// wantOpenAndRetoggle checks that 1 opens the result and that T searches
// again without the titles only option.
func wantOpenAndRetoggle(t *testing.T, h *harness, f *fakeSermo) {
	t.Helper()
	h.command("#", "1")
	if open, ok := msgOf[ui.OpenThreadMsg](h.msgs); !ok || open != (ui.OpenThreadMsg{ThreadID: 34941, PostID: 478487}) {
		t.Fatalf("1 gave %#v", h.msgs)
	}
	h.command("T", "")
	if calls := f.find("GET", "/api/v1/search"); len(calls) != 2 || calls[1].Query.Get("titlesOnly") != "" {
		t.Fatalf("T must search again without titles only, got %#v", calls)
	}
}

func TestSearchBBSMembers(t *testing.T) {
	f := searchServer(t)
	h := startBBS(t, NewSearch(bbsDeps(t, f, false)))
	h.command("U", "an")
	calls := f.find("GET", "/api/v1/users")
	if len(calls) != 1 || calls[0].Query.Get("prefix") != "an" {
		t.Fatalf("user searches = %#v", calls)
	}
	h.wantText("| ## | Usuario", "| 01 | Ana")
	h.wantFit()
	h.capture("search-members")
	h.command("#", "1")
	if open, ok := msgOf[ui.OpenProfileMsg](h.msgs); !ok || open.UserID != 12 {
		t.Fatalf("1 gave %#v", h.msgs)
	}
}
