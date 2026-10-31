package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/albertgmz/sermo-cli/internal/config"
	"github.com/albertgmz/sermo-cli/internal/fx"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/theme"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

const (
	rowTheme = iota
	rowEffect
	rowMotion
	rowLanguage
	settingsRows
)

var locales = []string{"es", "en"}

// settings changes the theme, the splash effect, the reduced motion and the
// language. Each change applies at once and is saved to the config file.
type settings struct {
	d    *ui.Deps
	list ui.List
}

func newSettings(d *ui.Deps) *settings {
	s := &settings{d: d}
	s.list.SetLen(settingsRows)
	s.list.SetHeight(settingsRows)
	return s
}

func (s *settings) Init() tea.Cmd       { return nil }
func (s *settings) Title() string       { return i18n.T("core.settings.title") }
func (s *settings) CapturesInput() bool { return false }
func (s *settings) Animating() bool     { return s.d.Animate() }
func (s *settings) Keys() []key.Binding {
	return append(ui.ListKeys()[:1],
		key.NewBinding(key.WithKeys("left", "right", "h", "l", "enter", "space"), key.WithHelp("← →", i18n.T("core.key.change"))))
}

func (s *settings) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		s.list.Update(msg)
		return s, nil
	}
	switch k.String() {
	case "left", "h":
		return s, s.change(-1)
	case "right", "l", "enter", "space":
		return s, s.change(1)
	}
	s.list.Update(msg)
	return s, nil
}

func (s *settings) change(dir int) tea.Cmd {
	c := s.d.Config
	switch s.list.Cursor() {
	case rowTheme:
		c.Theme = cycle(themeIDs(), s.d.Theme.ID, dir)
		s.d.SetTheme(theme.ByID(c.Theme))
	case rowEffect:
		c.Effect = cycle(effectNames(), c.Effect, dir)
	case rowMotion:
		c.ReducedMotion = !c.ReducedMotion
	case rowLanguage:
		c.Locale = cycle(locales, i18n.Locale(), dir)
		i18n.SetLocale(c.Locale)
	}
	if err := config.Save(*c); err != nil {
		return ui.Toast(i18n.T("core.toast.saveFailed"), ui.ToastError)
	}
	return nil
}

// cycle returns the value after cur in values, or before it when dir is
// negative. An unknown cur starts at the first value.
func cycle(values []string, cur string, dir int) string {
	i := 0
	for j, v := range values {
		if v == cur {
			i = j
		}
	}
	n := len(values)
	return values[((i+dir)%n+n)%n]
}

func themeIDs() []string {
	all := theme.All()
	ids := make([]string, len(all))
	for i, t := range all {
		ids[i] = t.ID
	}
	return ids
}

// effectNames returns "" for a random effect and then the effect names.
func effectNames() []string {
	names := []string{""}
	for _, e := range fx.All(theme.ByID("").Splash) {
		names = append(names, e.Name)
	}
	return names
}

func (s *settings) value(row int) string {
	c := s.d.Config
	switch row {
	case rowTheme:
		return s.d.Theme.Name
	case rowEffect:
		if c.Effect == "" {
			return i18n.T("core.settings.random")
		}
		return i18n.T("core.effect." + c.Effect)
	case rowMotion:
		if c.ReducedMotion {
			return i18n.T("core.settings.on")
		}
		return i18n.T("core.settings.off")
	}
	return i18n.T("core.lang." + i18n.Locale())
}

func settingLabel(row int) string {
	return i18n.T([]string{"core.settings.theme", "core.settings.effect", "core.settings.motion", "core.settings.language"}[row])
}

func (s *settings) View(width, height int) string {
	st, t := s.d.Styles, s.d.Theme
	labelW := 0
	for r := range settingsRows {
		labelW = max(labelW, ansi.StringWidth(settingLabel(r)))
	}
	rows := s.list.View(st, t, min(width, 64), s.d.Phase(), func(i, w int) string {
		label := settingLabel(i)
		return label + strings.Repeat(" ", labelW-ansi.StringWidth(label)+3) + "◀ " + s.value(i) + " ▶"
	})
	parts := []string{
		ui.Header(st, t, s.Title(), width), "", rows, "",
		st.Muted.Render(lipgloss.Wrap(i18n.T("core.settings.hint"), width, " ")), "",
		ui.Header(st, t, i18n.T("core.settings.preview"), width),
	}
	used := lipgloss.Height(strings.Join(parts, "\n"))
	if ph := height - used; ph >= 3 {
		parts = append(parts, s.preview(width, min(ph, 9)))
	}
	return strings.Join(parts, "\n")
}

// preview paints the chosen effect with the theme name on a plate.
func (s *settings) preview(width, height int) string {
	e, ok := fx.ByName(s.d.Config.Effect, s.d.Theme.Splash)
	if !ok {
		e = fx.All(s.d.Theme.Splash)[0]
	}
	t := staticTime
	if s.d.Animate() {
		t = s.d.AnimT
	}
	return fx.Render(e, width, height, t, []string{" " + s.d.Theme.Name + " "}, fx.RenderOptions{ASCII: s.d.Plain()})
}
