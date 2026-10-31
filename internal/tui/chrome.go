package tui

import (
	"image/color"
	"strconv"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	colorful "github.com/lucasb-eyer/go-colorful"

	"github.com/albertgmz/sermo-cli/internal/fx"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/theme"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

const (
	minWidth  = 80
	minHeight = 24
)

// chrome holds the data of the bars around the content area.
type chrome struct {
	s        ui.Styles
	t        theme.Theme
	site     string
	user     string // "" for a guest
	alerts   int
	messages int
	clock    string
}

// statusBar draws the top bar: site, member, badges and clock.
func (c chrome) statusBar(width int) string {
	sep := c.s.Muted.Render("  │  ")
	left := " " + lipgloss.NewStyle().Bold(true).Render(c.t.Title.Text(c.site, 0)) + sep
	if c.user == "" {
		left += c.s.Muted.Render("☺ " + i18n.T("core.status.guest"))
	} else {
		left += c.s.Subtitle.Render("☻ "+c.user) + sep +
			c.badge(i18n.T("core.status.alerts"), c.alerts) + "  " +
			c.badge(i18n.T("core.status.messages"), c.messages)
	}
	right := c.s.Accent.Render(c.clock) + " "
	gap := width - ansi.StringWidth(left) - ansi.StringWidth(right)
	line := left + strings.Repeat(" ", max(gap, 1)) + right
	return withBackground(ansi.Truncate(line, width, ""), width, dim(c.t.Border))
}

func (c chrome) badge(label string, n int) string {
	if n == 0 {
		return c.s.Muted.Render(label + " 0")
	}
	return c.s.Text.Render(label+" ") + c.s.Badge.Render(strconv.Itoa(n))
}

// hotkeyBar draws the bottom bar. The catalog entry core.hotkey.style
// selects the style. The value "bracket" gives [F]oros, where the key is the
// first letter of the word. Other values give a keycap before each label.
// When the labels do not fit, the bar shows only the keys.
func (c chrome) hotkeyBar(width int) string {
	inline := i18n.T("core.hotkey.style") == "bracket"
	full := make([]string, len(hotkeys))
	short := make([]string, len(hotkeys))
	for i, h := range hotkeys {
		full[i], short[i] = c.hotkeyItem(h, inline)
	}
	line := " " + strings.Join(full, "  ")
	if ansi.StringWidth(line) > width {
		line = " " + strings.Join(short, " ")
	}
	return withBackground(ansi.Truncate(line, width, ""), width, dim(c.t.Border))
}

// hotkeyItem returns the item with its label and the item with the key only.
func (c chrome) hotkeyItem(h hotkey, inline bool) (string, string) {
	up := strings.ToUpper(h.key)
	label := i18n.T(h.label)
	if !inline {
		keycap := c.s.Badge.Render(up)
		return keycap + c.s.Text.Render(label), keycap
	}
	bracket := c.s.Muted.Render("[") + c.s.Key.Render(up) + c.s.Muted.Render("]")
	rest := label
	if first, size := utf8.DecodeRuneInString(label); strings.EqualFold(string(first), h.key) {
		rest = label[size:]
	}
	return bracket + c.s.Text.Render(rest), bracket
}

// confirmBar draws the quit question in the place of the hotkey bar.
func (c chrome) confirmBar(width int) string {
	line := " " + c.s.Warning.Bold(true).Render(i18n.T("core.quit.confirm"))
	return withBackground(ansi.Truncate(line, width, "…"), width, dim(c.t.Border))
}

// toastBox draws a toast in a small box with the color of its kind.
func toastBox(s ui.Styles, t theme.Theme, msg ui.ToastMsg, maxWidth int) string {
	color := t.Accent
	switch msg.Kind {
	case ui.ToastSuccess:
		color = t.Success
	case ui.ToastError:
		color = t.Error
	}
	text := lipgloss.Wrap(msg.Text, max(maxWidth-4, 10), " ")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).BorderForeground(color).
		Foreground(t.Text).Background(t.Selection).
		Padding(0, 1).Render(text)
}

// overlay draws top over base with the top left corner at x, y.
func overlay(base, top string, x, y int) string {
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(base),
		lipgloss.NewLayer(top).X(x).Y(y).Z(1),
	).Render()
}

// withBackground fills a line of width cells with the gradient g as the
// background of each cell that has no background.
func withBackground(line string, width int, g fx.Gradient) string {
	c := lipgloss.NewCanvas(width, 1)
	c.Compose(lipgloss.NewLayer(line))
	for x := range width {
		if cell := c.CellAt(x, 0); cell != nil && cell.Style.Bg == nil {
			r, gg, b := g.At(float64(x) / float64(max(width-1, 1))).RGB255()
			cell.Style.Bg = color.RGBA{R: r, G: gg, B: b, A: 255}
		}
	}
	return c.Render()
}

// dim returns a dark copy of g for bar backgrounds.
func dim(g fx.Gradient) fx.Gradient {
	out := make(fx.Gradient, len(g))
	for i, c := range g {
		l, a, b := c.OkLab()
		out[i] = colorful.OkLab(l*0.24, a*0.35, b*0.35).Clamped()
	}
	return out
}

// tooSmall returns the message for a window under the minimum size.
func tooSmall(s ui.Styles, width, height int) string {
	text := i18n.Tf("core.tooSmall", map[string]any{"Width": width, "Height": height})
	text = lipgloss.Wrap(text, max(width-2, 10), " ")
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
		s.Warning.Align(lipgloss.Center).Render(text))
}
