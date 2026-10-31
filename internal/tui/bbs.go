package tui

import (
	"strings"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// bbsGlobal is one global command of the BBS prompt. The label is an i18n ID.
type bbsGlobal struct {
	name, label string
}

// bbsGlobals are the global commands in the order of the hint line and the
// help table. The hints that are used most come first, because the hint
// line leaves out the hints that do not fit.
var bbsGlobals = []bbsGlobal{
	{"F", "bbs.global.forums"},
	{"M", "bbs.global.messages"},
	{"N", "bbs.global.notifications"},
	{"B", "bbs.global.search"},
	{"P", "bbs.global.profile"},
	{"?", "bbs.global.help"},
	{"Q", "bbs.global.quit"},
	{"S", "bbs.global.settings"},
	{"V", "bbs.global.back"},
}

// parseCommand splits the prompt text into a command name and an
// argument. Digits give the name "#" with the digits as the argument.
// Empty text gives the name "". Else the first word in upper case is the
// name and the rest is the argument.
func parseCommand(text string) (string, string) {
	text = strings.TrimSpace(text)
	if text != "" && strings.IndexFunc(text, func(r rune) bool { return !unicode.IsDigit(r) }) < 0 {
		return "#", text
	}
	name, arg, _ := strings.Cut(text, " ")
	return strings.ToUpper(name), strings.TrimSpace(arg)
}

// bbsKey handles a key in the BBS interface. Printable keys go to the
// prompt. Enter runs the prompt. Esc clears the prompt, or goes back when
// the prompt is empty. Other keys, for example the arrows, go to the screen.
func (m *model) bbsKey(msg tea.KeyPressMsg) tea.Cmd {
	if m.top().CapturesInput() {
		return m.forward(msg)
	}
	switch msg.String() {
	case "enter":
		text := m.input
		m.input = ""
		return m.runCommand(text)
	case "esc":
		if m.input != "" {
			m.input = ""
			return nil
		}
		return m.pop()
	case "backspace":
		_, size := utf8.DecodeLastRuneInString(m.input)
		m.input = m.input[:len(m.input)-size]
		return nil
	}
	if msg.Text != "" && msg.Mod&^tea.ModShift == 0 {
		m.input += msg.Text
		return nil
	}
	return m.forward(msg)
}

// runCommand runs the prompt text. A command of the screen comes first.
// A command without Run only shows a hint, so its name goes on to the
// global commands. Then a bare Enter goes to the screen. A screen without
// commands gets a single typed key, so that its own keys still work.
func (m *model) runCommand(text string) tea.Cmd {
	name, arg := parseCommand(text)
	if c, ok := m.screenCommand(name); ok {
		return c.Run(arg)
	}
	if name == "" {
		return m.forward(tea.KeyPressMsg{Code: tea.KeyEnter})
	}
	if cmd, ok := m.globalCommand(name); ok {
		return cmd
	}
	if _, isCommander := m.top().(ui.Commander); !isCommander && utf8.RuneCountInString(text) == 1 {
		r, _ := utf8.DecodeRuneInString(text)
		return m.forward(tea.KeyPressMsg{Code: unicode.ToLower(r), Text: text})
	}
	return ui.Toast(i18n.T("bbs.unknown"), ui.ToastError)
}

func (m *model) screenCommand(name string) (ui.Command, bool) {
	c, ok := m.top().(ui.Commander)
	if !ok {
		return ui.Command{}, false
	}
	for _, cmd := range c.Commands() {
		if cmd.Run != nil && strings.EqualFold(cmd.Name, name) {
			return cmd, true
		}
	}
	return ui.Command{}, false
}

// globalCommand runs a global command of the BBS prompt.
func (m *model) globalCommand(name string) (tea.Cmd, bool) {
	switch name {
	case "F", "M", "N", "B", "P", "?", "Q":
		return m.hotkey(strings.ToLower(name)), true
	case "V":
		return m.pop(), true
	case "S":
		if _, ok := m.top().(*settings); ok {
			return nil, true
		}
		return m.push(newSettings(m.d)), true
	}
	return nil, false
}

// paste adds pasted text to the prompt. It returns false when the screen
// takes the text.
func (m *model) paste(msg tea.PasteMsg) bool {
	if !m.d.BBS() || m.splash != nil || m.top().CapturesInput() {
		return false
	}
	m.input += strings.ReplaceAll(msg.Content, "\n", " ")
	return true
}

// screenCommands returns the commands of the top screen, or nil.
func (m *model) screenCommands() []ui.Command {
	if c, ok := m.top().(ui.Commander); ok {
		return c.Commands()
	}
	return nil
}

// syncMode reacts to a change of the interface in the settings: the
// content area changes size, the prompt clears and the BBS data loads.
func (m *model) syncMode() tea.Cmd {
	if m.d.BBS() == m.bbsOn {
		return nil
	}
	m.bbsOn = m.d.BBS()
	m.input = ""
	return tea.Batch(m.resizeAll(), m.bbsData())
}
