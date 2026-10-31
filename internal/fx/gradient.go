// Package fx contains the visual effects, the color gradients and the cell
// renderer for the splash screen and the headers.
package fx

import (
	"math"
	"strings"

	colorful "github.com/lucasb-eyer/go-colorful"
	"github.com/rivo/uniseg"
)

// Gradient is a list of color stops at equal distances.
type Gradient []colorful.Color

// At returns the color at position t. The value 0 gives the first stop and 1
// gives the last stop. At clamps t into [0, 1] and blends in OKLab.
func (g Gradient) At(t float64) colorful.Color {
	switch len(g) {
	case 0:
		return colorful.Color{}
	case 1:
		return g[0].Clamped()
	}
	t = clamp01(t) * float64(len(g)-1)
	i := int(t)
	if i >= len(g)-1 {
		return g[len(g)-1].Clamped()
	}
	return g[i].BlendOkLab(g[i+1], t-float64(i)).Clamped()
}

// Text gives s a foreground color for each grapheme. The colors go from the
// first stop to the last stop. The phase moves the colors along the string,
// and the gradient turns back at each end.
func (g Gradient) Text(s string, phase float64) string {
	n := uniseg.GraphemeClusterCount(s)
	if n == 0 {
		return ""
	}
	span := float64(max(n-1, 1))
	var b strings.Builder
	b.Grow(len(s) + n*20)
	var p sgr
	gr := uniseg.NewGraphemes(s)
	for i := 0; gr.Next(); i++ {
		r, gg, bl := g.At(pingPong(float64(i)/span + phase)).RGB255()
		p.fg(&b, rgb{r, gg, bl})
		b.WriteString(gr.Str())
	}
	b.WriteString("\x1b[39m")
	return b.String()
}

// pingPong maps x to [0, 1] in a triangle wave with a period of 2.
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

func clamp01(x float64) float64 {
	return math.Max(0, math.Min(1, x))
}
