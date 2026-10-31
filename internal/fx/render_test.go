package fx

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

var testMask = []string{
	" /\\ ",
	"/__\\",
}

func TestRenderDimensions(t *testing.T) {
	sizes := [][2]int{{80, 24}, {1, 1}, {7, 3}, {200, 60}}
	for _, e := range All(testPalette) {
		for _, sz := range sizes {
			for _, ascii := range []bool{false, true} {
				out := Render(e, sz[0], sz[1], 1.5, testMask, RenderOptions{ASCII: ascii})
				checkBlock(t, e.Name, out, sz[0], sz[1])
			}
		}
	}
}

func TestRenderZeroSize(t *testing.T) {
	e := All(testPalette)[0]
	if out := Render(e, 0, 10, 0, testMask, RenderOptions{}); out != "" {
		t.Errorf("w=0 gives %q, want empty", out)
	}
	if out := Render(e, 10, 0, 0, testMask, RenderOptions{}); out != "" {
		t.Errorf("h=0 gives %q, want empty", out)
	}
}

func TestRenderASCIIHasNoEscapeCodes(t *testing.T) {
	for _, e := range All(testPalette) {
		out := Render(e, 60, 20, 2, testMask, RenderOptions{ASCII: true})
		if strings.ContainsRune(out, '\x1b') {
			t.Fatalf("%s ASCII output contains ESC", e.Name)
		}
		for _, r := range out {
			if r != '\n' && !strings.ContainsRune(asciiRamp+"/\\_", r) {
				t.Fatalf("%s ASCII output contains %q", e.Name, r)
			}
		}
	}
}

func TestRenderCentersMask(t *testing.T) {
	cases := []struct {
		name   string
		ascii  bool
		w, h   int
		mask   []string
		row    int
		col    int
		expect string
	}{
		{"even ascii", true, 10, 6, testMask, 3, 3, "/__\\"},
		{"odd ascii", true, 11, 5, testMask, 1, 3, " /\\ "},
		{"color", false, 10, 6, testMask, 3, 3, "/__\\"},
		{"clip wide mask", true, 2, 2, []string{"abcdef", "ghijkl"}, 0, 0, "cd"},
	}
	for _, c := range cases {
		e, _ := ByName("plasma", testPalette)
		lines := strings.Split(ansi.Strip(Render(e, c.w, c.h, 0, c.mask, RenderOptions{ASCII: c.ascii})), "\n")
		got := string([]rune(lines[c.row])[c.col : c.col+len(c.expect)])
		if strings.TrimSpace(c.expect) != strings.TrimSpace(got) {
			t.Errorf("%s: row %d col %d = %q, want %q", c.name, c.row, c.col, got, c.expect)
		}
	}
}

func TestRenderWideMaskRunes(t *testing.T) {
	e := All(testPalette)[0]
	out := Render(e, 20, 3, 0, []string{"日本 ok"}, RenderOptions{})
	checkBlock(t, "wide mask", out, 20, 3)
}

func TestRenderResetsAtLineEnds(t *testing.T) {
	e := All(testPalette)[0]
	for i, line := range strings.Split(Render(e, 30, 5, 0, testMask, RenderOptions{}), "\n") {
		if !strings.HasSuffix(line, "\x1b[m") {
			t.Errorf("line %d does not end with a reset", i)
		}
	}
}

func checkBlock(t *testing.T, name, out string, w, h int) {
	t.Helper()
	lines := strings.Split(out, "\n")
	if len(lines) != h {
		t.Fatalf("%s %dx%d: %d lines, want %d", name, w, h, len(lines), h)
	}
	for i, l := range lines {
		if got := ansi.StringWidth(l); got != w {
			t.Fatalf("%s %dx%d: line %d width %d, want %d", name, w, h, i, got, w)
		}
	}
}

func BenchmarkRender(b *testing.B) {
	mask := []string{
		"8888888888  .d88888b.  8888888b.   .d88888b.   .d8888b.",
		"888        d88P\" \"Y88b 888   Y88b d88P\" \"Y88b d88P  Y88b",
		"8888888    888     888 888   d88P 888     888  \"Y888b.",
		"888        Y88b. .d88P 888  T88b  Y88b. .d88P Y88b  d88P",
	}
	for _, e := range All(testPalette) {
		b.Run(e.Name, func(b *testing.B) {
			tm := 0.0
			for b.Loop() {
				tm += 1.0 / 30
				_ = Render(e, 200, 60, tm, mask, RenderOptions{})
			}
		})
	}
}
