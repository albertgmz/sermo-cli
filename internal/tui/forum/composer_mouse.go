package forum

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/i18n"
)

// hintSep is the mark between the parts of the composer hint.
const hintSep = " · "

// markHints puts each part of the composer hint in a zone.
func (c *composer) markHints(hint string) string {
	parts := strings.Split(hint, hintSep)
	for i, p := range parts {
		parts[i] = c.d.Mark(c.zid+"hint."+strconv.Itoa(i), p)
	}
	return strings.Join(parts, hintSep)
}

// clickHint runs the key of the clicked part of the hint. The first word
// of each part is its key, for example "Ctrl+S".
func (c *composer) clickHint(msg tea.MouseReleaseMsg) tea.Cmd {
	if c.sending || c.previewing {
		return nil
	}
	for i, p := range strings.Split(i18n.T("forum.compose.hint"), hintSep) {
		if !c.d.Clicked(c.zid+"hint."+strconv.Itoa(i), msg) {
			continue
		}
		if k, ok := hintKey(p); ok {
			return c.key(k)
		}
	}
	return nil
}

// hintKey returns the key of a hint part: Ctrl and a letter, Tab or Esc.
func hintKey(part string) (tea.KeyPressMsg, bool) {
	words := strings.Fields(strings.ToLower(part))
	if len(words) == 0 {
		return tea.KeyPressMsg{}, false
	}
	switch k := words[0]; {
	case k == "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}, true
	case k == "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}, true
	case strings.HasPrefix(k, "ctrl+") && len(k) == len("ctrl+")+1:
		return tea.KeyPressMsg{Code: rune(k[len(k)-1]), Mod: tea.ModCtrl}, true
	}
	return tea.KeyPressMsg{}, false
}
