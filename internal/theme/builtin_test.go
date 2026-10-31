package theme

import (
	"image/color"
	"io/fs"
	"testing"

	colorful "github.com/lucasb-eyer/go-colorful"
)

func TestEveryBuiltinFileParses(t *testing.T) {
	files, err := fs.Glob(builtinFS, "builtin/*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != len(builtinIDs) {
		t.Errorf("got %d built-in files, want %d", len(files), len(builtinIDs))
	}
	for _, id := range builtinIDs {
		data, err := builtinFS.ReadFile("builtin/" + id + ".json")
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		th, err := parse(id, data)
		if err != nil {
			t.Errorf("%s: %v", id, err)
		}
		if th.Custom || th.BorderStyle != "double" || th.BannerFont != "" || th.Effect != "" {
			t.Errorf("%s: options = %v %q %q %q, want the defaults", id, th.Custom, th.BorderStyle, th.BannerFont, th.Effect)
		}
	}
}

func TestBuiltinKeyColors(t *testing.T) {
	cases := []struct {
		id    string
		field string
		got   func(Theme) color.Color
		want  string
	}{
		{"habana-neon", "Text", func(t Theme) color.Color { return t.Text }, "#f7ecff"},
		{"habana-neon", "Accent", func(t Theme) color.Color { return t.Accent }, "#ff4fd8"},
		{"habana-neon", "Splash[3]", func(t Theme) color.Color { return t.Splash[3] }, "#00e5ff"},
		{"ambar-crt", "Accent", func(t Theme) color.Color { return t.Accent }, "#ffb000"},
		{"fosforo-verde", "Selection", func(t Theme) color.Color { return t.Selection }, "#0f3d1e"},
		{"malecon", "Border[2]", func(t Theme) color.Color { return t.Border[2] }, "#ff8a5b"},
		{"malecon", "Splash[4]", func(t Theme) color.Color { return t.Splash[4] }, "#ff6f91"},
	}
	for _, c := range cases {
		got, _ := colorful.MakeColor(c.got(ByID(c.id)))
		if got.Hex() != c.want {
			t.Errorf("%s %s = %s, want %s", c.id, c.field, got.Hex(), c.want)
		}
	}
}
