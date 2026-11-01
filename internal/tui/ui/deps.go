// Package ui holds the parts that all screens of the terminal client share:
// the dependencies, the Screen interface, the navigation messages and the
// shared widgets.
package ui

import (
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	zone "github.com/lrstanley/bubblezone/v2"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/api/gen"
	"github.com/albertgmz/sermo-cli/internal/auth"
	"github.com/albertgmz/sermo-cli/internal/config"
	"github.com/albertgmz/sermo-cli/internal/render"
	"github.com/albertgmz/sermo-cli/internal/theme"
)

// MaxReadingWidth is the maximum width in columns of post text.
const MaxReadingWidth = 100

// Deps holds the state that all screens share. The app owns it and changes
// it, for example after a sign-in or a theme change. Screens keep the pointer
// and read the fields when they need them.
type Deps struct {
	API     *api.Client
	Cred    auth.Credential
	Store   *auth.Store
	Server  string
	Config  *config.Config
	Theme   theme.Theme
	Styles  Styles
	Render  *render.Renderer
	Me      *gen.SelfUser // nil for a guest
	NoAnim  bool
	Profile colorprofile.Profile
	// Glamour is the Glamour style for post text: "dark", "light" or "notty".
	Glamour string
	// AnimT is the time of the animation clock in seconds.
	AnimT float64
	// Now gives the current time. Tests set a fixed clock.
	Now func() time.Time
	// Themes are the themes to select from. The zero Set holds the
	// built-in themes.
	Themes theme.Set
	// ThemesDir is the directory of the user theme files.
	ThemesDir string
	// LocalesDir is the directory of the user translation files.
	LocalesDir string
	// Zones finds the clickable parts of the views. Nil means no clicks.
	Zones *zone.Manager
}

// Animate reports whether the views can animate. It is false with
// --no-anim, with reduced motion or in the BBS interface.
func (d *Deps) Animate() bool {
	return d.Motion() && !d.BBS()
}

// Motion reports whether the user allows motion. It is false with
// --no-anim or with reduced motion. The BBS splash uses it for its text.
func (d *Deps) Motion() bool {
	return !d.NoAnim && (d.Config == nil || !d.Config.ReducedMotion)
}

// BBS reports whether the config selects the BBS interface.
func (d *Deps) BBS() bool {
	return d.Config != nil && d.Config.BBS()
}

// Phase returns the color phase for animated highlights. It is 0 when the
// views do not animate.
func (d *Deps) Phase() float64 {
	if !d.Animate() {
		return 0
	}
	return d.AnimT * 0.35
}

// Plain reports whether the terminal shows no color.
func (d *Deps) Plain() bool {
	return d.Profile == colorprofile.NoTTY || d.Profile == colorprofile.ASCII
}

// SetTheme sets the theme and makes the styles again.
func (d *Deps) SetTheme(t theme.Theme) {
	d.Theme = t
	d.Styles = NewStyles(t)
}

// ReadingWidth returns the width for post text in a content area of width
// columns.
func ReadingWidth(width int) int {
	return min(width, MaxReadingWidth)
}

// Screen is one view in the content area. The app draws the chrome around
// it.
type Screen interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (Screen, tea.Cmd)
	// View returns the content area only. The app draws the chrome.
	View(width, height int) string
	// Title is the breadcrumb segment of the screen.
	Title() string
	// Keys are the keys of the screen for the help screen.
	Keys() []key.Binding
	// CapturesInput is true while a text field has focus. Then the app
	// does not use the global hotkeys.
	CapturesInput() bool
}

// Animator is an optional interface of a Screen. While Animating returns
// true, the app sends AnimTickMsg at 30 frames a second or less. When no
// view animates, the app sends no ticks.
type Animator interface {
	Animating() bool
}
