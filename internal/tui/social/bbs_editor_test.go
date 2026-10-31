package social

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/api"
)

// replyEditor opens the line editor of a conversation reply and types
// the lines.
func replyEditor(t *testing.T, lines ...string) (*harness, *lineEditor) {
	t.Helper()
	h := startBBS(t, NewConversation(bbsDeps(t, conversationServer(t), true), 5))
	h.command("R", "")
	for _, l := range lines {
		h.typeText(l)
		h.key("enter")
	}
	return h, h.s.(*conversation).led
}

func TestLineEditorInsertKeepsLineNumbers(t *testing.T) {
	h, e := replyEditor(t, "a", "b", "c", "/i 2")
	h.wantText("|   1 | a", "|   > | _", "|   2 | b", "|   3 | c")
	if strings.Contains(h.screen(), "|   4 |") {
		t.Fatalf("an insert must not change the numbers of the lines:\n%s", h.screen())
	}
	h.typeText("/d 3")
	h.key("enter")
	if got := strings.Join(e.lines, "|"); got != "a|b" {
		t.Fatalf("/D 3 must delete the line shown as 3, lines = %q", got)
	}
	for _, l := range []string{"/i 1", "/l 2"} {
		h.typeText(l)
		h.key("enter")
	}
	h.wantText("|   2 | b")
	if strings.Contains(h.screen(), "|   1 | a") {
		t.Fatalf("/L 2 must list from the line shown as 2:\n%s", h.screen())
	}
}

func TestLineEditorEditThenWrapStaysInPlace(t *testing.T) {
	h, e := replyEditor(t, "uno", "dos", "tres", "/e 1")
	h.key("backspace", "backspace", "backspace")
	h.typeText(strings.Repeat("palabra ", 12))
	h.key("enter")
	if len(e.lines) != 4 || e.lines[2] != "dos" || e.lines[3] != "tres" || !strings.HasPrefix(e.lines[1], "palabra") {
		t.Fatalf("the wrapped text must stay after line 1, lines = %q", e.lines)
	}
}

func TestLineEditorEditThenPasteStaysInPlace(t *testing.T) {
	h, e := replyEditor(t, "uno", "dos", "tres", "/e 2")
	h.send(tea.PasteMsg{Content: "X\nY"})
	h.key("enter")
	if got := strings.Join(e.lines, "|"); got != "uno|dosX|Y|tres" {
		t.Fatalf("the pasted text must stay after line 2, lines = %q", got)
	}
}

func TestLineEditorKeepsInsertedLines(t *testing.T) {
	link := "https://example.com/" + strings.Repeat("a", 70)
	name := strings.Repeat("n", 30)
	quote := `[QUOTE="` + name + `, convMessage: 123456, member: 654321"]` + "texto [URL]" + link + "[/URL][/QUOTE]"
	start := link + "\n" + quote
	e := newLineEditor(bbsDeps(t, newFakeSermo(t), true), "H", api.MessageTarget(0), start, nil)
	e.setSize(bbsW, bbsH)
	e.addText(quote)
	want := []string{link, quote, quote}
	if strings.Join(e.lines, "\n") != strings.Join(want, "\n") {
		t.Fatalf("the editor must keep the inserted lines, lines = %q", e.lines)
	}
	e.typeText(strings.Repeat("palabra ", 12))
	if len(e.lines) != 4 || len([]rune(e.lines[3])) > e.cols() {
		t.Fatalf("the typed text must wrap, lines = %q", e.lines)
	}
}
