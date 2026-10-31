package social

import (
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// limit is the number of notifications in one request: the rows that fit
// in the BBS table, else pageSize.
func (n *notifications) limit() int { return rowLimit(n.d, n.pg) }

// Prompt is the label of the BBS prompt.
func (n *notifications) Prompt() string { return i18n.T("bbs.social.notif.prompt") }

// Commands are the BBS commands of the notification list.
func (n *notifications) Commands() []ui.Command {
	pick := func(arg string) tea.Cmd {
		i, ok := n.pg.pick(arg, len(n.items))
		if !ok {
			return noRow()
		}
		return n.open(i)
	}
	cmds := []ui.Command{
		{Name: "#", Label: i18n.T("bbs.social.notif.open"), Run: pick},
		{Name: "", Run: func(string) tea.Cmd { return pick(strconv.Itoa(n.pg.sel + 1)) }},
		{Name: "L", Label: i18n.T("bbs.social.notif.readAll"), Run: func(string) tea.Cmd { return n.markAll() }},
		{Name: "U", Label: i18n.T("bbs.social.notif.unreadOnly"), Run: func(string) tea.Cmd {
			n.unreadOnly = !n.unreadOnly
			return n.fetch("")
		}},
	}
	cmds = append(cmds, pageCommands(
		func() tea.Cmd { n.pg.prev(); return nil },
		n.nextWindow,
	)...)
	return append(cmds, back())
}

// nextWindow shows the next rows. It loads them when they are not there.
func (n *notifications) nextWindow() tea.Cmd {
	if n.load.loading || n.more {
		return nil
	}
	if n.pg.next(len(n.items), n.next != nil) {
		return n.fetch(*n.next)
	}
	return nil
}

func (n *notifications) bbsView(width, height int) string {
	if v, ok := n.load.bbsView(width, height); ok {
		return v
	}
	s := n.d.Styles
	n.pg.clamp(len(n.items))
	t := ui.NewTable(width,
		ui.Column{Title: i18n.T("bbs.help.number"), Width: 2, Align: ui.AlignRight},
		ui.Column{Title: "(*)", Width: 3},
		ui.Column{Title: i18n.T("bbs.social.notif.item")},
		ui.Column{Title: i18n.T("bbs.social.notif.when"), Width: 9},
	)
	title := i18n.T("bbs.social.notif.heading")
	if n.unreadOnly {
		title += " (" + i18n.T("social.notif.unreadOnly") + ")"
	}
	t.Full(s.Subtitle.Render(title)).Header()
	lo, hi := n.pg.window(len(n.items))
	for i := lo; i < hi; i++ {
		item := n.items[i]
		mark := "( )"
		if !item.Read {
			mark = s.Accent.Render("(*)")
		}
		t.Row(ui.Num(i-lo+1, hi-lo), mark, sentence(item), bbsAgo(n.d, item.UpdatedAt))
		if i-lo == n.pg.sel {
			t.Select()
		}
	}
	switch {
	case n.more:
		t.Full(i18n.T("bbs.social.loading"))
	case len(n.items) == 0:
		t.Full(i18n.T("social.notif.empty"))
	}
	t.Full(i18n.T("bbs.social.notif.legend"))
	return t.Render(s)
}
