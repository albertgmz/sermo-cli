// Package banner makes large FIGlet banners from text.
package banner

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
	figure "github.com/common-nighthawk/go-figure"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// fonts holds the FIGlet fonts from go-figure, from the widest to the
// narrowest.
var fonts = []string{"colossal", "doom", "cybermedium"}

// Lines returns text as a FIGlet banner that is not wider than maxWidth
// columns. It uses the first font that fits. When no font fits, it returns
// the plain uppercase text, cut to maxWidth. Lines removes diacritics because
// the fonts have only ASCII glyphs.
func Lines(text string, maxWidth int) []string {
	return LinesWithFont(text, maxWidth, "")
}

// LinesWithFont is Lines with the font tried first. When font is empty,
// unknown or too wide, LinesWithFont uses the same fonts as Lines.
func LinesWithFont(text string, maxWidth int, font string) []string {
	text = strings.ToUpper(strings.TrimSpace(stripMarks(text)))
	if text == "" {
		return nil
	}
	order := fonts
	if HasFont(font) {
		order = append([]string{font}, fonts...)
	}
	for _, font := range order {
		lines := trimBlank(figure.NewFigure(text, font, false).Slicify())
		if len(lines) > 0 && width(lines) <= maxWidth {
			return lines
		}
	}
	return []string{ansi.Truncate(text, maxWidth, "")}
}

// HasFont reports whether go-figure has a font with the name.
func HasFont(name string) bool {
	if name == "" {
		return false
	}
	_, err := figure.Asset("fonts/" + name + ".flf")
	return err == nil
}

// stripMarks removes the combining marks, for example "á" becomes "a".
func stripMarks(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, s)
	if err != nil {
		return s
	}
	return out
}

// trimBlank removes the blank lines at the start and at the end.
func trimBlank(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func width(lines []string) int {
	n := 0
	for _, l := range lines {
		n = max(n, ansi.StringWidth(l))
	}
	return n
}
