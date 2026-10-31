// Package theme holds the color themes of the client.
package theme

import (
	"image/color"

	"github.com/albertgmz/sermo-cli/internal/fx"
	colorful "github.com/lucasb-eyer/go-colorful"
)

// Theme is a set of colors for the text and a set of gradients for the
// effects. Text has a contrast ratio of 7 or more on a dark background.
type Theme struct {
	ID, Name string

	Text, Muted, Accent, Error, Success, Warning, Selection color.Color

	Border, Title, Highlight, Splash fx.Gradient
}

// All returns the themes. The first theme is the default.
func All() []Theme {
	return []Theme{habanaNeon(), ambarCRT(), fosforoVerde(), malecon()}
}

// ByID returns the theme with the given ID. It returns the default theme
// when no theme has that ID.
func ByID(id string) Theme {
	all := All()
	for _, t := range all {
		if t.ID == id {
			return t
		}
	}
	return all[0]
}

func habanaNeon() Theme {
	return Theme{
		ID: "habana-neon", Name: "Habana neón",
		Text: hex("#F7ECFF"), Muted: hex("#B3A3CF"), Accent: hex("#FF4FD8"),
		Error: hex("#FF5C7A"), Success: hex("#5CFFB0"), Warning: hex("#FFD166"),
		Selection: hex("#3A1D5C"),
		Border:    grad("#FF2E97", "#B026FF", "#00E5FF"),
		Title:     grad("#FF6EC7", "#FF2E97", "#00F0FF"),
		Highlight: grad("#00E5FF", "#FF4FD8"),
		Splash:    grad("#12002B", "#7A00FF", "#FF2E97", "#00E5FF"),
	}
}

func ambarCRT() Theme {
	return Theme{
		ID: "ambar-crt", Name: "Ámbar CRT",
		Text: hex("#FFDDA8"), Muted: hex("#C99D5E"), Accent: hex("#FFB000"),
		Error: hex("#FF6A3D"), Success: hex("#E8D05A"), Warning: hex("#FFCC66"),
		Selection: hex("#4A2E00"),
		Border:    grad("#7A4A00", "#FFB000", "#FFE0A0"),
		Title:     grad("#FF9500", "#FFB000", "#FFE0A0"),
		Highlight: grad("#FF9500", "#FFD27A"),
		Splash:    grad("#1A0E00", "#6B3E00", "#FFB000", "#FFE4B0"),
	}
}

func fosforoVerde() Theme {
	return Theme{
		ID: "fosforo-verde", Name: "Fósforo verde",
		Text: hex("#CCFFD8"), Muted: hex("#74C48A"), Accent: hex("#33FF66"),
		Error: hex("#FF7A70"), Success: hex("#7CFF9E"), Warning: hex("#E6FF66"),
		Selection: hex("#0F3D1E"),
		Border:    grad("#0A5C2A", "#33FF66", "#B3FFC8"),
		Title:     grad("#33FF66", "#B3FFC8"),
		Highlight: grad("#00FF88", "#CCFF66"),
		Splash:    grad("#001A08", "#00662A", "#33FF66", "#D6FFE0"),
	}
}

func malecon() Theme {
	return Theme{
		ID: "malecon", Name: "Malecón",
		Text: hex("#EAF6FF"), Muted: hex("#93B6CF"), Accent: hex("#FF8A5B"),
		Error: hex("#FF5E62"), Success: hex("#4FE3C1"), Warning: hex("#FFC15E"),
		Selection: hex("#12355B"),
		Border:    grad("#0B4F8A", "#1FA2D6", "#FF8A5B"),
		Title:     grad("#FFB26B", "#FF7F6B", "#FF6F91"),
		Highlight: grad("#4FC3F7", "#FF8A5B"),
		Splash:    grad("#031A33", "#0B4F8A", "#1FA2D6", "#FF8A5B", "#FF6F91"),
	}
}

func grad(stops ...string) fx.Gradient {
	g := make(fx.Gradient, len(stops))
	for i, s := range stops {
		g[i] = hex(s)
	}
	return g
}

// hex parses a color constant. It panics on a bad constant because the
// constants are fixed in this file and the tests parse each one.
func hex(s string) colorful.Color {
	c, err := colorful.Hex(s)
	if err != nil {
		panic(err)
	}
	return c
}
