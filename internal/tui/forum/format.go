package forum

import (
	"context"
	"hash/fnv"
	"image/color"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	colorful "github.com/lucasb-eyer/go-colorful"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/api/gen"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// pageSize is the number of threads or posts on one page of the server.
const pageSize = 20

// The glyphs of the lists. They show in Windows Terminal and on Linux.
const (
	glyphUnread = "●"
	glyphRead   = "○"
	glyphSticky = "▲"
	glyphLocked = "■"
	glyphMine   = "✓"
	glyphFocus  = "▌"
)

// bg is the context of the API calls of the screens.
func bg() context.Context { return context.Background() }

// pages returns the number of pages for total items. It is 1 or more.
func pages(total int) int { return max(1, (total+pageSize-1)/pageSize) }

// pageOf returns the page that holds the post at position.
func pageOf(position int) int { return max(position, 0)/pageSize + 1 }

// ago returns the time from t to now in words.
func ago(now, t time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return i18n.T("forum.ago.now")
	case d < time.Hour:
		return i18n.Plural("forum.ago.minutes", int(d.Minutes()), nil)
	case d < 24*time.Hour:
		return i18n.Plural("forum.ago.hours", int(d.Hours()), nil)
	case d < 30*24*time.Hour:
		return i18n.Plural("forum.ago.days", int(d.Hours()/24), nil)
	case d < 365*24*time.Hour:
		return i18n.Plural("forum.ago.months", int(d.Hours()/(24*30)), nil)
	}
	return i18n.Plural("forum.ago.years", int(d.Hours()/(24*365)), nil)
}

// absDate returns t as a local date and time.
func absDate(t time.Time) string { return t.Local().Format("02/01/2006 15:04") }

// authorName returns the name of an author, or the guest label.
func authorName(a gen.ContentAuthor) string {
	if n := api.AuthorName(a); n != "" {
		return n
	}
	return i18n.T("forum.post.guest")
}

// hexColor reads a "#rrggbb" color. It returns fallback for other text.
func hexColor(s string, fallback color.Color) color.Color {
	if c, err := colorful.Hex(s); err == nil {
		return c
	}
	return fallback
}

// stableColor picks a color from colors for id. The same id always gets
// the same color.
func stableColor(colors []color.Color, id int, fallback color.Color) color.Color {
	if len(colors) == 0 {
		return fallback
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(strconv.Itoa(id)))
	return colors[int(h.Sum32()%uint32(len(colors)))]
}

// paletteColors returns the colors of a palette.
func paletteColors(items []gen.PaletteColor) []color.Color {
	out := make([]color.Color, 0, len(items))
	for _, p := range items {
		if c, err := colorful.Hex(p.Color); err == nil && p.IsActive {
			out = append(out, c)
		}
	}
	return out
}

// fit puts left and right on one line of width w. It cuts left when the
// line is too narrow.
func fit(left, right string, w int) string {
	rw := ansi.StringWidth(right)
	if rw+2 > w {
		return ansi.Truncate(left, w, "…")
	}
	left = ansi.Truncate(left, w-rw-1, "…")
	return left + strings.Repeat(" ", w-rw-ansi.StringWidth(left)) + right
}

// oneLine changes line breaks in s into spaces.
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// prefixTag returns the label of a thread prefix in its color.
func prefixTag(d *ui.Deps, p *gen.ThreadPrefixTag) string {
	if p == nil {
		return ""
	}
	st := lipgloss.NewStyle().Bold(true).Foreground(hexColor(p.Color, d.Theme.Accent))
	return st.Render("["+p.Label+"]") + " "
}

// errorView shows the text of err and the retry hint in the center.
func errorView(d *ui.Deps, err error, w, h int) string {
	text := d.Styles.Error.Render(ui.ErrorText(err)) + "\n" + d.Styles.Muted.Render(i18n.T("forum.retry"))
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, lipgloss.NewStyle().Width(w).Align(lipgloss.Center).Render(text))
}

// spin moves the spinner for a tick while the screen loads. Without
// animation the spinner stays still.
func spin(d *ui.Deps, sp *spinner.Model, loading bool, msg spinner.TickMsg) tea.Cmd {
	if !loading || !d.Animate() {
		return nil
	}
	var cmd tea.Cmd
	*sp, cmd = sp.Update(msg)
	return cmd
}

func toast(id string) tea.Cmd { return ui.Toast(i18n.T(id), ui.ToastInfo) }

func loginToast() tea.Cmd { return toast("forum.toast.loginNeeded") }
