package ui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/theme"
)

// List is a cursor over a list of selectable rows. The screen keeps the
// items and draws each row. The zero value is an empty list.
type List struct {
	n, cursor, offset, height int
}

// The keys of a List.
var (
	listUp     = key.NewBinding(key.WithKeys("up", "k"))
	listDown   = key.NewBinding(key.WithKeys("down", "j"))
	listPgUp   = key.NewBinding(key.WithKeys("pgup"))
	listPgDown = key.NewBinding(key.WithKeys("pgdown"))
	listTop    = key.NewBinding(key.WithKeys("home", "g"))
	listBottom = key.NewBinding(key.WithKeys("end", "G"))
	listChoose = key.NewBinding(key.WithKeys("enter"))
)

// ListKeys returns the keys of a List with help text in the current
// language.
func ListKeys() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("up", "k", "down", "j"), key.WithHelp("↑↓ j k", i18n.T("core.key.move"))),
		key.NewBinding(key.WithKeys("pgup", "pgdown"), key.WithHelp("PgUp PgDn", i18n.T("core.key.page"))),
		key.NewBinding(key.WithKeys("home", "end", "g", "G"), key.WithHelp("g G", i18n.T("core.key.ends"))),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", i18n.T("core.key.open"))),
	}
}

// SetLen sets the number of rows. It keeps the cursor on a row.
func (l *List) SetLen(n int) {
	l.n = max(n, 0)
	l.move(0)
}

// SetHeight sets the number of visible rows.
func (l *List) SetHeight(h int) {
	l.height = max(h, 1)
	l.move(0)
}

// Len returns the number of rows.
func (l List) Len() int { return l.n }

// Cursor returns the index of the selected row.
func (l List) Cursor() int { return l.cursor }

// Select moves the cursor to row i.
func (l *List) Select(i int) { l.move(i - l.cursor) }

// Update moves the cursor for a key or a mouse wheel message. It returns
// true when the user chooses the selected row with Enter.
func (l *List) Update(msg tea.Msg) bool {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return l.key(msg)
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			l.move(-1)
		case tea.MouseWheelDown:
			l.move(1)
		}
	}
	return false
}

func (l *List) key(msg tea.KeyPressMsg) bool {
	page := max(l.height, 1)
	switch {
	case key.Matches(msg, listUp):
		l.move(-1)
	case key.Matches(msg, listDown):
		l.move(1)
	case key.Matches(msg, listPgUp):
		l.move(-page)
	case key.Matches(msg, listPgDown):
		l.move(page)
	case key.Matches(msg, listTop):
		l.move(-l.n)
	case key.Matches(msg, listBottom):
		l.move(l.n)
	case key.Matches(msg, listChoose):
		return l.n > 0
	}
	return false
}

func (l *List) move(d int) {
	l.cursor = max(0, min(l.cursor+d, l.n-1))
	h := max(l.height, 1)
	if l.cursor < l.offset {
		l.offset = l.cursor
	}
	if l.cursor >= l.offset+h {
		l.offset = l.cursor - h + 1
	}
	l.offset = max(0, min(l.offset, l.n-h))
}

// View draws the visible rows in width columns. The function row returns
// the text of row i for a width of w columns. The selected row gets a
// marker and the highlight of t at the phase.
func (l List) View(s Styles, t theme.Theme, width int, phase float64, row func(i, w int) string) string {
	h := max(l.height, 1)
	inner := max(width-2, 0)
	var lines []string
	for i := l.offset; i < l.n && i < l.offset+h; i++ {
		text := row(i, inner)
		if i == l.cursor {
			lines = append(lines, s.Key.Background(t.Selection).Render("▶ ")+Highlight(t, text, inner, phase))
			continue
		}
		lines = append(lines, "  "+ansi.Truncate(text, inner, "…"))
	}
	return strings.Join(lines, "\n")
}
