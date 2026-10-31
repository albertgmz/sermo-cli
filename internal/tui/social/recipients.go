package social

import (
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

const (
	// debounce is the time without a key before a user search starts.
	debounce = 300 * time.Millisecond
	// minPrefix is the shortest prefix that starts a user search.
	minPrefix = 2
	// maxSuggestions is the number of users that a search shows.
	maxSuggestions = 5
)

type debounceMsg struct {
	seq    int
	prefix string
}

type suggestMsg struct {
	seq   int
	users []api.UserMatch
	err   error
}

// recipients is a field for the members of a new conversation. The member
// types a part of a username, chooses a suggestion, and the user becomes a
// chip.
type recipients struct {
	d      *ui.Deps
	in     textinput.Model
	chips  []api.UserMatch
	sugg   []api.UserMatch
	cursor int
	seq    int
}

func newRecipients(d *ui.Deps) *recipients {
	return &recipients{d: d, in: newInput(d)}
}

// ids returns the user IDs of the chips.
func (r *recipients) ids() []int {
	out := make([]int, len(r.chips))
	for i, u := range r.chips {
		out[i] = u.Id
	}
	return out
}

func (r *recipients) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return r.key(msg)
	case tea.PasteMsg:
		return r.input(msg)
	case debounceMsg:
		return r.search(msg)
	case suggestMsg:
		if msg.seq == r.seq {
			r.setSuggestions(msg.users)
			return unauthorized(msg.err)
		}
	}
	return nil
}

func (r *recipients) key(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "up":
		r.cursor = max(r.cursor-1, 0)
		return nil
	case "down":
		r.cursor = min(r.cursor+1, max(len(r.sugg)-1, 0))
		return nil
	case "enter", ",":
		r.choose()
		return nil
	case "backspace":
		if r.in.Value() == "" && len(r.chips) > 0 {
			r.chips = r.chips[:len(r.chips)-1]
			return nil
		}
	}
	return r.input(msg)
}

// input gives a key or a paste to the text field and starts a search when
// the text changes.
func (r *recipients) input(msg tea.Msg) tea.Cmd {
	before := r.in.Value()
	var cmd tea.Cmd
	r.in, cmd = r.in.Update(msg)
	if r.in.Value() == before {
		return cmd
	}
	return tea.Batch(cmd, r.changed())
}

// changed starts the wait before a user search for the new text.
func (r *recipients) changed() tea.Cmd {
	r.seq++
	r.sugg, r.cursor = nil, 0
	prefix := strings.TrimSpace(r.in.Value())
	if len([]rune(prefix)) < minPrefix {
		return nil
	}
	m := debounceMsg{seq: r.seq, prefix: prefix}
	return tea.Tick(debounce, func(time.Time) tea.Msg { return m })
}

func (r *recipients) search(msg debounceMsg) tea.Cmd {
	if msg.seq != r.seq {
		return nil
	}
	client, limit := r.d.API, maxSuggestions+len(r.chips)
	return func() tea.Msg {
		users, err := client.SearchUsers(bg, msg.prefix, limit)
		return suggestMsg{seq: msg.seq, users: users, err: err}
	}
}

// setSuggestions keeps the users that are not chips and not the viewer.
func (r *recipients) setSuggestions(users []api.UserMatch) {
	r.sugg = r.sugg[:0]
	for _, u := range users {
		if r.d.Me != nil && u.Id == r.d.Me.Id || slices.ContainsFunc(r.chips, func(c api.UserMatch) bool { return c.Id == u.Id }) {
			continue
		}
		if len(r.sugg) < maxSuggestions {
			r.sugg = append(r.sugg, u)
		}
	}
	r.cursor = 0
}

func (r *recipients) choose() {
	if r.cursor >= len(r.sugg) {
		return
	}
	r.chips = append(r.chips, r.sugg[r.cursor])
	r.sugg, r.cursor = nil, 0
	r.seq++
	r.in.SetValue("")
}

// view returns the chips and the input on one line, then one line for
// each suggestion when the field has the focus.
func (r *recipients) view(width int, focused bool) []string {
	s := r.d.Styles
	var b strings.Builder
	for _, c := range r.chips {
		b.WriteString(s.Badge.Render(c.Username+" ×") + " ")
	}
	r.in.SetWidth(max(width-lipgloss.Width(b.String())-1, 8))
	lines := []string{b.String() + r.in.View()}
	if !focused {
		return lines
	}
	for i, u := range r.sugg {
		mark := "  "
		if i == r.cursor {
			mark = s.Key.Render("▶ ")
		}
		lines = append(lines, mark+s.Text.Render(u.Username))
	}
	return lines
}
