package tui

import (
	"strconv"
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
	rowPrefetch
	rowInterface
	rowMouse
	settingsRows
)

// interfaces are the values of the interface row. "" is the modern interface.
var interfaces = []string{"", config.InterfaceBBS}

// mouseModes are the values of the mouse row. "" is on.
var mouseModes = []string{"", config.MouseOff}

// prefetchLevels are the values of the prefetch row. "" is the default: no prefetch.
var prefetchLevels = []string{"", config.PrefetchLight, config.PrefetchFull}

// settings changes the theme, the splash effect, the reduced motion, the
// language, the prefetch, the interface and the mouse. Each change applies
// at once and is saved to the config file.
type settings struct {
	d    *ui.Deps
	list ui.List
	zid  string // the prefix of the zone IDs
}

func newSettings(d *ui.Deps) *settings {
	s := &settings{d: d, zid: ui.ZonePrefix()}
	s.list.SetClock(d.Now)
	s.list.SetZones(d, s.zid+"row")
	s.list.SetLen(settingsRows)
	s.list.SetHeight(settingsRows)
	return s
}

func (s *settings) Init() tea.Cmd       { return nil }
func (s *settings) Title() string       { return i18n.T("core.settings.title") }
func (s *settings) CapturesInput() bool { return false }
func (s *settings) Animating() bool     { return s.d.Animate() && s.list.Animating(s.d.Now()) }
func (s *settings) Keys() []key.Binding {
	return append(ui.ListKeys()[:1],
		key.NewBinding(key.WithKeys("left", "right", "h", "l", "enter", "space"), key.WithHelp("← →", i18n.T("core.key.change"))))
}

func (s *settings) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	if cmd, ok := s.clickArrow(msg); ok {
		return s, cmd
	}
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

// clickArrow selects the row of a clicked arrow and changes its value.
func (s *settings) clickArrow(msg tea.Msg) (tea.Cmd, bool) {
	for r := range settingsRows {
		for _, dir := range []int{-1, 1} {
			if s.d.Clicked(s.arrowZone(dir, r), msg) {
				s.list.Select(r)
				return s.change(dir), true
			}
		}
	}
	return nil, false
}

// arrowZone is the zone ID of the arrow of row r: dir -1 is the left arrow.
func (s *settings) arrowZone(dir, row int) string {
	name := "next."
	if dir < 0 {
		name = "prev."
	}
	return s.zid + name + strconv.Itoa(row)
}

// arrows puts the value of row between the two arrows left and right.
func (s *settings) arrows(row int, left, right string) string {
	return s.d.Mark(s.arrowZone(-1, row), left) + " " + s.value(row) + " " + s.d.Mark(s.arrowZone(1, row), right)
}

