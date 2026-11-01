package social

import (
	"context"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/api/gen"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

type conversationsMsg struct {
	page int
	data *api.Page[gen.ConversationSummary]
	err  error
}

type conversationCreatedMsg struct{ id int }

// conversations is the list of the conversations of the viewer, one page
// at a time.
type conversations struct {
	d     *ui.Deps
	load  loader
	items []gen.ConversationSummary
	page  int
	pages int
	list  ui.List
	ed    *editor
	led   *lineEditor
	pg    pager
	w, h  int
	zid   string // the prefix of the zone IDs
}

// NewConversations returns the conversation list of the viewer.
func NewConversations(d *ui.Deps) ui.Screen {
	c := &conversations{d: d, load: newLoader(d), page: 1, pages: 1, zid: ui.ZonePrefix()}
	c.list.SetClock(d.Now)
	c.list.SetZones(d, c.zid+"row")
	return c
}

func (c *conversations) Title() string       { return i18n.T("social.conv.title") }
func (c *conversations) CapturesInput() bool { return c.ed != nil || c.led != nil }
func (c *conversations) Animating() bool {
	return c.d.Animate() && !c.load.loading && c.ed == nil && c.list.Animating(c.d.Now())
}

func (c *conversations) Keys() []key.Binding {
	if c.ed != nil {
		return editorKeys()
	}
	return append(ui.ListKeys(),
		binding("h l", "social.key.pages", "h", "l"),
		binding("C", "social.key.newConversation", "c"),
		binding("R", "social.key.retry", "r"),
	)
}

func (c *conversations) Init() tea.Cmd {
	if c.d.Me == nil {
		return login
	}
	return c.fetch(c.page)
}

func (c *conversations) fetch(page int) tea.Cmd {
	client, limit := c.d.API, c.limit()
	return tea.Batch(c.load.begin(), func() tea.Msg {
		data, err := client.Conversations(bg, api.PageOpts{Page: page, Limit: limit})
		return conversationsMsg{page: page, data: data, err: err}
	})
}

func (c *conversations) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	if cmd, ok := routeEditor(&c.ed, msg); ok {
		return c, cmd
	}
	if cmd, ok := routeLineEditor(&c.led, msg); ok {
		return c, cmd
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.resize(msg.Width, msg.Height)
	case spinner.TickMsg:
		return c, c.load.spin(msg)
	case conversationsMsg:
		return c, c.loaded(msg)
	case conversationCreatedMsg:
		c.ed, c.led = nil, nil
		open := func() tea.Msg { return ui.OpenConversationMsg{ID: msg.id} }
		return c, tea.Batch(c.fetch(1), open)
	case tea.KeyPressMsg:
		return c, c.key(msg)
	case tea.MouseWheelMsg:
		c.list.Update(msg)
	case tea.MouseReleaseMsg:
		return c, c.click(msg)
	}
	return c, nil
}

// click turns the page for a click on the page counter. A click on a row
// selects it, or opens it when it is the selected row.
func (c *conversations) click(msg tea.MouseReleaseMsg) tea.Cmd {
	if c.load.loading || c.load.err != nil {
		return nil
	}
	if page, ok := pageTurn(c.d, c.zid+"page", msg, c.page); ok {
		return c.turn(page)
	}
	if c.d.BBS() {
		if i, ok := c.pg.click(c.d, c.zid+"row", msg, len(c.items)); ok {
			return ui.Push(NewConversation(c.d, c.items[i].Id))
		}
		return nil
	}
	if c.list.Update(msg) {
		return ui.Push(NewConversation(c.d, c.items[c.list.Cursor()].Id))
	}
	return nil
}

func (c *conversations) resize(w, h int) {
	c.w, c.h = w, h
	c.list.SetHeight(max(c.h-4, 1))
	if c.ed != nil {
		c.ed.setSize(c.w, c.h)
	}
	if c.led != nil {
		c.led.setSize(w, h)
	}
	if c.pg.per == 0 {
		c.pg.per = bbsRows(h, 8)
	}
}

