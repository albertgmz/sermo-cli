package tui

import (
	"errors"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/theme"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// maxProblemFiles is the number of bad theme files that the toast names.
const maxProblemFiles = 3

type themesLoadedMsg struct {
	set      theme.Set
	problems []theme.Problem
}

// problemReasons are the catalog IDs of the theme file errors.
var problemReasons = []struct {
	err error
	id  string
}{
	{theme.ErrRead, "core.themes.err.read"},
	{theme.ErrTooLarge, "core.themes.err.tooLarge"},
	{theme.ErrSyntax, "core.themes.err.syntax"},
	{theme.ErrID, "core.themes.err.id"},
	{theme.ErrMissing, "core.themes.err.missing"},
	{theme.ErrColor, "core.themes.err.color"},
	{theme.ErrGradient, "core.themes.err.gradient"},
	{theme.ErrBorder, "core.themes.err.border"},
	{theme.ErrFont, "core.themes.err.font"},
	{theme.ErrEffect, "core.themes.err.effect"},
	{theme.ErrContrast, "core.themes.err.contrast"},
}

// loadThemes reads the user theme files outside the UI loop.
func (m *model) loadThemes() tea.Cmd {
	dir := m.d.ThemesDir
	if dir == "" {
		return nil
	}
	return func() tea.Msg {
		set, problems := theme.Load(dir)
		return themesLoadedMsg{set: set, problems: problems}
	}
}

// updateThemes uses the loaded themes and applies the theme of the config.
// One toast tells about the bad files.
func (m *model) updateThemes(msg tea.Msg) (tea.Cmd, bool) {
	l, ok := msg.(themesLoadedMsg)
	if !ok {
		return nil, false
	}
	m.d.Themes = l.set
	m.d.SetTheme(l.set.ByID(m.d.Config.Theme))
	if m.splash != nil {
		m.splash.retheme()
	}
	if len(l.problems) == 0 {
		return nil, true
	}
	return ui.Toast(problemsText(l.problems), ui.ToastError), true
}

func problemsText(problems []theme.Problem) string {
	parts := make([]string, 0, maxProblemFiles+1)
	for i, p := range problems {
		if i == maxProblemFiles {
			parts = append(parts, "…")
			break
		}
		parts = append(parts, p.File+": "+problemReason(p.Err))
	}
	return i18n.Plural("core.themes.loadFailed", len(problems), map[string]any{"Files": strings.Join(parts, "; ")})
}

func problemReason(err error) string {
	for _, r := range problemReasons {
		if errors.Is(err, r.err) {
			return i18n.T(r.id)
		}
	}
	return err.Error()
}
