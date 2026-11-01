package social

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

func (t *postsTab) limit() int     { return rowLimit(t.d, t.pg) }
func (t *bookmarksTab) limit() int { return rowLimit(t.d, t.pg) }
func (t *watchedTab) limit() int   { return rowLimit(t.d, t.pg) }

// editing reports whether an editor is open.
func (t *postsTab) editing() bool { return t.ed != nil || t.led != nil }

// enterCommand opens the selected row of p on a bare Enter.
func enterCommand(p *pager, n int, open func(i int) tea.Cmd) ui.Command {
	return ui.Command{Name: "", Run: func(string) tea.Cmd {
		lo, hi := p.window(n)
		if lo+p.sel >= hi {
			return nil
		}
		return open(lo + p.sel)
	}}
}

func (t *postsTab) bbsCommands() []ui.Command {
	if t.open >= 0 {
		return []ui.Command{
			{Name: "C", Label: i18n.T("bbs.social.profile.comment"), Run: func(string) tea.Cmd { return t.commentOn(t.open) }},
			{Name: "O", Label: i18n.T("bbs.social.conv.link"), Run: t.openLink},
			{Name: "V", Label: i18n.T("bbs.global.back"), Run: func(string) tea.Cmd { return t.toggle(t.open) }},
		}
	}
	n := len(t.items)
	cmds := []ui.Command{
		numberCommand("bbs.social.notif.open", &t.pg, n, t.toggle),
		enterCommand(&t.pg, n, t.toggle),
		{Name: "W", Label: i18n.T("bbs.social.profile.write"), Run: func(string) tea.Cmd { return t.write() }},
		{Name: "C", Label: i18n.T("bbs.social.profile.commentOn"), Run: t.commentNumber},
	}
	return append(cmds, pageCommands(func() tea.Cmd { t.pg.prev(); return nil }, t.nextWindow)...)
}

// commentNumber comments the post with the row number arg, or the
// selected post without arg.
func (t *postsTab) commentNumber(arg string) tea.Cmd {
	if t.d.Me == nil {
		return loginNeeded()
	}
	if arg == "" {
		arg = strconv.Itoa(t.pg.sel + 1)
	}
	i, ok := t.pg.pick(arg, len(t.items))
	if !ok {
		return noRow()
	}
	return t.commentOn(i)
}

func (t *postsTab) openLink(arg string) tea.Cmd {
	n, err := strconv.Atoi(arg)
	for _, l := range t.links {
		if err == nil && l.N == n {
			return ui.Open(l.URL)
		}
	}
	return ui.Toast(i18n.T("bbs.social.conv.noLink"), ui.ToastError)
}

func (t *postsTab) nextWindow() tea.Cmd {
	if t.load.loading || t.more {
		return nil
	}
	if t.pg.next(len(t.items), t.next != nil) {
		return t.fetch(*t.next)
	}
	return nil
}

func (t *postsTab) bbsKey(k string) {
	if t.open < 0 {
		t.pg.move(k, len(t.items))
	}
}

// renderBBS makes the lines of the open post and its comments.
func (t *postsTab) renderBBS() {
	s, post := t.d.Styles, t.items[t.open]
	w := t.w
	text, links, err := t.d.Render.PostBBS(post.BodyHtml, attachments(post.Attachments), max(w-6, 10))
	if err != nil {
		text = err.Error()
	}
	t.links = links
	lines := []string{boxRow(s, w, apart(api.AuthorName(post.Author), bbsDate(t.d, post.CreatedAt), w-4)), ui.Rule(s, w, '-')}
	for _, l := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		lines = append(lines, boxRow(s, w, " "+l))
	}
	if post.CommentCount > 0 && t.comments == nil {
		lines = append(lines, ui.Rule(s, w, '-'), boxRow(s, w, i18n.T("bbs.social.loading")))
	}
	for _, c := range t.comments {
		lines = append(lines, ui.Rule(s, w, '-'),
			boxRow(s, w, s.Accent.Render(api.AuthorName(c.Author))+"  "+s.Muted.Render(bbsAgo(t.d, c.CreatedAt))))
		for _, l := range strings.Split(lipgloss.Wrap(plainText(c.BodyHtml), max(w-8, 10), " "), "\n") {
			lines = append(lines, boxRow(s, w, "   "+l))
		}
	}
	t.bbsDetail = append(lines, ui.Rule(s, w, '='))
}

func (t *postsTab) bbsView(width, height int) string {
	if v, ok := t.load.bbsView(width, height); ok {
		return v
	}
	s := t.d.Styles
	if t.open >= 0 {
		head := []string{ui.Rule(s, width, '-'), boxRow(s, width, s.Subtitle.Render(i18n.T("bbs.social.profile.post")))}
		return t.d.MarkLinks(t.zid+"link.", pad(append(head, t.bbsDetail...), height), len(t.links))
	}
	t.pg.clamp(len(t.items))
	tb := ui.NewTable(width,
		ui.Column{Title: i18n.T("bbs.help.number"), Width: 2, Align: ui.AlignRight},
		ui.Column{Title: i18n.T("bbs.social.profile.author"), Width: 14},
		ui.Column{Title: i18n.T("bbs.social.profile.message")},
		ui.Column{Title: i18n.T("bbs.social.notif.when"), Width: 9},
	)
	tb.Full(s.Subtitle.Render(i18n.T("bbs.social.profile.wall"))).Header()
	lo, hi := t.pg.window(len(t.items))
	for i := lo; i < hi; i++ {
		p := t.items[i]
		tb.Row(ui.Num(i-lo+1, hi-lo), api.AuthorName(p.Author), plainText(p.BodyHtml), bbsAgo(t.d, p.CreatedAt)).
			Zone(t.d, rowZone(t.zid+"row", i-lo))
		if i-lo == t.pg.sel {
			tb.Select()
		}
	}
	switch {
	case t.more:
		tb.Full(i18n.T("bbs.social.loading"))
	case len(t.items) == 0:
		tb.Full(i18n.T("social.profile.noPosts"))
	}
	return tb.Render(s)
}