func (c *conversations) loaded(msg conversationsMsg) tea.Cmd {
	if cmd := c.load.done(msg.err); msg.err != nil {
		return cmd
	}
	c.page, c.items = msg.page, msg.data.Items
	c.pages = pageCountOf(msg.data.Total, msg.page, c.limit())
	c.pg.clamp(len(c.items))
	c.list.Reset()
	c.list.SetHeight(max(c.h-4, 1))
	c.list.SetLen(len(c.items))
	return nil
}

// pageCount returns the number of pages for total items, or page when
// the total is not known.
func pageCount(total *int, page int) int { return pageCountOf(total, page, pageSize) }

// pageCountOf is pageCount for pages of per items.
func pageCountOf(total *int, page, per int) int {
	if total == nil {
		return page
	}
	return max((*total+per-1)/per, 1)
}

func (c *conversations) key(msg tea.KeyPressMsg) tea.Cmd {
	if cmd, stop := c.load.blocked(msg, func() tea.Cmd { return c.fetch(c.page) }); stop {
		return cmd
	}
	if c.d.BBS() {
		c.pg.move(msg.String(), len(c.items))
		return nil
	}
	switch msg.String() {
	case "c":
		return c.compose()
	case "h", "left":
		if c.page > 1 {
			return c.fetch(c.page - 1)
		}
		return nil
	case "l", "right":
		if c.page < c.pages {
			return c.fetch(c.page + 1)
		}
		return nil
	}
	if c.list.Update(msg) {
		return ui.Push(NewConversation(c.d, c.items[c.list.Cursor()].Id))
	}
	return nil
}

func (c *conversations) compose() tea.Cmd {
	if c.d.BBS() {
		return c.composeBBS()
	}
	client := c.d.API
	c.ed = newConversationEditor(c.d, func(ctx context.Context, v draft) (tea.Msg, error) {
		out, err := client.CreateConversation(ctx, v.Title, v.Body, v.To)
		if err != nil {
			return nil, err
		}
		return conversationCreatedMsg{id: out.Conversation.Id}, nil
	})
	c.ed.setSize(c.w, c.h)
	return nil
}

func (c *conversations) View(width, height int) string {
	if c.ed != nil {
		return c.ed.view()
	}
	if c.led != nil {
		return c.led.view()
	}
	if c.d.BBS() {
		return c.bbsView(width, height)
	}
	if v, ok := c.load.view(width, height); ok {
		return v
	}
	s := c.d.Styles
	head := ui.Header(s, c.d.Theme, c.Title(), width)
	if len(c.items) == 0 {
		return head + "\n" + ui.Empty(s, i18n.T("social.conv.empty"), width, height-1)
	}
	rows := c.list.View(s, c.d.Theme, width, c.d.Phase(), func(i, _ int) string { return c.row(c.items[i]) })
	foot := c.d.Mark(c.zid+"page", s.Muted.Render(i18n.Tf("social.page", map[string]any{"Page": c.page, "Pages": c.pages})))
	return head + "\n\n" + rows + "\n\n" + foot
}

func (c *conversations) row(v gen.ConversationSummary) string {
	s := c.d.Styles
	mark := "  "
	title := s.Text.Render(v.Title)
	if v.IsUnread {
		mark = s.Accent.Render("●") + " "
		title = s.Subtitle.Render(v.Title)
	}
	who := i18n.Plural("social.conv.participants", v.ParticipantCount, map[string]any{"Starter": v.Starter.Username})
	last := i18n.Tf("social.conv.last", map[string]any{
		"Name": api.AuthorName(v.LastMessage.User), "When": relTime(c.d, v.LastMessage.SentAt),
	})
	return mark + title + s.Muted.Render("  ·  "+who+"  ·  "+last)
}

// editorKeys are the keys of an open editor for the help screen.
func editorKeys() []key.Binding {
	return []key.Binding{
		binding("Ctrl+S", "social.key.send", "ctrl+s"),
		binding("Ctrl+P", "social.key.preview", "ctrl+p"),
		binding("Ctrl+B I U K", "social.key.format", "ctrl+b", "ctrl+i", "ctrl+u", "ctrl+k"),
		binding("Tab", "social.key.nextField", "tab"),
		binding("Esc", "social.key.cancel", "esc"),
	}
}
