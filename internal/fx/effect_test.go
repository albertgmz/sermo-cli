package fx

import (
	"math/rand/v2"
	"slices"
	"testing"

	colorful "github.com/lucasb-eyer/go-colorful"
)

var testPalette = Gradient{
	colorful.Color{R: 0.2, G: 0, B: 0.4},
	colorful.Color{R: 1, G: 0.1, B: 0.6},
	colorful.Color{R: 0, G: 1, B: 1},
}

func TestAllNamesInOrder(t *testing.T) {
	var names []string
	for _, e := range All(testPalette) {
		names = append(names, e.Name)
	}
	want := []string{"plasma", "fire", "gradient", "starfield", "crt"}
	if !slices.Equal(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
}

func TestEffectsGiveValidDeterministicColors(t *testing.T) {
	const w, h = 40, 20
	for _, e := range All(testPalette) {
		again, _ := ByName(e.Name, testPalette)
		for _, tm := range []float64{0, 0.5, 3.7, 125.25} {
			checkFrame(t, e, again, w, h, tm)
		}
	}
}

func checkFrame(t *testing.T, e, again Effect, w, h int, tm float64) {
	t.Helper()
	for y := range h {
		for x := range w {
			c := e.Color(x, y, w, h, tm)
			if !c.IsValid() {
				t.Fatalf("%s(%d,%d,t=%v) = %v is not valid", e.Name, x, y, tm, c)
			}
			if c2 := again.Color(x, y, w, h, tm); c2 != c {
				t.Fatalf("%s(%d,%d,t=%v) is not deterministic: %v and %v", e.Name, x, y, tm, c, c2)
			}
		}
	}
}

func TestEffectsChangeOverTime(t *testing.T) {
	const w, h = 40, 20
	for _, e := range All(testPalette) {
		if frame(e, w, h, 0) == frame(e, w, h, 2.5) {
			t.Errorf("%s gives the same frame at t=0 and t=2.5", e.Name)
		}
	}
}

func TestEffectsUseThePalette(t *testing.T) {
	const w, h = 40, 20
	other := Gradient{colorful.Color{G: 0.3}, colorful.Color{R: 1, G: 0.8}}
	for i, e := range All(testPalette) {
		if frame(e, w, h, 1) == frame(All(other)[i], w, h, 1) {
			t.Errorf("%s ignores the palette", e.Name)
		}
	}
}

func TestByNameUnknown(t *testing.T) {
	if _, ok := ByName("lava-lamp", testPalette); ok {
		t.Error("ByName(\"lava-lamp\") returned ok")
	}
}

func TestRandomPicksFromAll(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	names := map[string]bool{}
	for _, e := range All(testPalette) {
		names[e.Name] = true
	}
	seen := map[string]bool{}
	for range 200 {
		e := Random(r, testPalette)
		if !names[e.Name] {
			t.Fatalf("Random returned unknown effect %q", e.Name)
		}
		seen[e.Name] = true
	}
	if len(seen) != len(names) {
		t.Errorf("Random gave %d of %d effects in 200 picks", len(seen), len(names))
	}
}

func frame(e Effect, w, h int, tm float64) string {
	var b []byte
	for y := range h {
		for x := range w {
			r, g, bl := e.Color(x, y, w, h, tm).RGB255()
			b = append(b, r, g, bl)
		}
	}
	return string(b)
}
