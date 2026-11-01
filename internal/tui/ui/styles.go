package ui

import (
	"image/color"
	"math"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	colorful "github.com/lucasb-eyer/go-colorful"
	"github.com/rivo/uniseg"

	"github.com/albertgmz/sermo-cli/internal/fx"
	"github.com/albertgmz/sermo-cli/internal/theme"
)

// Ink is the dark text color on bright gradient backgrounds.
var Ink color.Color = colorful.Color{R: 0.04, G: 0.04, B: 0.08}

// Styles are the Lip Gloss styles of one theme. Use them for all text in
// the content area.
type Styles struct {
	Title, Subtitle, Text, Muted, Accent, Error, Success, Warning lipgloss.Style
	// Selected is the style of a selected row without animation.
	Selected lipgloss.Style
	// Badge is a small label with a colored background, for example a count.
	Badge lipgloss.Style
	// Key is the style of a hotkey letter.
	Key lipgloss.Style
	// Border is a rounded box with the border gradient of the theme.
	Border lipgloss.Style
}

// NewStyles makes the styles for the theme t.
func NewStyles(t theme.Theme) Styles {
	base := lipgloss.NewStyle()
	return Styles{
		Title:    base.Bold(true).Foreground(t.Accent),
		Subtitle: base.Bold(true).Foreground(t.Text),
		Text:     base.Foreground(t.Text),
		Muted:    base.Foreground(t.Muted),
		Accent:   base.Foreground(t.Accent),
		Error:    base.Bold(true).Foreground(t.Error),
		Success:  base.Foreground(t.Success),
		Warning:  base.Foreground(t.Warning),
		Selected: base.Bold(true).Foreground(t.Text).Background(t.Selection),
		Badge:    base.Bold(true).Foreground(Ink).Background(t.Accent).Padding(0, 1),
		Key:      base.Bold(true).Foreground(t.Accent),
		Border: base.Border(lipgloss.RoundedBorder()).
			BorderForegroundBlend(loop(t.Border)...),
	}
}

// loop gives the stops of g forward and back, so that a blend around a box
// ends with the color where it starts.
func loop(g fx.Gradient) []color.Color {
	out := make([]color.Color, 0, 2*len(g))
	for _, c := range g {
		out = append(out, c)
	}
	for i := len(g) - 2; i >= 0; i-- {
		out = append(out, g[i])
	}
	return out
}

// Header returns a section header of width columns: the text on a gradient
// bar, then a gradient rule to the end of the line.
func Header(s Styles, t theme.Theme, text string, width int) string {
	if width <= 0 {
		return ""
	}
	label := " " + ansi.Truncate(ansi.Strip(text), max(width-2, 0), "…") + " "
	tail := []rune("▓▒░")
	var p painter
	col := 0
	at := func() colorful.Color { return t.Title.At(float64(col) / float64(max(width-1, 1))) }
	p.b.WriteString("\x1b[1m")
	g := uniseg.NewGraphemes(label)
	for g.Next() && col < width {
		p.bg(at())
		p.fg(toColorful(Ink))
		p.b.WriteString(g.Str())
		col += g.Width()
	}
	p.b.WriteString("\x1b[22;49m")
	p.bgOn = false
	for ; col < width; col++ {
		p.fg(at())
		if i := col - ansi.StringWidth(label); i < len(tail) {
			p.b.WriteRune(tail[i])
			continue
		}
		p.b.WriteRune('━')
	}
	p.b.WriteString("\x1b[m")
	return p.b.String()
}

// Frame draws a box of width x height cells with the border gradient of t
// around content. t.BorderStyle selects the lines. The default is double. The title shows in the top border. The
// content area is width-4 columns and height-2 rows. Frame cuts longer
// content.
func Frame(s Styles, t theme.Theme, title, content string, width, height int) string {
	if width < 4 || height < 2 {
		return ""
	}
	f := frame{t: t, s: s, w: width, h: height, r: frameRunesFor(t.BorderStyle)}
	var b strings.Builder
	f.top(&b, title)
	lines := strings.Split(content, "\n")
	inner := width - 4
	for y := 1; y < height-1; y++ {
		b.WriteByte('\n')
		f.side(&b, y, true)
		b.WriteByte(' ')
		line := ""
		if y-1 < len(lines) {
			line = ansi.Truncate(lines[y-1], inner, "")
		}
		b.WriteString(line)
		b.WriteString("\x1b[m")
		b.WriteString(strings.Repeat(" ", inner-ansi.StringWidth(line)))
		b.WriteByte(' ')
		f.side(&b, y, false)
	}
	b.WriteByte('\n')
	f.bottom(&b)
	return b.String()
}

