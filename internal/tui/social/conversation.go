package social

import (
	"context"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/api/gen"
	"github.com/albertgmz/sermo-cli/internal/bbcode"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/render"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

type messagesMsg struct {
	conv *gen.Conversation
	page int
	data *api.Page[gen.Message]
	jump int // the message ID to show first, or 0
	err  error
}

type quoteMsg struct {
	quote *gen.MessageQuote
	err   error
}

type replySentMsg struct{ id int }

// conversation reads one conversation, one page at a time, oldest
// message first.
type conversation struct {
	d      *ui.Deps
	id     int
	load   loader
	conv   *gen.Conversation
	msgs   []gen.Message
	page   int
	pages  int
	total  int
	vp     viewport.Model
	texts  []string
	links  [][]render.Link
	starts []int
	cur    int
	width  int // the width of texts
	read   bool
	// pendingJump is the message to show after the next load.
	pendingJump int
	ed          *editor
	led         *lineEditor
	picker      linkPicker
	// off is the first shown line of the BBS view.
	off      int
	bbsLines []string
	// jumpAt is one more than the index of the message to show after the
	// next load, or 0.
	jumpAt int
	w, h   int
}

// NewConversation returns the conversation id.
func NewConversation(d *ui.Deps, id int) ui.Screen {
	return &conversation{d: d, id: id, load: newLoader(d), page: 1, pages: 1, vp: viewport.New()}
}

func (c *conversation) Title() string {
	if c.conv != nil {
		return c.conv.Title
	}
	return i18n.T("social.conv.one")
}

func (c *conversation) CapturesInput() bool { return c.ed != nil || c.led != nil }

func (c *conversation) Keys() []key.Binding {
	if c.ed != nil {
		return editorKeys()
	}
	return []key.Binding{
		binding("↑↓ j k", "social.key.scroll", "up", "down", "j", "k"),
		binding("g G", "core.key.ends", "g", "G"),
		binding("[ ]", "social.key.messages", "[", "]"),
		binding("h l", "social.key.pages", "h", "l"),
		binding("R", "social.key.reply", "r"),
		binding("C", "social.key.quote", "c"),
		binding("O", "social.key.links", "o"),
		binding("A", "social.key.author", "a"),
	}
}

func (c *conversation) Init() tea.Cmd {
	if c.d.Me == nil {
		return login
	}
	return c.fetch(0, true)
}

// fetch loads the page. With locate true it loads the page of the first
// unread message instead. jump is the message to show first.
func (c *conversation) fetch(page int, locate bool) tea.Cmd {
	client, id, conv := c.d.API, c.id, c.conv
	return tea.Batch(c.load.begin(), func() tea.Msg {
		out := messagesMsg{conv: conv, page: page}
		out.err = fetchMessages(client, id, locate, &out)
		return out
	})
}

func fetchMessages(client *api.Client, id int, locate bool, out *messagesMsg) error {
	var err error
	if out.conv == nil {
		if out.conv, err = client.Conversation(bg, id); err != nil {
			return err
		}
	}
	if locate {
		loc, err := client.LocateUnreadMessage(bg, id, pageSize)
		if err != nil {
			return err
		}
		out.page, out.jump = loc.Page, loc.MessageId
	}
	out.data, err = client.Messages(bg, id, false, api.PageOpts{Page: out.page, Limit: pageSize})
	return err
}

func (c *conversation) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	if cmd, ok := c.routeEditors(msg); ok {
		return c, cmd
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.resize(msg.Width, msg.Height)
	case spinner.TickMsg:
		return c, c.load.spin(msg)
	case messagesMsg:
		return c, c.loaded(msg)
	case quoteMsg:
		return c, c.quoted(msg)
	case replySentMsg:
		return c, c.replied(msg.id)
	case apiDoneMsg:
		return c, unauthorized(msg.err)
	case tea.KeyPressMsg:
		return c, c.key(msg)
	case tea.MouseWheelMsg:
		c.vp, _ = c.vp.Update(msg)
		c.follow()
	}
	return c, nil
}

// replied loads the last page and shows the new message id.
func (c *conversation) replied(id int) tea.Cmd {
	c.ed, c.led = nil, nil
	c.total++
	return tea.Batch(ui.Toast(i18n.T("social.sent"), ui.ToastSuccess), c.fetchAt(pageCount(&c.total, 1), id))
}

// fetchAt loads page and shows the message jump first.
func (c *conversation) fetchAt(page, jump int) tea.Cmd {
	cmd := c.fetch(page, false)
	c.pendingJump = jump
	return cmd
}

