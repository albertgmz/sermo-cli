// Package social holds the screens for notifications, conversations,
// profiles and search.
package social

import (
	"context"
	"hash/fnv"
	"html"
	"image/color"
	"regexp"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	colorful "github.com/lucasb-eyer/go-colorful"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// pageSize is the number of items that a screen asks for in one request.
const pageSize = 20

// bg is the context of the API calls. The app has no cancel for a screen.
var bg = context.Background()

// relTime returns the time t relative to the clock of d, for example
// "hace 5 min". Times older than a week show the date.
func relTime(d *ui.Deps, t time.Time) string {
	now := time.Now()
	if d.Now != nil {
		now = d.Now()
	}
	ago := now.Sub(t)
	switch {
	case ago < time.Minute:
		return i18n.T("social.time.now")
	case ago < time.Hour:
		return i18n.Tf("social.time.minutes", map[string]any{"N": int(ago.Minutes())})
	case ago < 24*time.Hour:
		return i18n.Tf("social.time.hours", map[string]any{"N": int(ago.Hours())})
	case ago < 7*24*time.Hour:
		return i18n.Tf("social.time.days", map[string]any{"N": int(ago.Hours() / 24)})
	}
	return date(t)
}

// date returns the day of t in the format of the current language.
func date(t time.Time) string {
	return t.Local().Format(i18n.T("social.time.layout"))
}

var tagRE = regexp.MustCompile(`<[^>]*>`)

// plainText returns the text of an HTML fragment on one line.
func plainText(s string) string {
	s = tagRE.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(html.UnescapeString(s)), " ")
}

// nameColor returns the color of a username: the hex color of the server
// when it has one, else a color of the highlight gradient that the name
// picks.
func nameColor(d *ui.Deps, name string, hex *string) color.Color {
	if hex != nil {
		if c, err := colorful.Hex(*hex); err == nil {
			return c
		}
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return d.Theme.Highlight.At(float64(h.Sum32()%1000) / 999)
}

// userName returns name in its color and bold.
func userName(d *ui.Deps, name string, hex *string) string {
	if d.Plain() {
		return name
	}
	return lipgloss.NewStyle().Bold(true).Foreground(nameColor(d, name, hex)).Render(name)
}

// loader holds the load state that all screens share.
type loader struct {
	d       *ui.Deps
	sp      spinner.Model
	loading bool
	err     error
}

func newLoader(d *ui.Deps) loader {
	return loader{d: d, sp: ui.NewSpinner(d.Styles), loading: true}
}

// begin sets the loading state and returns the spinner tick when the views
// animate.
func (l *loader) begin() tea.Cmd {
	l.loading, l.err = true, nil
	if l.d.Animate() {
		return l.sp.Tick
	}
	return nil
}

// done ends the loading state. A 401 gives the Cmd of ui.ErrorCmd.
func (l *loader) done(err error) tea.Cmd {
	l.loading, l.err = false, err
	return unauthorized(err)
}

// blocked reports whether a key must wait for the data. After an error
// the retry key runs retry.
func (l *loader) blocked(msg tea.KeyPressMsg, retry func() tea.Cmd) (tea.Cmd, bool) {
	if l.loading {
		return nil, true
	}
	if l.err == nil {
		return nil, false
	}
	if key.Matches(msg, retryKey) {
		return retry(), true
	}
	return nil, true
}

// spin moves the spinner while it loads.
func (l *loader) spin(msg spinner.TickMsg) tea.Cmd {
	if !l.loading || !l.d.Animate() {
		return nil
	}
	var cmd tea.Cmd
	l.sp, cmd = l.sp.Update(msg)
	return cmd
}

// view returns the loading or error view and true, or false when the data
// is ready.
func (l *loader) view(width, height int) (string, bool) {
	if l.loading {
		return ui.Loading(l.d.Styles, l.sp, "", width, height), true
	}
	if l.err != nil {
		s := l.d.Styles
		text := s.Error.Render(ui.ErrorText(l.err)) + "\n\n" + s.Muted.Render(i18n.T("social.retry"))
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, lipgloss.Wrap(text, width, " ")), true
	}
	return "", false
}

// unauthorized returns the Cmd of ui.ErrorCmd for a 401 and nil for other
// errors, which the view shows.
func unauthorized(err error) tea.Cmd {
	if api.IsUnauthorized(err) {
		return ui.ErrorCmd(err)
	}
	return nil
}

// loginNeeded is the toast for a guest who tries a member action.
func loginNeeded() tea.Cmd {
	return ui.Toast(i18n.T("social.loginNeeded"), ui.ToastInfo)
}

// login asks the app for the sign-in screen.
func login() tea.Msg { return ui.LoginMsg{} }

// binding returns a key binding with help text from the catalog.
func binding(help, id string, keys ...string) key.Binding {
	return key.NewBinding(key.WithKeys(keys...), key.WithHelp(help, i18n.T(id)))
}

// retryKey is the key that loads the data again after an error.
var retryKey = key.NewBinding(key.WithKeys("r", "R"))

// pad returns lines with blank lines added to fill height rows.
func pad(lines []string, height int) string {
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}
