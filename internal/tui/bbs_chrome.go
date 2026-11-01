package tui

import (
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// bbsChromeHeight is the number of lines of the BBS chrome: the status
// box of three lines, the hint line and the prompt line.
const bbsChromeHeight = 5

// renderBBSScreen draws the BBS splash while it shows, else the top screen
// in the BBS chrome.
func (m *model) renderBBSScreen() string {
	if m.splash != nil {
		return m.bbsSplash()
	}
	return m.renderBBS()
}

// renderBBS draws the BBS chrome around the top screen. Each line is
// exactly the window width.
func (m *model) renderBBS() string {
	cw, ch := m.contentSize()
	lines := strings.Split(ui.Box(m.d.Styles, m.w, []string{m.statusLine(m.w - 4)}), "\n")
	content := strings.Split(m.top().View(cw, ch), "\n")
	for i := range ch {
		l := ""
		if i < len(content) {
			l = content[i]
		}
		lines = append(lines, fit(l, m.w))
	}
	if m.toast != nil {
		lines[len(lines)-1] = fit(m.bbsToast(*m.toast), m.w)
	}
	hints := m.hintLine()
	if m.confirmQuit {
		hints = " " + m.d.Styles.Warning.Render(i18n.T("core.quit.confirm"))
	}
	return strings.Join(append(lines, fit(hints, m.w), fit(m.promptLine(), m.w)), "\n")
}

// fit cuts a line to width cells and fills it with spaces to width cells.
func fit(line string, width int) string {
	line = ansi.Truncate(line, width, "")
	if line != "" {
		line += "\x1b[m"
	}
	return line + strings.Repeat(" ", max(width-ansi.StringWidth(line), 0))
}

// statusLine is the line of the status box: the site, the visitor count,
// the member with the group and the clock. The parts spread over width
// cells.
func (m *model) statusLine(width int) string {
	s := m.d.Styles
	parts := []string{s.Subtitle.Render(i18n.Tf("bbs.status.site", map[string]any{"Site": strings.ToUpper(m.site)}))}
	if m.haveOnline {
		parts = append(parts, s.Text.Render(i18n.Tf("bbs.status.online", map[string]any{"Count": ui.Thousands(m.online)})))
	}
	member := s.Muted.Render(i18n.T("bbs.status.guest"))
	if m.d.Me != nil {
		member = s.Text.Render(m.d.Me.Username)
		if m.group != "" {
			member += " " + s.Muted.Render("[") + s.Accent.Render(m.group) + s.Muted.Render("]")
		}
	}
	parts = append(parts, member, s.Accent.Render(bbsClock(m.d.Now())))
	return spread(parts, width)
}

// spread puts the parts on one line of width cells with equal gaps. The
// gaps are two cells or more. The line is cut when it is too long.
func spread(parts []string, width int) string {
	used := 0
	for _, p := range parts {
		used += ansi.StringWidth(p)
	}
	gaps := len(parts) - 1
	if gaps == 0 {
		return parts[0]
	}
	free := max(width-used, 2*gaps)
	var b strings.Builder
	for i, p := range parts {
		if i > 0 {
			n := free / gaps
			if i <= free%gaps {
				n++
			}
			b.WriteString(strings.Repeat(" ", n))
		}
		b.WriteString(p)
	}
	return ansi.Truncate(b.String(), width, "")
}

// bbsClock formats t like "Sáb 31-Oct  18:26" with the names of the
// current language.
func bbsClock(t time.Time) string {
	days := strings.Split(i18n.T("bbs.days"), ",")
	months := strings.Split(i18n.T("bbs.months"), ",")
	day, month := t.Weekday().String()[:3], t.Month().String()[:3]
	if len(days) == 7 {
		day = days[t.Weekday()]
	}
	if len(months) == 12 {
		month = months[t.Month()-1]
	}
	return day + " " + strconv.Itoa(t.Day()) + "-" + month + "  " + t.Format("15:04")
}

// hintLine is the line of command hints. A screen with commands gives its
// hints. Other screens get the global hints. Hints for M and N show the
// unread counts. A hint that does not fit in the width is left out with
// the hints after it.
func (m *model) hintLine() string {
	line, used := "", 0
	for i, h := range m.hints() {
		w := 2 + ansi.StringWidth(h.label)
		if used+w > m.w {
			break
		}
		line += strings.Repeat(" ", min(used, 1)+1) + m.markHint(i, h.label)
		used += w
	}
	return line
}

// hint is one hint of the hint line: the name of its command and the label.
type hint struct {
	name, label string
}

// hints returns the hints of the top screen, or the global hints.
func (m *model) hints() []hint {
	var out []hint
	c, ok := m.top().(ui.Commander)
	if !ok {
		for _, g := range bbsGlobals {
			out = append(out, hint{g.name, m.withCount(g.name, i18n.T(g.label))})
		}
		return out
	}
	for _, cmd := range c.Commands() {
		switch {
		case cmd.Label == "":
		case cmd.NoCount:
			out = append(out, hint{cmd.Name, cmd.Label})
		default:
			out = append(out, hint{cmd.Name, m.withCount(cmd.Name, cmd.Label)})
		}
	}
	return out
}

// withCount adds the unread count to the hint of the global M or N.
func (m *model) withCount(name, label string) string {
	n := 0
	switch strings.ToUpper(name) {
	case "M":
		n = m.counters.Conversations
	case "N":
		n = m.counters.Notifications
	}
	if m.d.Me == nil || n == 0 {
		return label
	}
	return label + "(" + strconv.Itoa(n) + ")"
}

// paintHint gives the text in brackets the Accent style, the brackets the
// Muted style and the rest the Text style.
func (m *model) paintHint(label string) string {
	s := m.d.Styles
	var b strings.Builder
	for label != "" {
		open := strings.Index(label, "[")
		end := strings.Index(label, "]")
		if open < 0 || end < open {
			b.WriteString(s.Text.Render(label))
			break
		}
		if open > 0 {
			b.WriteString(s.Text.Render(label[:open]))
		}
		b.WriteString(s.Muted.Render("[") + s.Key.Render(label[open+1:end]) + s.Muted.Render("]"))
		label = label[end+1:]
	}
	return b.String()
}

// promptLine is the prompt: the label of the screen, ">" and the typed
// text with a cursor. While the screen takes the keys, the prompt shows no
// cursor.
func (m *model) promptLine() string {
	s := m.d.Styles
	label := i18n.T("bbs.prompt.command")
	if c, ok := m.top().(ui.Commander); ok && c.Prompt() != "" {
		label = c.Prompt()
	}
	if m.top().CapturesInput() {
		return " " + s.Muted.Render(label+" >")
	}
	return " " + s.Text.Render(label) + s.Accent.Render(" > ") + s.Text.Render(m.input) + s.Accent.Render("_")
}

// bbsToast is the toast line of the BBS interface: ">> text" in the color
// of its kind.
func (m *model) bbsToast(t ui.ToastMsg) string {
	s := m.d.Styles
	st := s.Accent
	switch t.Kind {
	case ui.ToastSuccess:
		st = s.Success
	case ui.ToastError:
		st = s.Error
	}
	return " " + st.Render(">> "+strings.ReplaceAll(t.Text, "\n", " "))
}