func (c *conversation) resize(w, h int) {
	c.w, c.h = w, h
	c.vp.SetWidth(ui.ReadingWidth(w))
	c.vp.SetHeight(max(h-4, 1))
	if c.ed != nil {
		c.ed.setSize(w, h)
	}
	if c.led != nil {
		c.led.setSize(w, h)
	}
	if c.msgs != nil && c.width != c.textWidth() {
		c.build()
	}
}

func (c *conversation) loaded(msg messagesMsg) tea.Cmd {
	if cmd := c.load.done(msg.err); msg.err != nil {
		return cmd
	}
	c.conv, c.page, c.msgs = msg.conv, msg.page, msg.data.Items
	if msg.data.Total != nil {
		c.total = *msg.data.Total
	}
	c.pages = pageCount(msg.data.Total, msg.page)
	c.build()
	jump := msg.jump
	if jump == 0 {
		jump, c.pendingJump = c.pendingJump, 0
	}
	c.show(c.indexOf(jump))
	if c.jumpAt > 0 {
		c.show(c.jumpAt - 1)
		c.jumpAt = 0
	}
	warm := c.warmNextPage()
	if c.read {
		return warm
	}
	c.read = true
	client, id := c.d.API, c.id
	return tea.Batch(warm, func() tea.Msg { return apiDoneMsg{err: client.MarkConversationRead(bg, id)} })
}

// warmNextPage warms the page after the shown page when the setting allows a prefetch.
// The Cmd only adds a job to the prefetch queue.
func (c *conversation) warmNextPage() tea.Cmd {
	if !c.d.Config.LightPrefetch() || c.page >= c.pages {
		return nil
	}
	client, id, p := c.d.API, c.id, c.page+1
	return func() tea.Msg {
		client.WarmMessages(id, false, api.PageOpts{Page: p, Limit: pageSize})
		return nil
	}
}

func (c *conversation) indexOf(messageID int) int {
	for i, m := range c.msgs {
		if m.Id == messageID {
			return i
		}
	}
	return 0
}

// build renders the messages at the reading width.
func (c *conversation) build() {
	if c.d.BBS() {
		c.buildBBS()
		return
	}
	c.width = ui.ReadingWidth(c.w)
	c.texts, c.links, c.starts = nil, nil, nil
	var lines []string
	s := c.d.Styles
	for _, m := range c.msgs {
		c.starts = append(c.starts, len(lines))
		name := api.AuthorName(m.Author)
		lines = append(lines, userName(c.d, name, nil)+s.Muted.Render("  "+relTime(c.d, m.CreatedAt)))
		text, links, err := c.d.Render.Post(m.BodyHtml, attachments(m.Attachments), c.width, c.d.Glamour)
		if err != nil {
			text = s.Error.Render(err.Error())
		}
		c.links = append(c.links, links)
		lines = append(lines, strings.Split(strings.TrimRight(text, "\n"), "\n")...)
		lines = append(lines, "")
	}
	c.vp.SetContent(strings.Join(lines, "\n"))
}

// attachment is the type of one attachment of a message, a profile post
// or a comment in package gen.
type attachment = struct {
	ByteSize     int     `json:"byteSize"`
	ContentType  string  `json:"contentType"`
	FileId       int     `json:"fileId"`
	Filename     *string `json:"filename"`
	Height       *int    `json:"height"`
	ThumbnailUrl *string `json:"thumbnailUrl"`
	Url          string  `json:"url"`
	Width        *int    `json:"width"`
	Widths       []struct {
		Url   string `json:"url"`
		Width int    `json:"width"`
	} `json:"widths"`
}

// attachments converts the attachments for the renderer.
func attachments(in []attachment) []render.Attachment {
	out := make([]render.Attachment, 0, len(in))
	for _, a := range in {
		name := ""
		if a.Filename != nil {
			name = *a.Filename
		}
		out = append(out, render.Attachment{ID: a.FileId, URL: a.Url, Filename: name, ContentType: a.ContentType})
	}
	return out
}

// show selects the message i and scrolls to it.
func (c *conversation) show(i int) {
	if len(c.starts) == 0 {
		return
	}
	c.cur = max(0, min(i, len(c.starts)-1))
	c.vp.SetYOffset(c.starts[c.cur])
	c.off = c.starts[c.cur]
}

// follow selects the last message that starts above the top of the view.
func (c *conversation) follow() {
	c.cur = 0
	for i, at := range c.starts {
		if at <= c.vp.YOffset() {
			c.cur = i
		}
	}
}

