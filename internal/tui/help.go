package tui

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

var settingsKey = key.NewBinding(key.WithKeys("s", "S"))

// helpScreen shows the global keys and the keys of the screen under it.
type helpScreen struct {
	d          *ui.Deps
	screenKeys []key.Binding
}

func newHelp(d *ui.Deps, screenKeys []key.Binding) *helpScreen {
	return &helpScreen{d: d, screenKeys: screenKeys}
}

func (h *helpScreen) Init() tea.Cmd       { return nil }
func (h *helpScreen) Title() string       { return i18n.T("core.help.title") }
func (h *helpScreen) CapturesInput() bool { return false }
func (h *helpScreen) Keys() []key.Binding {
	return []key.Binding{key.NewBinding(key.WithKeys("s", "S"), key.WithHelp("S", i18n.T("core.key.settings")))}
}

func (h *helpScreen) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && key.Matches(k, settingsKey) {
		return h, ui.Push(newSettings(h.d))
	}
	return h, nil
}

func (h *helpScreen) View(width, height int) string {
	s := h.d.Styles
	m := help.New()
	m.Styles.FullKey = s.Key
	m.Styles.FullDesc = s.Text
	m.Styles.FullSeparator = s.Muted
	column := func(title string, keys []key.Binding) string {
		return s.Subtitle.Render(title) + "\n\n" + m.FullHelpView([][]key.Binding{keys})
	}
	cols := []string{column(i18n.T("core.help.global"), append(globalBindings(), h.Keys()...))}
	if len(h.screenKeys) > 0 {
		cols = append(cols, "      ", column(i18n.T("core.help.screen"), h.screenKeys))
	}
	return ui.Header(s, h.d.Theme, h.Title(), width) + "\n\n" +
		lipgloss.NewStyle().PaddingLeft(2).Render(lipgloss.JoinHorizontal(lipgloss.Top, cols...))
}
