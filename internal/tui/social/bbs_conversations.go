package social

import (
	"context"
	"errors"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/api/gen"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// limit is the number of conversations on one page: the rows that fit in
// the BBS table, else pageSize.
func (c *conversations) limit() int { return rowLimit(c.d, c.pg) }

// Prompt is the label of the BBS prompt.
func (c *conversations) Prompt() string {
	if c.led != nil {
		return i18n.T("bbs.social.editor.prompt")
	}
	return i18n.T("bbs.social.conv.prompt")
}

// Commands are the BBS commands of the conversation list.
func (c *conversations) Commands() []ui.Command {
	if c.led != nil {
		return editorHint()
	}
	pick := func(arg string) tea.Cmd {
		i, ok := c.pg.pick(arg, len(c.items))
		if !ok {
			return noRow()
		}
		return ui.Push(NewConversation(c.d, c.items[i].Id))
	}
	cmds := []ui.Command{
		{Name: "#", Label: i18n.T("bbs.social.conv.open"), Run: pick},
		{Name: "", Run: func(string) tea.Cmd { return pick(strconv.Itoa(c.pg.sel + 1)) }},
		{Name: "C", Label: i18n.T("bbs.social.conv.new"), Run: func(string) tea.Cmd { return c.compose() }},
	}
	cmds = append(cmds, pageCommands(func() tea.Cmd { return c.turn(c.page - 1) }, func() tea.Cmd { return c.turn(c.page + 1) })...)
	return append(cmds, back())
}

func (c *conversations) turn(page int) tea.Cmd {
	if c.load.loading || page < 1 || page > c.pages {
		return nil
	}
	c.pg.sel = 0
	return c.fetch(page)
}

// composeBBS opens the line editor for a new conversation. It asks the
// recipients and the title before the body.
func (c *conversations) composeBBS() tea.Cmd {
	client := c.d.API
	heading := i18n.T("bbs.social.conv.newHeading")
	c.led = newLineEditor(c.d, heading, api.MessageTarget(0), "", func(ctx context.Context, v draft) (tea.Msg, error) {
		ids, err := resolveNames(ctx, client, v.Names)
		if err != nil {
			return nil, err
		}
		out, err := client.CreateConversation(ctx, v.Title, v.Body, ids)
		if err != nil {
			return nil, err
		}
		return conversationCreatedMsg{id: out.Conversation.Id}, nil
	})
	c.led.asks = []string{i18n.T("social.editor.to"), i18n.T("bbs.social.conv.subject")}
	c.led.setSize(c.w, c.h)
	return nil
}

// resolveNames returns the user IDs of the usernames. A name must match a
// username exactly, without case.
func resolveNames(ctx context.Context, client *api.Client, names []string) ([]int, error) {
	ids := make([]int, 0, len(names))
	for _, name := range names {
		users, err := client.SearchUsers(ctx, name, 10)
		if err != nil {
			return nil, err
		}
		id := 0
		for _, u := range users {
			if strings.EqualFold(u.Username, name) {
				id = u.Id
			}
		}
		if id == 0 {
			return nil, errors.New(i18n.Tf("bbs.social.conv.noUser", map[string]any{"Name": name}))
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// whoTalks returns the starter and the count of the other participants,
// for example "Ana +2".
func whoTalks(v gen.ConversationSummary) string {
	if v.ParticipantCount <= 1 {
		return v.Starter.Username
	}
	return v.Starter.Username + " +" + strconv.Itoa(v.ParticipantCount-1)
}

func (c *conversations) bbsView(width, height int) string {
	if v, ok := c.load.bbsView(width, height); ok {
		return v
	}
	s := c.d.Styles
	t := ui.NewTable(width,
		ui.Column{Title: i18n.T("bbs.help.number"), Width: 2, Align: ui.AlignRight},
		ui.Column{Title: "(*)", Width: 3},
		ui.Column{Title: i18n.T("bbs.social.conv.subjectCol")},
		ui.Column{Title: i18n.T("bbs.social.conv.who"), Width: 13},
		ui.Column{Title: i18n.T("bbs.social.conv.last"), Width: 18},
	)
	page := i18n.Tf("bbs.social.page", map[string]any{"Page": c.page, "Pages": c.pages})
	t.Full(apart(s.Subtitle.Render(i18n.T("bbs.social.conv.heading")), s.Text.Render(page), width-4)).Header()
	rows := min(len(c.items), bbsRows(height, 8))
	for i := range rows {
		v := c.items[i]
		mark := "( )"
		if v.IsUnread {
			mark = s.Accent.Render("(*)")
		}
		last := bbsAgo(c.d, v.LastMessage.SentAt) + " " + api.AuthorName(v.LastMessage.User)
		t.Row(ui.Num(i+1, rows), mark, v.Title, whoTalks(v), last)
		if i == c.pg.sel {
			t.Select()
		}
	}
	if len(c.items) == 0 {
		t.Full(i18n.T("social.conv.empty"))
	}
	t.Full(i18n.T("bbs.social.notif.legend"))
	return t.Render(s)
}