func (s *settings) change(dir int) tea.Cmd {
	c := s.d.Config
	switch s.list.Cursor() {
	case rowTheme:
		c.Theme = cycle(themeIDs(s.d.Themes), s.d.Theme.ID, dir)
		s.d.SetTheme(s.d.Themes.ByID(c.Theme))
	case rowEffect:
		c.Effect = cycle(effectNames(), c.Effect, dir)
	case rowMotion:
		c.ReducedMotion = !c.ReducedMotion
	case rowLanguage:
		c.Locale = cycle(i18n.Locales(), i18n.Locale(), dir)
		i18n.SetLocale(c.Locale)
	case rowPrefetch:
		c.Prefetch = cycle(prefetchLevels, c.Prefetch, dir)
	case rowInterface:
		c.Interface = cycle(interfaces, c.Interface, dir)
	case rowMouse:
		c.Mouse = cycle(mouseModes, c.Mouse, dir)
		if s.d.Zones != nil {
			s.d.Zones.SetEnabled(c.MouseOn())
		}
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

func themeIDs(set theme.Set) []string {
	all := set.All()
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
		return themeName(s.d.Theme)
	case rowEffect:
		if c.Effect == "" {
			return i18n.T("core.settings.random")
		}
		return i18n.T("core.effect." + c.Effect)
	case rowMotion:
		return onOff(c.ReducedMotion)
	case rowMouse:
		return onOff(c.MouseOn())
	case rowPrefetch:
		return prefetchName(c)
	case rowInterface:
		if c.BBS() {
			return i18n.T("bbs.interface.bbs")
		}
		return i18n.T("bbs.interface.modern")
	}
	return i18n.LanguageName(i18n.Locale())
}

func onOff(on bool) string {
	if on {
		return i18n.T("core.settings.on")
	}
	return i18n.T("core.settings.off")
}

// prefetchName returns the name of the prefetch level. An unknown value is off.
func prefetchName(c *config.Config) string {
	switch {
	case c.FullPrefetch():
		return i18n.T("prefetch.level.full")
	case c.LightPrefetch():
		return i18n.T("prefetch.level.light")
	}
	return i18n.T("prefetch.level.off")
}

// themeName returns the theme name with a mark for a user theme.
func themeName(t theme.Theme) string {
	if t.Custom {
		return i18n.Tf("core.settings.custom", map[string]any{"Name": t.Name})
	}
	return t.Name
}

func settingLabel(row int) string {
	return i18n.T([]string{
		"core.settings.theme", "core.settings.effect", "core.settings.motion", "core.settings.language",
		"prefetch.settings.label", "bbs.settings.interface", "core.settings.mouse",
	}[row])
}

func (s *settings) View(width, height int) string {
	if s.d.BBS() {
		return s.bbsView(width)
	}
	st, t := s.d.Styles, s.d.Theme
	labelW := 0
	for r := range settingsRows {
		labelW = max(labelW, ansi.StringWidth(settingLabel(r)))
	}
	rows := s.list.View(st, t, min(width, 64), s.d.Phase(), func(i, w int) string {
		label := settingLabel(i)
		return label + strings.Repeat(" ", labelW-ansi.StringWidth(label)+3) + s.arrows(i, "◀", "▶")
	})
	parts := []string{
		ui.Header(st, t, s.Title(), width), "", rows, "",
		st.Muted.Render(lipgloss.Wrap(i18n.T("core.settings.hint"), width, " ")),
		st.Muted.Render(lipgloss.Wrap(i18n.T("prefetch.settings.note"), width, " ")), "",
		ui.Header(st, t, i18n.T("core.settings.preview"), width),
	}
	// The path has its own line, so it breaks only when it is wider than the screen.
	footer := st.Muted.Render(i18n.T("core.settings.themesDir") + "\n" + lipgloss.Wrap(s.d.ThemesDir, width, ""))
	used := lipgloss.Height(strings.Join(parts, "\n")) + 1 + lipgloss.Height(footer)
	if ph := height - used; ph >= 3 {
		parts = append(parts, s.preview(width, min(ph, 9)))
	}
	return strings.Join(append(parts, "", footer), "\n")
}

// preview paints the chosen effect with the theme name on a plate.
func (s *settings) preview(width, height int) string {
	e, ok := fx.ByName(splashEffect(s.d), s.d.Theme.Splash)
	if !ok {
		e = fx.All(s.d.Theme.Splash)[0]
	}
	t := staticTime
	if s.d.Animate() {
		t = s.d.AnimT
	}
	return fx.Render(e, width, height, t, []string{" " + s.d.Theme.Name + " "}, fx.RenderOptions{ASCII: s.d.Plain()})
}

// Prompt is the label of the BBS prompt.
func (s *settings) Prompt() string { return i18n.T("bbs.prompt.settings") }

// Commands are the BBS commands: a number selects a row, Enter and > give
// the next value and < gives the previous value.
func (s *settings) Commands() []ui.Command {
	return []ui.Command{
		{Name: "#", Label: i18n.T("bbs.settings.pick"), Run: s.pick},
		{Name: "", Label: i18n.T("bbs.settings.change"), Run: func(string) tea.Cmd { return s.change(1) }},
		{Name: "<", Label: i18n.T("bbs.settings.prev"), Run: func(string) tea.Cmd { return s.change(-1) }},
		{Name: ">", Label: i18n.T("bbs.settings.next"), Run: func(string) tea.Cmd { return s.change(1) }},
		{Name: "V", Label: i18n.T("bbs.global.back")},
		{Name: "?", Label: i18n.T("bbs.global.help")},
	}
}

// pick selects the row with the number arg.
func (s *settings) pick(arg string) tea.Cmd {
	n, err := strconv.Atoi(arg)
	if err != nil || n < 1 || n > settingsRows {
		return ui.Toast(i18n.T("bbs.unknown"), ui.ToastError)
	}
	s.list.Select(n - 1)
	return nil
}

// bbsView shows the settings in an ASCII table with a number for each row.
func (s *settings) bbsView(width int) string {
	st := s.d.Styles
	labelW := 0
	for r := range settingsRows {
		labelW = max(labelW, ansi.StringWidth(settingLabel(r)))
	}
	t := ui.NewTable(width, ui.Column{Title: i18n.T("bbs.help.number"), Width: 2, Align: ui.AlignRight},
		ui.Column{Title: i18n.T("bbs.settings.option"), Width: labelW}, ui.Column{Title: i18n.T("bbs.settings.value")})
	t.Full(st.Subtitle.Render(s.Title())).Header()
	for r := range settingsRows {
		t.Row(ui.Num(r+1, settingsRows), settingLabel(r), s.arrows(r, "<", ">")).Zone(s.d, s.list.RowZone(r))
		if r == s.list.Cursor() {
			t.Select()
		}
	}
	t.Wrap(i18n.T("core.settings.hint")).Wrap(i18n.T("prefetch.settings.note"))
	return t.Render(st)
}