func (c *conversation) key(msg tea.KeyPressMsg) tea.Cmd {
	if c.picker.open {
		return c.picker.update(msg)
	}
	if cmd, stop := c.load.blocked(msg, func() tea.Cmd {
		return c.fetch(c.page, c.conv == nil)
	}); stop {
		return cmd
	}
	if c.d.BBS() {
		c.bbsScroll(msg.String())
		return nil
	}
	if cmd, ok := c.action(msg.String()); ok {
		return cmd
	}
	c.scroll(msg)
	return nil
}

// action runs the key k of a reader action.
func (c *conversation) action(k string) (tea.Cmd, bool) {
	switch k {
	case "r":
		return c.reply(""), true
	case "c":
		return c.quote(), true
	case "o":
		return c.picker.start(c.links[c.cur]), true
	case "a":
		return c.author(), true
	case "h", "left":
		return c.turn(c.page - 1), true
	case "l", "right":
		return c.turn(c.page + 1), true
	}
	return nil, false
}

func (c *conversation) scroll(msg tea.KeyPressMsg) {
	switch msg.String() {
	case "g", "home":
		c.show(0)
		c.vp.GotoTop()
	case "G", "end":
		c.vp.GotoBottom()
		c.follow()
	case "[":
		c.show(c.cur - 1)
	case "]":
		c.show(c.cur + 1)
	default:
		c.vp, _ = c.vp.Update(msg)
		c.follow()
	}
}

func (c *conversation) turn(page int) tea.Cmd {
	if page < 1 || page > c.pages {
		return nil
	}
	return c.fetch(page, false)
}

func (c *conversation) author() tea.Cmd {
	id := api.AuthorID(c.msgs[c.cur].Author)
	if id == 0 {
		return nil
	}
	return func() tea.Msg { return ui.OpenProfileMsg{UserID: id} }
}

func (c *conversation) quote() tea.Cmd { return c.quoteAt(c.cur) }

// quoteAt gets the quote of the message i of the page.
func (c *conversation) quoteAt(i int) tea.Cmd {
	client, id := c.d.API, c.msgs[i].Id
	return func() tea.Msg {
		q, err := client.QuoteMessage(bg, id)
		return quoteMsg{quote: q, err: err}
	}
}

func (c *conversation) quoted(msg quoteMsg) tea.Cmd {
	if msg.err != nil {
		return ui.ErrorCmd(msg.err)
	}
	q := msg.quote
	if c.led != nil {
		return c.led.addText(bbcode.QuoteMessage(api.AuthorName(q.Author), q.MessageId, api.AuthorID(q.Author), q.BodySource))
	}
	return c.reply(bbcode.QuoteMessage(api.AuthorName(q.Author), q.MessageId, api.AuthorID(q.Author), q.BodySource) + "\n")
}

func (c *conversation) reply(text string) tea.Cmd {
	if c.d.BBS() {
		return c.replyBBS(text)
	}
	client, id := c.d.API, c.id
	heading := i18n.Tf("social.editor.reply", map[string]any{"Title": c.Title()})
	c.ed = newEditor(c.d, heading, api.MessageTarget(id), text, func(ctx context.Context, v draft) (tea.Msg, error) {
		m, err := client.ReplyConversation(ctx, id, v.Body)
		if err != nil {
			return nil, err
		}
		return replySentMsg{id: m.Id}, nil
	})
	c.ed.setSize(c.w, c.h)
	return nil
}

func (c *conversation) View(width, height int) string {
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
	who := s.Muted.Render(ansiTrim(participants(c.conv), width))
	if c.picker.open {
		return head + "\n" + who + "\n\n" + c.picker.view(c.d, width)
	}
	return head + "\n" + who + "\n" + c.vp.View() + "\n" + s.Muted.Render(ansiTrim(c.footer(), width))
}

func (c *conversation) footer() string {
	name := ""
	if len(c.msgs) > 0 {
		name = api.AuthorName(c.msgs[c.cur].Author)
	}
	return i18n.Tf("social.conv.footer", map[string]any{
		"Page": c.page, "Pages": c.pages, "N": c.cur + 1, "Count": len(c.msgs), "Name": name,
	})
}

// participants returns the usernames of the conversation.
func participants(conv *gen.Conversation) string {
	names := make([]string, 0, len(conv.Participants))
	for _, p := range conv.Participants {
		names = append(names, p.User.Username)
	}
	return strings.Join(names, ", ")
}
