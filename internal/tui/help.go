package tui

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

var settingsKey = key.NewBinding(key.WithKeys("s", "S"))

// helpScreen shows the global keys and the keys of the screen under it.
type helpScreen struct {
	d          *ui.Deps
	screenKeys []key.Binding
	// commands are the BBS commands of the screen under the help.
	commands []ui.Command
}

func newHelp(d *ui.Deps, screenKeys []key.Binding, commands []ui.Command) *helpScreen {
	return &helpScreen{d: d, screenKeys: screenKeys, commands: commands}
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
	if h.d.BBS() {
		return h.bbsView(width)
	}
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

// Prompt is the label of the BBS prompt.
func (h *helpScreen) Prompt() string { return i18n.T("bbs.prompt.help") }

// Commands gives no own commands, so that the hint line shows the global
// commands.
func (h *helpScreen) Commands() []ui.Command {
	out := make([]ui.Command, len(bbsGlobals))
	for i, g := range bbsGlobals {
		out[i] = ui.Command{Name: g.name, Label: i18n.T(g.label)}
	}
	return out
}

// bbsView lists the commands of the screen and the global commands in an
// ASCII table.
func (h *helpScreen) bbsView(width int) string {
	nameW := ansi.StringWidth(i18n.T("bbs.help.command"))
	for _, c := range h.commands {
		nameW = max(nameW, ansi.StringWidth(commandName(c.Name)))
	}
	name := ui.Column{Title: i18n.T("bbs.help.command"), Width: max(nameW, 5)}
	action := ui.Column{Title: i18n.T("bbs.help.action")}
	t := ui.NewTable(width, name, action, name, action)
	t.Full(h.d.Styles.Subtitle.Render(h.Title())).Header()
	if rows := helpRows(h.commands); len(rows) > 0 {
		t.Section(i18n.T("bbs.help.screen"))
		addPairs(t, rows)
		t.Rule()
	}
	globals := make([][2]string, len(bbsGlobals))
	for i, g := range bbsGlobals {
		globals[i] = [2]string{g.name, i18n.T(g.label)}
	}
	t.Section(i18n.T("bbs.help.global"))
	addPairs(t, globals)
	t.Wrap(i18n.T("bbs.help.keys"))
	return t.Render(h.d.Styles)
}

// addPairs adds the commands to t, two commands in each row.
func addPairs(t *ui.Table, rows [][2]string) {
	for i := 0; i < len(rows); i += 2 {
		cells := rows[i][:]
		if i+1 < len(rows) {
			cells = append(cells, rows[i+1][:]...)
		}
		t.Row(cells...)
	}
}

// helpRows returns the name and the label of each command with a label.
func helpRows(commands []ui.Command) [][2]string {
	var out [][2]string
	for _, c := range commands {
		if c.Label != "" {
			out = append(out, [2]string{commandName(c.Name), c.Label})
		}
	}
	return out
}

// commandName is the name of a command as the user types it.
func commandName(name string) string {
	switch name {
	case "#":
		return i18n.T("bbs.help.number")
	case "":
		return i18n.T("bbs.help.enter")
	}
	return strings.ToUpper(name)
}
