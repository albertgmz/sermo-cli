// Package theme holds the color themes of the client. The built-in themes
// are JSON files in the builtin directory. Users can add theme files of the
// same format to a directory, see Load.
package theme

import (
	"embed"
	"fmt"
	"image/color"
	"slices"

	"github.com/albertgmz/sermo-cli/internal/fx"
)

// Theme is a set of colors for the text and a set of gradients for the
// effects. Text has a contrast ratio of 7 or more on a dark background.
type Theme struct {
	ID, Name string

	Text, Muted, Accent, Error, Success, Warning, Selection color.Color

	Border, Title, Highlight, Splash fx.Gradient

	// BorderStyle is the line style of the frame: "double", "normal",
	// "rounded", "thick" or "ascii".
	BorderStyle string
	// BannerFont is the go-figure font of the splash banner. Empty means
	// the default fonts.
	BannerFont string
	// Effect is the splash effect when the config sets no effect. Empty
	// means a random effect.
	Effect string
	// Custom is true for a theme from a user file.
	Custom bool
}

//go:embed builtin/*.json
var builtinFS embed.FS

// builtinIDs are the IDs of the built-in themes in their order. The first
// theme is the default.
var builtinIDs = []string{"habana-neon", "ambar-crt", "fosforo-verde", "malecon"}

var builtins = mustBuiltins()

// mustBuiltins parses the embedded theme files. A bad file is an error in
// the program, thus it panics.
func mustBuiltins() []Theme {
	out := make([]Theme, len(builtinIDs))
	for i, id := range builtinIDs {
		data, err := builtinFS.ReadFile("builtin/" + id + ".json")
		if err == nil {
			out[i], err = parse(id, data)
		}
		if err != nil {
			panic(fmt.Sprintf("theme: built-in %s: %v", id, err))
		}
	}
	return out
}

// Set is an ordered list of themes. The zero Set holds the built-in themes.
type Set struct {
	themes []Theme
}

// Builtin returns the set of the built-in themes.
func Builtin() Set {
	return Set{themes: slices.Clone(builtins)}
}

// All returns the themes. The first theme is the default.
func (s Set) All() []Theme {
	if len(s.themes) == 0 {
		return slices.Clone(builtins)
	}
	return slices.Clone(s.themes)
}

// ByID returns the theme with the given ID. It returns the default theme
// when no theme has that ID.
func (s Set) ByID(id string) Theme {
	all := s.All()
	for _, t := range all {
		if t.ID == id {
			return t
		}
	}
	return all[0]
}

// All returns the built-in themes. The first theme is the default.
func All() []Theme {
	return Builtin().All()
}

// ByID returns the built-in theme with the given ID. It returns the default
// theme when no built-in theme has that ID.
func ByID(id string) Theme {
	return Builtin().ByID(id)
}
