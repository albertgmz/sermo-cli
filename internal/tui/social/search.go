package social

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/api/gen"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

type searchMsg struct {
	page int
	// limit is the page size of the request.
	limit   int
	data    *api.Page[gen.SearchResult]
	users   []api.UserMatch
	members bool
	err     error
}

// search finds posts by words, or members by the start of the username.
// It searches only on Enter because the server limits the search rate.
// The query field gets the focus with / or Enter, so the global hotkeys
// work when the screen opens.
type search struct {
	d          *ui.Deps
	in         textinput.Model
	typing     bool
	titlesOnly bool
	members    bool
	load       loader
	searched   bool
	results    []gen.SearchResult
	users      []api.UserMatch
	page       int
	pages      int
	// shown is the page size of the shown results, or 0.
	shown int
	list  ui.List
	pg    pager
	w, h  int
}

// NewSearch returns the search screen.
func NewSearch(d *ui.Deps) ui.Screen {
	s := &search{d: d, in: newInput(d), load: newLoader(d), page: 1, pages: 1}
	s.load.loading = false
	s.list.SetClock(d.Now)
	s.in.Placeholder = i18n.T("social.search.placeholder")
	return s
}

func (s *search) Title() string {
	if s.members {
		return i18n.T("social.search.membersTitle")
	}
	return i18n.T("social.search.title")
}

func (s *search) CapturesInput() bool { return s.typing }
func (s *search) Init() tea.Cmd       { return nil }
func (s *search) Animating() bool {
	return s.d.Animate() && !s.typing && len(s.items()) > 0 && s.list.Animating(s.d.Now())
}

func (s *search) Keys() []key.Binding {
	return append(ui.ListKeys(),
		binding("Ctrl+T", "social.key.titlesOnly", "ctrl+t"),
		binding("Ctrl+U", "social.key.memberSearch", "ctrl+u"),
		binding("/", "social.key.editQuery", "/", "i"),
		binding("h l", "social.key.pages", "h", "l"),
	)
}

func (s *search) items() []string {
	out := make([]string, 0, len(s.results)+len(s.users))
	for _, r := range s.results {
		out = append(out, r.Thread.Title)
	}
	for _, u := range s.users {
		out = append(out, u.Username)
	}
	return out
}

func (s *search) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.w, s.h = msg.Width, msg.Height
		s.in.SetWidth(max(s.w-14, 8))
		s.pg.per = bbsRows(s.h, searchFrame)
		return s, s.refit()
	case spinner.TickMsg:
		return s, s.load.spin(msg)
	case searchMsg:
		return s, s.loaded(msg)
	case tea.KeyPressMsg:
		return s, s.key(msg)
	case tea.PasteMsg:
		if s.typing {
			s.in, _ = s.in.Update(msg)
		}
	case tea.MouseWheelMsg:
		s.list.Update(msg)
	}
	return s, nil
}

func (s *search) key(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+t":
		s.titlesOnly = !s.titlesOnly
		return nil
	case "ctrl+u":
		s.members = !s.members
		s.edit()
		return nil
	}
	if s.d.BBS() {
		s.pg.move(msg.String(), len(s.items()))
		return nil
	}
	if s.typing {
		return s.typeKey(msg)
	}
	return s.resultKey(msg)
}

func (s *search) typeKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		return s.run(1)
	case "esc":
		s.typing = false
		s.in.Blur()
		if s.searched {
			return nil
		}
		return ui.Pop()
	}
	var cmd tea.Cmd
	s.in, cmd = s.in.Update(msg)
	return cmd
}

func (s *search) resultKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "/", "i":
		s.edit()
		return nil
	case "h", "left":
		return s.turn(s.page - 1)
	case "l", "right":
		return s.turn(s.page + 1)
	}
	if s.load.err != nil && key.Matches(msg, retryKey) {
		return s.run(s.page)
	}
	if s.list.Update(msg) {
		return s.open(s.list.Cursor())
	}
	if msg.String() == "enter" {
		s.edit()
	}
	return nil
}

// edit gives the focus to the query field.
func (s *search) edit() {
	s.typing = true
	_ = s.in.Focus()
}

func (s *search) turn(page int) tea.Cmd {
	if s.members || page < 1 || page > s.pages || s.load.loading {
		return nil
	}
	return s.run(page)
}

