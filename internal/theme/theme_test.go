package theme

import (
	"image/color"
	"math"
	"testing"
)

var darkBackgrounds = []color.Color{
	color.RGBA{0, 0, 0, 255},
	color.RGBA{0x1e, 0x1e, 0x1e, 255},
}

func TestAllThemes(t *testing.T) {
	want := map[string]string{
		"habana-neon":   "Habana neón",
		"ambar-crt":     "Ámbar CRT",
		"fosforo-verde": "Fósforo verde",
		"malecon":       "Malecón",
	}
	all := All()
	if len(all) != len(want) {
		t.Fatalf("got %d themes, want %d", len(all), len(want))
	}
	if all[0].ID != "habana-neon" {
		t.Errorf("first theme = %q, want the default habana-neon", all[0].ID)
	}
	seen := map[string]bool{}
	for _, th := range all {
		if seen[th.ID] {
			t.Errorf("duplicate ID %q", th.ID)
		}
		seen[th.ID] = true
		if want[th.ID] != th.Name {
			t.Errorf("theme %q name = %q, want %q", th.ID, th.Name, want[th.ID])
		}
	}
}

func TestByID(t *testing.T) {
	if got := ByID("malecon"); got.ID != "malecon" {
		t.Errorf("ByID(malecon).ID = %q", got.ID)
	}
	for _, id := range []string{"", "nope"} {
		if got := ByID(id); got.ID != "habana-neon" {
			t.Errorf("ByID(%q).ID = %q, want habana-neon", id, got.ID)
		}
	}
}

func TestThemesHaveColors(t *testing.T) {
	for _, th := range All() {
		for name, c := range map[string]color.Color{
			"Text": th.Text, "Muted": th.Muted, "Accent": th.Accent, "Error": th.Error,
			"Success": th.Success, "Warning": th.Warning, "Selection": th.Selection,
		} {
			if c == nil {
				t.Errorf("%s: %s is nil", th.ID, name)
			}
		}
		for name, g := range map[string]int{
			"Border": len(th.Border), "Title": len(th.Title), "Highlight": len(th.Highlight), "Splash": len(th.Splash),
		} {
			if g < 2 {
				t.Errorf("%s: %s gradient has %d stops, want 2 or more", th.ID, name, g)
			}
		}
	}
}

func TestTextContrast(t *testing.T) {
	for _, th := range All() {
		for _, bg := range darkBackgrounds {
			if r := contrast(th.Text, bg); r < 7 {
				t.Errorf("%s: Text contrast %.2f on %v, want 7 or more", th.ID, r, bg)
			}
			for name, c := range map[string]color.Color{
				"Muted": th.Muted, "Accent": th.Accent, "Error": th.Error,
				"Success": th.Success, "Warning": th.Warning,
			} {
				if r := contrast(c, bg); r < 4.5 {
					t.Errorf("%s: %s contrast %.2f on %v, want 4.5 or more", th.ID, name, r, bg)
				}
			}
		}
		if r := contrast(th.Text, th.Selection); r < 7 {
			t.Errorf("%s: Text contrast %.2f on Selection, want 7 or more", th.ID, r)
		}
	}
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
