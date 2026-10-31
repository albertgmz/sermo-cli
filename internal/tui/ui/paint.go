package ui

import (
	"strconv"
	"strings"

	colorful "github.com/lucasb-eyer/go-colorful"
)

// painter writes truecolor codes only when the color changes. Bubble Tea
// changes the codes to the color profile of the terminal.
type painter struct {
	b          strings.Builder
	fgOn, bgOn bool
	fgC, bgC   [3]uint8
}

func (p *painter) fg(c colorful.Color) {
	v := rgb(c)
	if p.fgOn && p.fgC == v {
		return
	}
	p.fgOn, p.fgC = true, v
	p.code("38", v)
}

func (p *painter) bg(c colorful.Color) {
	v := rgb(c)
	if p.bgOn && p.bgC == v {
		return
	}
	p.bgOn, p.bgC = true, v
	p.code("48", v)
}

func (p *painter) code(kind string, v [3]uint8) {
	p.b.WriteString("\x1b[" + kind + ";2;")
	p.b.WriteString(strconv.Itoa(int(v[0])) + ";" + strconv.Itoa(int(v[1])) + ";" + strconv.Itoa(int(v[2])) + "m")
}

func rgb(c colorful.Color) [3]uint8 {
	r, g, b := c.Clamped().RGB255()
	return [3]uint8{r, g, b}
}