// run sends the search for the page.
func (s *search) run(page int) tea.Cmd {
	q := strings.TrimSpace(s.in.Value())
	if q == "" || s.load.loading {
		return nil
	}
	s.typing, s.searched = false, true
	s.in.Blur()
	client, members, limit := s.d.API, s.members, s.limit()
	opts := api.SearchOpts{Query: q, TitlesOnly: s.titlesOnly, PageOpts: api.PageOpts{Page: page, Limit: limit}}
	return tea.Batch(s.load.begin(), func() tea.Msg {
		if members {
			users, err := client.SearchUsers(bg, q, limit)
			return searchMsg{users: users, members: true, err: err}
		}
		data, err := client.Search(bg, opts)
		return searchMsg{page: page, limit: limit, data: data, err: err}
	})
}

func (s *search) loaded(msg searchMsg) tea.Cmd {
	if cmd := s.load.done(msg.err); msg.err != nil {
		return cmd
	}
	s.results, s.users = nil, nil
	if msg.members {
		s.users = msg.users
	} else {
		s.results, s.page, s.shown = msg.data.Items, msg.page, msg.limit
		s.pages = pageCountOf(msg.data.Total, msg.page, msg.limit)
	}
	s.pg.sel = 0
	s.list.Reset()
	s.list.SetLen(len(s.items()))
	return s.refit()
}

func (s *search) open(i int) tea.Cmd {
	var msg tea.Msg
	if s.users != nil {
		msg = ui.OpenProfileMsg{UserID: s.users[i].Id}
	} else {
		r := s.results[i]
		msg = ui.OpenThreadMsg{ThreadID: r.Thread.Id, PostID: r.Post.Id}
	}
	return func() tea.Msg { return msg }
}

func (s *search) View(width, height int) string {
	if s.d.BBS() {
		return s.bbsView(width, height)
	}
	st := s.d.Styles
	label := st.Muted.Render(i18n.T("social.search.label") + " ")
	if s.typing {
		label = st.Key.Render(i18n.T("social.search.label") + " ")
	}
	top := []string{
		ui.Header(st, s.d.Theme, s.Title(), width),
		label + s.in.View(),
		st.Muted.Render(ansiTrim(s.options(), width)),
		"",
	}
	return strings.Join(top, "\n") + "\n" + s.body(width, max(height-len(top), 1))
}

func (s *search) options() string {
	box := "[ ]"
	if s.titlesOnly {
		box = "[x]"
	}
	return i18n.Tf("social.search.options", map[string]any{"Box": box})
}

func (s *search) body(width, height int) string {
	if v, ok := s.load.view(width, height); ok {
		return v
	}
	st := s.d.Styles
	if !s.searched {
		return ui.Empty(st, i18n.T("social.search.hint"), width, height)
	}
	if len(s.items()) == 0 {
		return ui.Empty(st, i18n.T("social.search.none"), width, height)
	}
	if s.users != nil {
		s.list.SetHeight(height)
		return s.list.View(st, s.d.Theme, width, s.d.Phase(), func(i, _ int) string {
			return userName(s.d, s.users[i].Username, nil)
		})
	}
	return s.resultsView(width, height)
}

// resultsView shows one row for each result and the snippet of the
// selected result under the list.
func (s *search) resultsView(width, height int) string {
	st := s.d.Styles
	s.list.SetHeight(max(height-6, 1))
	rows := s.list.View(st, s.d.Theme, width, s.d.Phase(), func(i, _ int) string {
		r := s.results[i]
		return st.Subtitle.Render(r.Thread.Title) + st.Muted.Render("  · "+r.Thread.Node.Title+" · "+
			api.AuthorName(r.Post.Author)+" · "+relTime(s.d, r.Post.CreatedAt))
	})
	snippet := plainText(s.results[s.list.Cursor()].Post.Excerpt)
	wrapped := firstLines(strings.Split(lipgloss.Wrap(snippet, ui.ReadingWidth(width), " "), "\n"), 3)
	page := i18n.Tf("social.page", map[string]any{"Page": s.page, "Pages": s.pages})
	return rows + "\n\n" + st.Text.Render(strings.Join(wrapped, "\n")) + "\n\n" + st.Muted.Render(page)
}
