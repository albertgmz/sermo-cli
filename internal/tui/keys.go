package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/i18n"
)

// hotkey is one global key of the hotkey bar. The key letter is the same
// in all languages. The label is an i18n ID.
type hotkey struct {
	key, label string
}

var hotkeys = []hotkey{
	{"f", "core.hotkey.forums"},
	{"m", "core.hotkey.messages"},
	{"n", "core.hotkey.notifications"},
	{"b", "core.hotkey.search"},
	{"p", "core.hotkey.profile"},
	{"?", "core.hotkey.help"},
	{"q", "core.hotkey.quit"},
}

// globalKey returns the hotkey letter of msg. Uppercase letters work too.
func globalKey(msg tea.KeyPressMsg) (string, bool) {
	k := strings.ToLower(strings.TrimPrefix(msg.String(), "shift+"))
	for _, h := range hotkeys {
		if h.key == k {
			return k, true
		}
	}
	return "", false
}

func isBack(msg tea.KeyPressMsg) bool {
	s := msg.String()
	return s == "esc" || s == "backspace"
}

// globalBindings returns the global keys with help text in the current
// language.
func globalBindings() []key.Binding {
	out := make([]key.Binding, 0, len(hotkeys)+2)
	for _, h := range hotkeys {
		up := strings.ToUpper(h.key)
		out = append(out, key.NewBinding(key.WithKeys(h.key, up), key.WithHelp(up, i18n.T(h.label))))
	}
	return append(out,
		key.NewBinding(key.WithKeys("esc", "backspace"), key.WithHelp("Esc", i18n.T("core.key.back"))),
		key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("Ctrl+C", i18n.T("core.key.forceQuit"))),
	)
}
