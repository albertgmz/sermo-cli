package ui

import (
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/theme"
)

// highlightTime is how long the highlight of the selected row animates
// after the last cursor move.
const highlightTime = 2 * time.Second

// List is a cursor over a list of selectable rows. The screen keeps the
// items and draws each row. The zero value is an empty list that uses the
// system clock.
type List struct {
	n, cursor, offset, height int
	now                       func() time.Time
	moved                     time.Time
	d                         *Deps  // marks the rows; nil means no clicks
	zone                      string // the zone ID of the list
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

// SetClock sets the clock that records the time of the cursor moves. A nil
// clock is the system clock.
func (l *List) SetClock(now func() time.Time) { l.now = now }

// SetZones makes the rows clickable. d marks the rows. The zone ID id must
// be unique on the screen.
func (l *List) SetZones(d *Deps, id string) { l.d, l.zone = d, id }

// RowZone returns the zone ID of row i. A screen that draws its own rows,
// for example in a Table, marks them with this ID.
func (l List) RowZone(i int) string { return l.zone + "." + strconv.Itoa(i) }

// Reset makes the list empty. It keeps the clock and the zones.
func (l *List) Reset() { *l = List{now: l.now, d: l.d, zone: l.zone} }

// Animating reports whether the highlight animates at the time now. It is
// true for 2 s after the last cursor move and after the list first gets
// rows.
func (l List) Animating(now time.Time) bool {
	return l.n > 0 && now.Sub(l.moved) < highlightTime
}

// SetLen sets the number of rows. It keeps the cursor on a row.
func (l *List) SetLen(n int) {
	if l.n == 0 && n > 0 {
		l.touch()
	}
	l.n = max(n, 0)
	l.move(0)
}

// touch records the time of a cursor move.
func (l *List) touch() {
	if l.now == nil {
		l.moved = time.Now()
		return
	}
	l.moved = l.now()
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

// Update moves the cursor for a key, a click or a mouse wheel message. It
// returns true when the user chooses the selected row with Enter or with a
// click on it. A click on another row selects that row.
func (l *List) Update(msg tea.Msg) bool {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return l.key(msg)
	case tea.MouseReleaseMsg:
		return l.click(msg)
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

func (l *List) click(msg tea.MouseReleaseMsg) bool {
	if l.d == nil {
		return false
	}
	for i := range l.n {
		if l.d.Clicked(l.RowZone(i), msg) {
			if i == l.cursor {
				return true
			}
			l.Select(i)
			return false
		}
	}
	return false
}

func (l *List) move(d int) {
	before := l.cursor
	l.cursor = max(0, min(l.cursor+d, l.n-1))
	if l.cursor != before {
		l.touch()
	}
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
			lines = append(lines, l.markRow(i, s.Key.Background(t.Selection).Render("▶ ")+Highlight(t, text, inner, phase), width))
			continue
		}
		lines = append(lines, l.markRow(i, "  "+ansi.Truncate(text, inner, "…"), width))
	}
	return strings.Join(lines, "\n")
}

// markRow puts a row in its zone. The zone fills the width, so that a click
// right of the text also hits the row.
func (l List) markRow(i int, line string, width int) string {
	if !l.d.zonesOn() {
		return line
	}
	return l.d.Mark(l.RowZone(i), line+strings.Repeat(" ", max(width-ansi.StringWidth(line), 0)))
}
