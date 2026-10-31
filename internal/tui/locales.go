package tui

import (
	"errors"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

type localesLoadedMsg struct{ problems []i18n.Problem }

// localeReasons are the catalog IDs of the translation file errors.
var localeReasons = []struct {
	err error
	id  string
}{
	{i18n.ErrRead, "locales.err.read"},
	{i18n.ErrSyntax, "locales.err.syntax"},
	{i18n.ErrLanguage, "locales.err.language"},
}

// loadLocales reads the user translation files outside the UI loop.
func (m *model) loadLocales() tea.Cmd {
	dir := m.d.LocalesDir
	if dir == "" {
		return nil
	}
	return func() tea.Msg { return localesLoadedMsg{problems: i18n.LoadDir(dir)} }
}

// updateLocales sets the language of the run again, because it can be a
// user language that only now exists. One toast tells about the bad files.
func (m *model) updateLocales(msg tea.Msg) (tea.Cmd, bool) {
	l, ok := msg.(localesLoadedMsg)
	if !ok {
		return nil, false
	}
	want := m.opts.Locale
	if want == "" {
		want = m.d.Config.Locale
	}
	i18n.SetLocale(want)
	if len(l.problems) == 0 {
		return nil, true
	}
	return ui.Toast(localeProblemsText(l.problems), ui.ToastError), true
}

func localeProblemsText(problems []i18n.Problem) string {
	parts := make([]string, 0, maxProblemFiles+1)
	for i, p := range problems {
		if i == maxProblemFiles {
			parts = append(parts, "…")
			break
		}
		parts = append(parts, p.File+": "+localeReason(p.Err))
	}
	return i18n.Plural("locales.loadFailed", len(problems), map[string]any{"Files": strings.Join(parts, "; ")})
}

func localeReason(err error) string {
	for _, r := range localeReasons {
		if errors.Is(err, r.err) {
			return i18n.T(r.id)
		}
	}
	return err.Error()
}
