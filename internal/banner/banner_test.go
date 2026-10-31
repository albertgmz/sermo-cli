package banner

import (
	"strings"
	"testing"
)

func TestLinesFitsWidth(t *testing.T) {
	for _, w := range []int{200, 120, 80, 60, 50} {
		lines := Lines("Foros Cubá", w)
		if len(lines) < 2 {
			t.Errorf("width %d: got %d lines, want a FIGlet banner", w, len(lines))
		}
		checkWidth(t, lines, w)
	}
}

func TestLinesUsesBiggerFontOnWideScreens(t *testing.T) {
	wide, narrow := Lines("Foros Cuba", 200), Lines("Foros Cuba", 60)
	if cols(wide) <= cols(narrow) {
		t.Errorf("wide banner is %d columns, narrow banner is %d", cols(wide), cols(narrow))
	}
}

func TestLinesFallsBackToPlainText(t *testing.T) {
	got := Lines("Foros Cubá", 12)
	want := []string{"FOROS CUBA"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("Lines = %q, want %q", got, want)
	}
}

func TestLinesFallbackTruncates(t *testing.T) {
	got := Lines("Foros Cuba", 5)
	if len(got) != 1 || got[0] != "FOROS" {
		t.Errorf("Lines = %q, want [\"FOROS\"]", got)
	}
}

func TestLinesStripsDiacritics(t *testing.T) {
	accented := Lines("Ñandú Café", 200)
	plain := Lines("Nandu Cafe", 200)
	if strings.Join(accented, "\n") != strings.Join(plain, "\n") {
		t.Errorf("accented banner differs from plain banner:\n%s\n---\n%s",
			strings.Join(accented, "\n"), strings.Join(plain, "\n"))
	}
	if strings.Contains(strings.Join(accented, ""), "?") {
		t.Error("banner contains '?' for an accented letter")
	}
}

func TestLinesHasNoEmptyEdgeLines(t *testing.T) {
	lines := Lines("Foros Cuba", 200)
	if strings.TrimSpace(lines[0]) == "" || strings.TrimSpace(lines[len(lines)-1]) == "" {
		t.Errorf("banner has an empty first or last line: %q", lines)
	}
}

func TestLinesEmpty(t *testing.T) {
	if got := Lines("   ", 80); len(got) != 0 {
		t.Errorf("Lines(blank) = %q, want none", got)
	}
}

func checkWidth(t *testing.T, lines []string, w int) {
	t.Helper()
	if got := cols(lines); got > w {
		t.Errorf("banner is %d columns, max %d", got, w)
	}
}

func cols(lines []string) int {
	n := 0
	for _, l := range lines {
		n = max(n, len([]rune(l)))
	}
	return n
}

func TestLinesWithFontUsesTheFont(t *testing.T) {
	got := LinesWithFont("Foros Cuba", 200, "standard")
	if strings.Join(got, "\n") == strings.Join(Lines("Foros Cuba", 200), "\n") {
		t.Fatal("the standard font gives the same banner as the default fonts")
	}
	if len(got) < 2 {
		t.Errorf("got %d lines, want a FIGlet banner", len(got))
	}
}

func TestLinesWithFontFallsBackWhenTooWide(t *testing.T) {
	got := LinesWithFont("Foros Cuba", 60, "colossal")
	want := Lines("Foros Cuba", 60)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("LinesWithFont = %q, want the default banner %q", got, want)
	}
}

func TestLinesWithFontIgnoresUnknownFont(t *testing.T) {
	got := LinesWithFont("Foros Cuba", 200, "no-such-font")
	if strings.Join(got, "\n") != strings.Join(Lines("Foros Cuba", 200), "\n") {
		t.Errorf("an unknown font must give the default banner, got %q", got)
	}
}

func TestHasFont(t *testing.T) {
	for name, want := range map[string]bool{"doom": true, "standard": true, "slant": true, "no-such-font": false, "": false, "../fonts/doom": false} {
		if got := HasFont(name); got != want {
			t.Errorf("HasFont(%q) = %v, want %v", name, got, want)
		}
	}
}
