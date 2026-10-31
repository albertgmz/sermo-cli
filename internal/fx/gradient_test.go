package fx

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	colorful "github.com/lucasb-eyer/go-colorful"
)

var (
	red   = colorful.Color{R: 1}
	green = colorful.Color{G: 1}
	blue  = colorful.Color{B: 1}
)

func TestGradientAtEndpoints(t *testing.T) {
	g := Gradient{red, green, blue}
	if got := g.At(0); !got.AlmostEqualRgb(red) {
		t.Errorf("At(0) = %v, want %v", got, red)
	}
	if got := g.At(1); !got.AlmostEqualRgb(blue) {
		t.Errorf("At(1) = %v, want %v", got, blue)
	}
	if got := g.At(0.5); !got.AlmostEqualRgb(green) {
		t.Errorf("At(0.5) = %v, want middle stop %v", got, green)
	}
}

func TestGradientAtBlendsInOkLab(t *testing.T) {
	g := Gradient{red, blue}
	want := red.BlendOkLab(blue, 0.5).Clamped()
	if got := g.At(0.5); !got.AlmostEqualRgb(want) {
		t.Errorf("At(0.5) = %v, want OKLab blend %v", got, want)
	}
	if rgb := red.BlendRgb(blue, 0.5); g.At(0.5).AlmostEqualRgb(rgb) {
		t.Errorf("At(0.5) equals the RGB blend %v", rgb)
	}
}

func TestGradientAtClampsT(t *testing.T) {
	g := Gradient{red, blue}
	if got := g.At(-3); !got.AlmostEqualRgb(red) {
		t.Errorf("At(-3) = %v, want %v", got, red)
	}
	if got := g.At(7); !got.AlmostEqualRgb(blue) {
		t.Errorf("At(7) = %v, want %v", got, blue)
	}
}

func TestGradientAtSingleStopAndEmpty(t *testing.T) {
	g := Gradient{green}
	for _, x := range []float64{0, 0.3, 1} {
		if got := g.At(x); !got.AlmostEqualRgb(green) {
			t.Errorf("At(%v) = %v, want %v", x, got, green)
		}
	}
	if got := (Gradient{}).At(0.5); !got.IsValid() {
		t.Errorf("empty gradient At = %v, want a valid color", got)
	}
}

func TestGradientAtResultIsValid(t *testing.T) {
	// Saturated stops can blend out of the sRGB gamut in OKLab.
	g := Gradient{colorful.Color{R: 1, B: 1}, colorful.Color{G: 1}}
	for i := 0; i <= 100; i++ {
		if c := g.At(float64(i) / 100); !c.IsValid() {
			t.Fatalf("At(%v) = %v is not a valid color", float64(i)/100, c)
		}
	}
}

func TestGradientText(t *testing.T) {
	g := Gradient{red, blue}
	s := "Hola, ñandú 🇨🇺"
	out := g.Text(s, 0)
	if got := ansi.Strip(out); got != s {
		t.Errorf("stripped text = %q, want %q", got, s)
	}
	if ansi.StringWidth(out) != ansi.StringWidth(s) {
		t.Errorf("width = %d, want %d", ansi.StringWidth(out), ansi.StringWidth(s))
	}
	if !strings.HasPrefix(out, "\x1b[38;2;255;0;0m") {
		t.Errorf("text does not start with the first stop color: %q", out)
	}
	if !strings.HasSuffix(out, "\x1b[39m") {
		t.Errorf("text does not reset the foreground at the end: %q", out)
	}
	if !strings.Contains(out, "\x1b[38;2;0;0;255m") {
		t.Errorf("last grapheme does not get the last stop color: %q", out)
	}
}

func TestGradientTextEmpty(t *testing.T) {
	if got := (Gradient{red}).Text("", 0); got != "" {
		t.Errorf("Text(\"\") = %q, want empty", got)
	}
}

func TestGradientTextPhaseShiftsColors(t *testing.T) {
	g := Gradient{red, blue}
	if g.Text("abcdef", 0) == g.Text("abcdef", 0.25) {
		t.Error("phase 0.25 gives the same output as phase 0")
	}
}