func (t *bookmarksTab) bbsCommands() []ui.Command {
	n := len(t.items)
	open := func(i int) tea.Cmd { return openBookmark(t.items[i]) }
	turn := func(page int) tea.Cmd {
		if t.load.loading || page < 1 || page > t.pages {
			return nil
		}
		return t.fetch(page)
	}
	cmds := []ui.Command{numberCommand("bbs.social.notif.open", &t.pg, n, open), enterCommand(&t.pg, n, open)}
	return append(cmds, pageCommands(func() tea.Cmd { return turn(t.page - 1) }, func() tea.Cmd { return turn(t.page + 1) })...)
}

func (t *bookmarksTab) bbsKey(k string) { t.pg.move(k, len(t.items)) }

func (t *bookmarksTab) bbsView(width, height int) string {
	if v, ok := t.load.bbsView(width, height); ok {
		return v
	}
	s := t.d.Styles
	t.pg.clamp(len(t.items))
	tb := ui.NewTable(width,
		ui.Column{Title: i18n.T("bbs.help.number"), Width: 2, Align: ui.AlignRight},
		ui.Column{Title: i18n.T("bbs.social.profile.title")},
		ui.Column{Title: i18n.T("bbs.social.profile.author"), Width: 14},
		ui.Column{Title: i18n.T("bbs.social.search.date"), Width: 9},
	)
	page := i18n.Tf("bbs.social.page", map[string]any{"Page": t.page, "Pages": t.pages})
	tb.Full(apart(s.Subtitle.Render(i18n.T("bbs.social.profile.bookmarksHeading")), t.d.Mark(t.zid+"page", s.Text.Render(page)), width-4)).Header()
	lo, hi := t.pg.window(len(t.items))
	for i := lo; i < hi; i++ {
		b := t.items[i]
		tb.Row(ui.Num(i-lo+1, hi-lo), b.Title, api.AuthorName(b.Author), bbsAgo(t.d, b.CreatedAt)).
			Zone(t.d, rowZone(t.zid+"row", i-lo))
		if i-lo == t.pg.sel {
			tb.Select()
		}
	}
	if len(t.items) == 0 {
		tb.Full(i18n.T("social.profile.noBookmarks"))
	}
	return tb.Render(s)
}

func (t *watchedTab) bbsCommands() []ui.Command {
	n := len(t.items)
	open := func(i int) tea.Cmd {
		id := t.items[i].ThreadId
		return func() tea.Msg { return ui.OpenThreadMsg{ThreadID: id} }
	}
	unwatch := func(i int) tea.Cmd {
		client, id := t.d.API, t.items[i].ThreadId
		return func() tea.Msg { return unwatchedMsg{threadID: id, err: client.UnwatchThread(bg, id)} }
	}
	cmds := []ui.Command{numberCommand("bbs.social.notif.open", &t.pg, n, open), enterCommand(&t.pg, n, open)}
	x := numberCommand("bbs.social.profile.unwatch", &t.pg, n, unwatch)
	x.Name = "X"
	cmds = append(cmds, x)
	return append(cmds, pageCommands(func() tea.Cmd { t.pg.prev(); return nil }, t.nextWindow)...)
}

func (t *watchedTab) nextWindow() tea.Cmd {
	if t.load.loading || t.more {
		return nil
	}
	if t.pg.next(len(t.items), t.next != nil) {
		return t.fetch(*t.next)
	}
	return nil
}

func (t *watchedTab) bbsKey(k string) { t.pg.move(k, len(t.items)) }

func (t *watchedTab) bbsView(width, height int) string {
	if v, ok := t.load.bbsView(width, height); ok {
		return v
	}
	s := t.d.Styles
	t.pg.clamp(len(t.items))
	tb := ui.NewTable(width,
		ui.Column{Title: i18n.T("bbs.help.number"), Width: 2, Align: ui.AlignRight},
		ui.Column{Title: i18n.T("bbs.social.search.thread")},
		ui.Column{Title: i18n.T("bbs.social.conv.last"), Width: 9},
	)
	tb.Full(s.Subtitle.Render(i18n.T("bbs.social.profile.watchedHeading"))).Header()
	lo, hi := t.pg.window(len(t.items))
	for i := lo; i < hi; i++ {
		w := t.items[i]
		tb.Row(ui.Num(i-lo+1, hi-lo), w.Title, bbsAgo(t.d, w.LastPostAt)).Zone(t.d, rowZone(t.zid+"row", i-lo))
		if i-lo == t.pg.sel {
			tb.Select()
		}
	}
	switch {
	case t.more:
		tb.Full(i18n.T("bbs.social.loading"))
	case len(t.items) == 0:
		tb.Full(i18n.T("social.profile.noWatched"))
	}
	return tb.Render(s)
}
