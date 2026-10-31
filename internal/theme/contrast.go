package theme

import (
	"fmt"
	"image/color"
	"math"
)

// Minimum contrast ratios on a dark background.
const (
	minTextContrast  = 7
	minOtherContrast = 4.5
)

// darkBackgrounds are the backgrounds for the contrast check: black and a
// usual dark terminal gray.
var darkBackgrounds = []color.Color{
	color.RGBA{0, 0, 0, 255},
	color.RGBA{0x1e, 0x1e, 0x1e, 255},
}

// readable returns an error that wraps ErrContrast when a color of t is
// hard to read. Text must have a ratio of 7 or more on each dark background
// and on Selection. The other text colors must have 4.5 or more.
func readable(t Theme) error {
	others := []struct {
		name string
		c    color.Color
	}{
		{"colors.muted", t.Muted}, {"colors.accent", t.Accent}, {"colors.error", t.Error},
		{"colors.success", t.Success}, {"colors.warning", t.Warning},
	}
	for _, bg := range darkBackgrounds {
		if r := contrast(t.Text, bg); r < minTextContrast {
			return fmt.Errorf("%w: colors.text has a ratio of %.2f, the minimum is 7", ErrContrast, r)
		}
		for _, o := range others {
			if r := contrast(o.c, bg); r < minOtherContrast {
				return fmt.Errorf("%w: %s has a ratio of %.2f, the minimum is 4.5", ErrContrast, o.name, r)
			}
		}
	}
	if r := contrast(t.Text, t.Selection); r < minTextContrast {
		return fmt.Errorf("%w: colors.text on colors.selection has a ratio of %.2f, the minimum is 7", ErrContrast, r)
	}
	return nil
}

// contrast is the WCAG 2 contrast ratio of two colors.
func contrast(a, b color.Color) float64 {
	la, lb := luminance(a), luminance(b)
	return (math.Max(la, lb) + 0.05) / (math.Min(la, lb) + 0.05)
}

// luminance is the WCAG 2 relative luminance.
func luminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	lin := func(v uint32) float64 {
		s := float64(v) / 0xffff
		if s <= 0.04045 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}
