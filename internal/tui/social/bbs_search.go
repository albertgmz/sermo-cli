package social

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// searchFrame is the number of lines of the BBS search around the rows of
// the table: the frame and two lines of the snippet.
const searchFrame = 11

func (s *search) limit() int { return rowLimit(s.d, s.pg) }

// refit searches again for the page that shows the first shown result
// when the page size changed, for example after a resize.
func (s *search) refit() tea.Cmd {
	limit := s.limit()
	if !s.d.BBS() || s.members || s.load.loading || s.shown == 0 || s.shown == limit {
		return nil
	}
	return s.run(refitPage(s.page, s.shown, limit))
}

// Prompt is the label of the BBS prompt.
func (s *search) Prompt() string { return i18n.T("social.search.title") }

// Commands are the BBS commands of the search. B and U take the text to
// search after the command letter.
func (s *search) Commands() []ui.Command {
	n := len(s.items())
	cmds := []ui.Command{
		{Name: "B", Label: i18n.T("bbs.social.search.query"), Run: func(arg string) tea.Cmd { return s.query(arg, false) }},
		numberCommand("bbs.social.notif.open", &s.pg, n, s.open),
		enterCommand(&s.pg, n, s.open),
		{Name: "T", Label: i18n.T("bbs.social.search.titles"), Run: func(string) tea.Cmd { return s.toggleTitles() }},
		{Name: "U", Label: i18n.T("bbs.social.search.members"), Run: func(arg string) tea.Cmd { return s.query(arg, true) }},
	}
	cmds = append(cmds, back())
	return append(cmds, pageCommands(func() tea.Cmd { return s.turn(s.page - 1) }, func() tea.Cmd { return s.turn(s.page + 1) })...)
}

// query searches the text: members by the start of the username, else
// posts.
func (s *search) query(text string, members bool) tea.Cmd {
	if strings.TrimSpace(text) == "" {
		return ui.Toast(i18n.T("bbs.social.search.noText"), ui.ToastInfo)
	}
	s.in.SetValue(text)
	s.members = members
	return s.run(1)
}

// toggleTitles changes the titles only option. A post search shows its
// results again with the new option.
func (s *search) toggleTitles() tea.Cmd {
	s.titlesOnly = !s.titlesOnly
	if s.searched && !s.members {
		return s.run(1)
	}
	return nil
}

func (s *search) bbsView(width, height int) string {
	st := s.d.Styles
	box := "[ ]"
	if s.titlesOnly {
		box = "[x]"
	}
	head := i18n.T("bbs.social.search.heading")
	if s.searched {
		head += "  «" + strings.TrimSpace(s.in.Value()) + "»"
	}
	right := box + " " + i18n.T("social.key.titlesOnly")
	if !s.members && s.searched {
		page := i18n.Tf("bbs.social.page", map[string]any{"Page": s.page, "Pages": s.pages})
		right = s.d.Mark(s.zid+"page", page) + "   " + right
	}
	title := apart(st.Subtitle.Render(head), st.Text.Render(right), width-4)
	if v, ok := s.load.bbsView(width, height-3); ok {
		return ui.Box(st, width, []string{title}) + "\n" + v
	}
	if !s.searched {
		t := ui.NewTable(width, ui.Column{})
		return t.Full(title).Rule().Wrap(i18n.T("bbs.social.search.hint")).Render(st)
	}
	if s.members {
		return s.membersView(width, title)
	}
	return s.resultsBBS(width, height, title)
}

func (s *search) membersView(width int, title string) string {
	st := s.d.Styles
	t := ui.NewTable(width,
		ui.Column{Title: i18n.T("bbs.help.number"), Width: 2, Align: ui.AlignRight},
		ui.Column{Title: i18n.T("bbs.social.online.user")},
	)
	t.Full(title).Header()
	for i, u := range s.users {
		t.Row(ui.Num(i+1, len(s.users)), u.Username).Zone(s.d, rowZone(s.zid+"row", i))
		if i == s.pg.sel {
			t.Select()
		}
	}
	if len(s.users) == 0 {
		t.Full(i18n.T("social.search.none"))
	}
	return t.Render(st)
}

func (s *search) resultsBBS(width, height int, title string) string {
	st := s.d.Styles
	t := ui.NewTable(width,
		ui.Column{Title: i18n.T("bbs.help.number"), Width: 2, Align: ui.AlignRight},
		ui.Column{Title: i18n.T("bbs.social.search.thread")},
		ui.Column{Title: i18n.T("bbs.social.search.forum"), Width: 16},
		ui.Column{Title: i18n.T("bbs.social.profile.author"), Width: 12},
		ui.Column{Title: i18n.T("bbs.social.search.date"), Width: 9},
	)
	t.Full(title).Header()
	rows := min(len(s.results), bbsRows(height, searchFrame))
	for i := range rows {
		r := s.results[i]
		t.Row(ui.Num(i+1, rows), r.Thread.Title, r.Thread.Node.Title, api.AuthorName(r.Post.Author), bbsAgo(s.d, r.Post.CreatedAt)).
			Zone(s.d, rowZone(s.zid+"row", i))
		if i == s.pg.sel {
			t.Select()
		}
	}
	if rows == 0 {
		t.Full(i18n.T("social.search.none"))
		return t.Render(st)
	}
	snippet := plainText(s.results[min(s.pg.sel, rows-1)].Post.Excerpt)
	for _, l := range firstLines(strings.Split(lipgloss.Wrap(snippet, max(width-4, 1), " "), "\n"), 2) {
		t.Full(st.Muted.Render(l))
	}
	return t.Render(st)
}
