package fx

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
	colorful "github.com/lucasb-eyer/go-colorful"
)

// asciiRamp holds the characters for monochrome output, from dark to light.
const asciiRamp = " .:-=+*#%@"

// RenderOptions changes the output of Render.
type RenderOptions struct {
	// ASCII gives plain characters from asciiRamp and no escape codes.
	ASCII bool
}

// Render paints a block of w columns and h rows with the effect e at time t.
// The mask lines are centered in the block. The cells of the mask show the
// mask glyphs in a bright color on a dark plate, and the other cells show
// the effect. The rows have a reset code at the end and a newline between them.
func Render(e Effect, w, h int, t float64, mask []string, opt RenderOptions) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	m := newMaskGrid(mask, w, h)
	var b strings.Builder
	if opt.ASCII {
		b.Grow((w + 1) * h)
	} else {
		b.Grow(w * h * 22)
	}
	for y := range h {
		if y > 0 {
			b.WriteByte('\n')
		}
		if opt.ASCII {
			asciiRow(&b, e, m, y, w, h, t)
		} else {
			colorRow(&b, e, m, y, w, h, t)
		}
	}
	return b.String()
}

func colorRow(b *strings.Builder, e Effect, m maskGrid, y, w, h int, t float64) {
	var s sgr
	for x := range w {
		c := e.Color(x, y, w, h, t)
		r, onPlate := m.cell(x, y)
		if !onPlate {
			s.bg(b, toRGB(c))
			b.WriteByte(' ')
			continue
		}
		s.bg(b, toRGB(plate(c)))
		if r == ' ' {
			b.WriteByte(' ')
			continue
		}
		s.fg(b, toRGB(glow(c)))
		b.WriteRune(r)
	}
	b.WriteString("\x1b[m")
}

func asciiRow(b *strings.Builder, e Effect, m maskGrid, y, w, h int, t float64) {
	for x := range w {
		if r, onPlate := m.cell(x, y); onPlate {
			b.WriteRune(r)
			continue
		}
		c := e.Color(x, y, w, h, t)
		luma := 0.2126*c.R + 0.7152*c.G + 0.0722*c.B
		b.WriteByte(asciiRamp[int(clamp01(luma)*float64(len(asciiRamp)-1)+0.5)])
	}
}

// plate darkens the effect color behind the mask.
func plate(c colorful.Color) colorful.Color {
	l, a, bb := c.OkLab()
	return colorful.OkLab(l*0.3, a*0.5, bb*0.5).Clamped()
}

// glow gives a bright color with the hue of the effect color.
func glow(c colorful.Color) colorful.Color {
	_, a, bb := c.OkLab()
	return colorful.OkLab(0.93, a*0.55, bb*0.55).Clamped()
}

// maskGrid holds the mask runes and the position of the mask in the block.
type maskGrid struct {
	rows   [][]rune
	w, h   int
	ox, oy int
}

func newMaskGrid(mask []string, w, h int) maskGrid {
	m := maskGrid{rows: make([][]rune, len(mask)), h: len(mask)}
	for i, line := range mask {
		m.rows[i] = maskRunes(line)
		m.w = max(m.w, len(m.rows[i]))
	}
	m.ox = (w - m.w) / 2
	m.oy = (h - m.h) / 2
	return m
}

// maskRunes changes the runes that are not 1 cell wide to '?'.
func maskRunes(line string) []rune {
	rs := []rune(line)
	for i, r := range rs {
		if r >= 0x80 || r < 0x20 {
			if ansi.StringWidth(string(r)) != 1 {
				rs[i] = '?'
			}
		}
	}
	return rs
}

// cell returns the mask rune at the block position x, y. The second value is
// true when the position is on the plate. The plate is the mask box plus 1
// column on each side.
func (m maskGrid) cell(x, y int) (rune, bool) {
	mx, my := x-m.ox, y-m.oy
	if my < 0 || my >= m.h || mx < -1 || mx > m.w {
		return ' ', false
	}
	row := m.rows[my]
	if mx < 0 || mx >= len(row) {
		return ' ', true
	}
	return row[mx], true
}

type rgb [3]uint8

func toRGB(c colorful.Color) rgb {
	r, g, b := c.RGB255()
	return rgb{r, g, b}
}

// sgr writes color codes only when the color changes.
type sgr struct {
	fgOn, bgOn bool
	fgC, bgC   rgb
}

func (s *sgr) fg(b *strings.Builder, c rgb) {
	if s.fgOn && s.fgC == c {
		return
	}
	s.fgOn, s.fgC = true, c
	writeSGR(b, "\x1b[38;2;", c)
}

func (s *sgr) bg(b *strings.Builder, c rgb) {
	if s.bgOn && s.bgC == c {
		return
	}
	s.bgOn, s.bgC = true, c
	writeSGR(b, "\x1b[48;2;", c)
}

var decimal = func() (d [256]string) {
	for i := range d {
		d[i] = strconv.Itoa(i)
	}
	return d
}()

func writeSGR(b *strings.Builder, prefix string, c rgb) {
	b.WriteString(prefix)
	b.WriteString(decimal[c[0]])
	b.WriteByte(';')
	b.WriteString(decimal[c[1]])
	b.WriteByte(';')
	b.WriteString(decimal[c[2]])
	b.WriteByte('m')
}
