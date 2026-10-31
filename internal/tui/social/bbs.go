package social

import (
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// maxBBSRows is the largest number of rows that a BBS table asks for.
const maxBBSRows = 50

// now returns the time of the clock of d.
func now(d *ui.Deps) time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

// monthName returns the short name of the month m in the current
// language.
func monthName(m time.Month) string {
	months := strings.Split(i18n.T("bbs.months"), ",")
	if len(months) == 12 {
		return months[m-1]
	}
	return m.String()[:3]
}

// bbsDate formats t like "31-Oct-2026 14:02".
func bbsDate(d *ui.Deps, t time.Time) string {
	t = t.In(now(d).Location())
	return strconv.Itoa(t.Day()) + "-" + monthName(t.Month()) + "-" + strconv.Itoa(t.Year()) + " " + t.Format("15:04")
}

// bbsDay formats t like "31-Oct-2026".
func bbsDay(d *ui.Deps, t time.Time) string {
	t = t.In(now(d).Location())
	return strconv.Itoa(t.Day()) + "-" + monthName(t.Month()) + "-" + strconv.Itoa(t.Year())
}

// bbsAgo formats t short and relative to the clock: "ahora", "hace 3m",
// "hace 2h", "ayer", "29-Oct" in this year, else "12-Ene-2025".
func bbsAgo(d *ui.Deps, t time.Time) string {
	n := now(d)
	t = t.In(n.Location())
	ago := n.Sub(t)
	switch {
	case ago < time.Minute:
		return i18n.T("bbs.social.ago.now")
	case ago < time.Hour:
		return i18n.Tf("bbs.social.ago.minutes", map[string]any{"N": int(ago.Minutes())})
	case ago < 24*time.Hour && t.Day() == n.Day():
		return i18n.Tf("bbs.social.ago.hours", map[string]any{"N": int(ago.Hours())})
	}
	y := n.AddDate(0, 0, -1)
	if t.Year() == y.Year() && t.YearDay() == y.YearDay() {
		return i18n.T("bbs.social.ago.yesterday")
	}
	if t.Year() == n.Year() {
		return strconv.Itoa(t.Day()) + "-" + monthName(t.Month())
	}
	return bbsDay(d, t)
}

// bbsRows returns the number of table rows that fit in height lines when
// the frame of the table takes used lines.
func bbsRows(height, used int) int {
	return max(1, min(height-used, maxBBSRows))
}

// refitPage returns the page of limit items that shows the first item of
// page when a page had old items.
func refitPage(page, old, limit int) int {
	return (page-1)*old/limit + 1
}

// pager is a window of per rows over a list that can grow at its end.
// sel is the selected row in the window.
type pager struct {
	start, per, sel int
}

// window returns the bounds of the shown rows of a list of n rows.
func (p *pager) window(n int) (int, int) {
	lo := min(p.start, n)
	return lo, min(lo+max(p.per, 1), n)
}

// next moves to the next window. It returns true when the rows of the new
// window must load first.
func (p *pager) next(n int, more bool) bool {
	if p.start+p.per < n {
		p.start += p.per
		p.sel = 0
		return false
	}
	if !more {
		return false
	}
	p.start += p.per
	p.sel = 0
	return true
}

// prev moves to the window before.
func (p *pager) prev() {
	p.start = max(p.start-p.per, 0)
	p.sel = 0
}

// clamp keeps the window inside a list of n rows.
func (p *pager) clamp(n int) {
	if p.start >= n && n > 0 {
		p.start = (n - 1) / max(p.per, 1) * max(p.per, 1)
	}
	if p.start >= n {
		p.start = 0
	}
	_, hi := p.window(n)
	p.sel = max(0, min(p.sel, hi-p.start-1))
}

// move moves the selection with the arrow keys. It returns true when the
// key moved it.
func (p *pager) move(k string, n int) bool {
	lo, hi := p.window(n)
	switch k {
	case "up", "k":
		p.sel = max(p.sel-1, 0)
	case "down", "j":
		p.sel = min(p.sel+1, max(hi-lo-1, 0))
	default:
		return false
	}
	return true
}

// pick returns the list index of the row number arg in the window, or
// false when no row has that number.
func (p *pager) pick(arg string, n int) (int, bool) {
	i, err := strconv.Atoi(arg)
	lo, hi := p.window(n)
	if err != nil || i < 1 || lo+i > hi {
		return 0, false
	}
	return lo + i - 1, true
}

// noRow is the toast for a number that no row has.
func noRow() tea.Cmd {
	return ui.Toast(i18n.T("bbs.social.noRow"), ui.ToastError)
}

// boxRow returns the line "| text |" of width cells.
func boxRow(s ui.Styles, width int, text string) string {
	return strings.Split(ui.Box(s, width, []string{text}), "\n")[1]
}

// apart puts left at the start and right at the end of width cells.
func apart(left, right string, width int) string {
	gap := max(width-ansi.StringWidth(left)-ansi.StringWidth(right), 2)
	return left + strings.Repeat(" ", gap) + right
}

// bbsView returns the BBS view of the load state and true, or false
// when the data is ready. It has no spinner and no Unicode glyphs.
func (l *loader) bbsView(width, height int) (string, bool) {
	s := l.d.Styles
	var text string
	switch {
	case l.loading:
		text = s.Muted.Render(i18n.T("bbs.social.loading"))
	case l.err != nil:
		text = s.Error.Render(ui.ErrorText(l.err))
	default:
		return "", false
	}
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, lipgloss.Wrap(text, width, " ")), true
}

// pageCommands returns the commands [<] and [>] with one label.
func pageCommands(prev, next func() tea.Cmd) []ui.Command {
	return []ui.Command{
		{Name: "<", Label: i18n.T("bbs.social.pages"), Run: func(string) tea.Cmd { return prev() }},
		{Name: ">", Run: func(string) tea.Cmd { return next() }},
	}
}

// back is the hint of the global back command.
func back() ui.Command { return ui.Command{Name: "V", Label: i18n.T("bbs.global.back")} }