type frame struct {
	t    theme.Theme
	s    Styles
	w, h int
	r    frameRunes
}

// frameRunes are the lines of a frame and the two marks around the title.
type frameRunes struct {
	lipgloss.Border
	open, close string
}

var frameStyles = map[string]frameRunes{
	"normal":  {lipgloss.NormalBorder(), "┤", "├"},
	"rounded": {lipgloss.RoundedBorder(), "┤", "├"},
	"thick":   {lipgloss.ThickBorder(), "┫", "┣"},
	"ascii":   {lipgloss.ASCIIBorder(), "|", "|"},
}

// frameRunesFor returns the lines for a border style. An unknown style
// gives double lines.
func frameRunesFor(style string) frameRunes {
	if r, ok := frameStyles[style]; ok {
		return r
	}
	return frameRunes{lipgloss.DoubleBorder(), "╡", "╞"}
}

// color gives the border color at perimeter index i. The index goes
// clockwise from the top left corner.
func (f frame) color(i int) colorful.Color {
	per := 2*(f.w-1) + 2*(f.h-1)
	return f.t.Border.At(pingPong(2 * float64(i) / float64(per)))
}

func (f frame) top(b *strings.Builder, title string) {
	var p painter
	title = ansi.Truncate(ansi.Strip(title), max(f.w-8, 0), "…")
	tw := ansi.StringWidth(title)
	x := 0
	cell := func(r string) {
		p.fg(f.color(x))
		p.b.WriteString(r)
		x++
	}
	cell(f.r.TopLeft)
	cell(f.r.Top)
	if tw > 0 {
		cell(f.r.open)
		p.b.WriteString("\x1b[m")
		p.fgOn = false
		p.b.WriteString(f.s.Subtitle.Render(" " + title + " "))
		x += tw + 2
		cell(f.r.close)
	}
	for x < f.w-1 {
		cell(f.r.Top)
	}
	cell(f.r.TopRight)
	p.b.WriteString("\x1b[m")
	b.WriteString(p.b.String())
}

func (f frame) side(b *strings.Builder, y int, left bool) {
	var p painter
	line := f.r.Right
	if left {
		line = f.r.Left
		p.fg(f.color(2*(f.w-1) + 2*(f.h-1) - y))
	} else {
		p.fg(f.color(f.w - 1 + y))
	}
	p.b.WriteString(line + "\x1b[m")
	b.WriteString(p.b.String())
}

func (f frame) bottom(b *strings.Builder) {
	var p painter
	base := f.w - 1 + f.h - 1
	for x := range f.w {
		p.fg(f.color(base + f.w - 1 - x))
		switch x {
		case 0:
			p.b.WriteString(f.r.BottomLeft)
		case f.w - 1:
			p.b.WriteString(f.r.BottomRight)
		default:
			p.b.WriteString(f.r.Bottom)
		}
	}
	p.b.WriteString("\x1b[m")
	b.WriteString(p.b.String())
}

// Highlight returns text as a selected row of width columns: the highlight
// gradient of t on the selection color. The phase moves the colors. Use the
// same phase for a static highlight. The zones of text stay in the result.
func Highlight(t theme.Theme, text string, width int, phase float64) string {
	if width <= 0 {
		return ""
	}
	plain := ansi.Truncate(ansi.Strip(text), width, "…")
	plain += strings.Repeat(" ", width-ansi.StringWidth(plain))
	var p painter
	p.bg(toColorful(t.Selection))
	p.b.WriteString("\x1b[1m")
	p.b.WriteString(t.Highlight.Text(plain, phase))
	p.b.WriteString("\x1b[m")
	return KeepZones(text, p.b.String())
}

func pingPong(x float64) float64 {
	x = math.Mod(x, 2)
	if x < 0 {
		x += 2
	}
	if x > 1 {
		return 2 - x
	}
	return x
}

func toColorful(c color.Color) colorful.Color {
	cc, _ := colorful.MakeColor(c)
	return cc
}
