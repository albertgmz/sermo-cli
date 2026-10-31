// Package tui is the full screen terminal client. It holds the root app
// model, the chrome around the screens, the splash, the sign-in, the
// settings and the help screens, and the router between screens.
package tui

import (
	"context"
	"math/rand/v2"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/albertgmz/sermo-cli/internal/auth"
	"github.com/albertgmz/sermo-cli/internal/config"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/render"
	"github.com/albertgmz/sermo-cli/internal/tui/forum"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// maxFPS is the frame rate limit of the renderer and of the animation clock.
const maxFPS = 30

// Start selects the first view.
type Start int

// The first views.
const (
	StartSplash Start = iota
	StartHome
)

// Options change how Run starts.
type Options struct {
	Start Start
	// Sleep waits between the polls of the device login. Nil uses a timer.
	Sleep func(context.Context, time.Duration) error
	// Locale sets the language for this run only. It is not saved. Empty
	// uses d.Config.Locale.
	Locale string
}

// Run shows the terminal client until the user quits or ctx ends. Run sets
// the theme from d.Config, the color profile of the terminal and the
// defaults of the empty fields of d.
func Run(ctx context.Context, d ui.Deps, opts Options) error {
	d.Profile = colorprofile.Detect(os.Stdout, os.Environ())
	m := newModel(ctx, &d, opts)
	_, err := tea.NewProgram(m, tea.WithContext(ctx), tea.WithFPS(maxFPS)).Run()
	return err
}

func newModel(ctx context.Context, d *ui.Deps, opts Options) *model {
	if d.Config == nil {
		d.Config = &config.Config{}
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Store == nil {
		d.Store = auth.NewStore()
	}
	if d.Render == nil {
		d.Render = render.New(d.Server)
	}
	i18n.SetLocale(d.Config.Locale)
	if opts.Locale != "" {
		i18n.SetLocale(opts.Locale)
	}
	if d.ThemesDir == "" {
		d.ThemesDir, _ = config.ThemesDir()
	}
	d.SetTheme(d.Themes.ByID(d.Config.Theme))
	d.Glamour = glamourStyle(d, true)
	m := &model{
		ctx:   ctx,
		d:     d,
		opts:  opts,
		start: time.Now(),
		site:  hostOf(d.Server),
		slide: newToastSlide(),
		stack: []ui.Screen{forum.NewNodes(d)},
	}
	if opts.Start == StartSplash {
		m.splash = newSplash(d, rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 1)))
	}
	return m
}

func glamourStyle(d *ui.Deps, dark bool) string {
	switch {
	case d.Plain():
		return "notty"
	case dark:
		return "dark"
	}
	return "light"
}
