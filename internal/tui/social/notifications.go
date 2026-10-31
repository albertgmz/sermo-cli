package social

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/api/gen"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// sentenceIDs maps a notification type to the i18n ID of its sentence.
var sentenceIDs = map[string]string{
	"thread.watched":       "social.notif.reply",
	"node.post":            "social.notif.reply",
	"content.quoted":       "social.notif.quote",
	"content.mentioned":    "social.notif.mention",
	"content.reaction":     "social.notif.reaction",
	"conversation.message": "social.notif.message",
	"conversation.added":   "social.notif.added",
	"profile.post":         "social.notif.profilePost",
	"profile.comment":      "social.notif.profileComment",
	"profile.commented":    "social.notif.profileCommented",
	"member.followed":      "social.notif.follow",
	"node.thread":          "social.notif.newThread",
	"member.thread":        "social.notif.newThread",
}

type notificationsMsg struct {
	page   *api.NotificationList
	more   bool
	err    error
	unread bool
}

type apiDoneMsg struct{ err error }

// notifications is the list of the notifications of the viewer.
type notifications struct {
	d          *ui.Deps
	load       loader
	items      []gen.Notification
	next       *string
	list       ui.List
	unreadOnly bool
	more       bool
	pg         pager
	w, h       int
}

// NewNotifications returns the notification list of the viewer.
func NewNotifications(d *ui.Deps) ui.Screen {
	n := &notifications{d: d, load: newLoader(d)}
	n.list.SetClock(d.Now)
	return n
}

func (n *notifications) Title() string       { return i18n.T("social.notif.title") }
func (n *notifications) CapturesInput() bool { return false }
func (n *notifications) Animating() bool {
	return n.d.Animate() && !n.load.loading && n.list.Animating(n.d.Now())
}

func (n *notifications) Keys() []key.Binding {
	return append(ui.ListKeys(),
		binding("A", "social.key.markAll", "a"),
		binding("U", "social.key.unreadOnly", "u"),
		binding("R", "social.key.retry", "r"),
	)
}

func (n *notifications) Init() tea.Cmd {
	if n.d.Me == nil {
		return login
	}
	client := n.d.API
	viewed := func() tea.Msg {
		if err := client.MarkNotificationsViewed(bg); err != nil {
			return apiDoneMsg{err: err}
		}
		return ui.CountersChangedMsg{}
	}
	return tea.Batch(n.fetch(""), viewed)
}

// fetch loads the page after cursor. An empty cursor loads the first page.
func (n *notifications) fetch(cursor string) tea.Cmd {
	client, unread, limit := n.d.API, n.unreadOnly, n.limit()
	var spin tea.Cmd
	if cursor == "" {
		spin = n.load.begin()
	} else {
		n.more = true
	}
	return tea.Batch(spin, func() tea.Msg {
		page, err := client.Notifications(bg, unread, api.PageOpts{Cursor: cursor, Limit: limit})
		return notificationsMsg{page: page, more: cursor != "", err: err, unread: unread}
	})
}

func (n *notifications) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		n.w, n.h = msg.Width, msg.Height
		n.list.SetHeight(max(n.h-3, 1))
		n.pg.per = bbsRows(n.h, 9)
	case spinner.TickMsg:
		return n, n.load.spin(msg)
	case notificationsMsg:
		return n, n.loaded(msg)
	case apiDoneMsg:
		return n, unauthorized(msg.err)
	case tea.KeyPressMsg:
		return n, n.key(msg)
	case tea.MouseWheelMsg:
		n.list.Update(msg)
		return n, n.loadMore()
	}
	return n, nil
}

func (n *notifications) loaded(msg notificationsMsg) tea.Cmd {
	if msg.unread != n.unreadOnly {
		return nil
	}
	n.more = false
	if msg.err != nil && msg.more {
		return ui.ErrorCmd(msg.err)
	}
	if cmd := n.load.done(msg.err); msg.err != nil {
		return cmd
	}
	if !msg.more {
		n.items, n.pg = nil, pager{per: n.pg.per}
		n.list.Reset()
		n.list.SetHeight(max(n.h-3, 1))
	}
	n.items = append(n.items, msg.page.Items...)
	n.next = msg.page.NextCursor
	n.list.SetLen(len(n.items))
	return nil
}

func (n *notifications) key(msg tea.KeyPressMsg) tea.Cmd {
	if cmd, stop := n.load.blocked(msg, func() tea.Cmd {
		return n.fetch("")
	}); stop {
		return cmd
	}
	if n.d.BBS() {
		n.pg.move(msg.String(), len(n.items))
		return nil
	}
	switch msg.String() {
	case "a":
		return n.markAll()
	case "u":
		n.unreadOnly = !n.unreadOnly
		return n.fetch("")
	}
	if n.list.Update(msg) {
		return n.open(n.list.Cursor())
	}
	return n.loadMore()
}

// loadMore loads the next page when the cursor is on the last row.
func (n *notifications) loadMore() tea.Cmd {
	if n.more || n.next == nil || n.list.Cursor() < n.list.Len()-1 {
		return nil
	}
	return n.fetch(*n.next)
}

func (n *notifications) markAll() tea.Cmd {
	for i := range n.items {
		n.items[i].Read = true
	}
	client := n.d.API
	return func() tea.Msg {
		_, err := client.MarkNotificationsRead(bg)
		return apiDoneMsg{err: err}
	}
}

// open marks the notification i as read and opens its target.
func (n *notifications) open(i int) tea.Cmd {
	item := &n.items[i]
	var mark tea.Cmd
	if !item.Read {
		item.Read = true
		client, id := n.d.API, item.Id
		mark = func() tea.Msg {
			_, err := client.MarkNotificationsRead(bg, id)
			return apiDoneMsg{err: err}
		}
	}
	return tea.Batch(mark, n.target(*item))
}

// target returns the Cmd that opens the content of a notification.
func (n *notifications) target(item gen.Notification) tea.Cmd {
	if msg := intent(item, n.d.Me); msg != nil {
		return func() tea.Msg { return msg }
	}
	return ui.Open(absURL(n.d.Server, item.Url))
}

// intent returns the screen intent for a notification, or nil when the
// client has no screen for it.
func intent(item gen.Notification, me *gen.SelfUser) tea.Msg {
	if msg := contentIntent(item, me); msg != nil {
		return msg
	}
	if item.ThreadId != nil {
		return ui.OpenThreadMsg{ThreadID: *item.ThreadId}
	}
	return nil
}

// contentIntent returns the intent for the content type of a
// notification, or nil.
func contentIntent(item gen.Notification, me *gen.SelfUser) tea.Msg {
	switch item.ContentType {
	case "post":
		if item.ThreadId != nil {
			return ui.OpenThreadMsg{ThreadID: *item.ThreadId, PostID: item.ContentId}
		}
	case "conversation":
		return ui.OpenConversationMsg{ID: item.ContentId}
	case "conversation_message":
		if id, ok := item.Data["conversationId"].(float64); ok {
			return ui.OpenConversationMsg{ID: int(id)}
		}
	case "user":
		return ui.OpenProfileMsg{UserID: item.ContentId}
	case "profile_post":
		if item.Type == "profile.post" && me != nil {
			return ui.OpenProfileMsg{UserID: me.Id}
		}
	}
	return nil
}

// absURL returns path as an absolute URL on the server.
func absURL(server, path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return strings.TrimRight(server, "/") + "/" + strings.TrimLeft(path, "/")
}

// sentence returns the text of a notification.
func sentence(item gen.Notification) string {
	title := ""
	if item.ContentTitle != nil {
		title = *item.ContentTitle
	}
	id, known := sentenceIDs[item.Type]
	if known && len(item.Actors) > 0 {
		return i18n.Plural(id, max(item.ActorCount, 1), map[string]any{"Actors": actors(item), "Title": title})
	}
	if title != "" {
		return i18n.Tf("social.notif.generic", map[string]any{"Title": title})
	}
	return i18n.T("social.notif.genericNoTitle")
}

// actors returns the first actor and the count of the others.
func actors(item gen.Notification) string {
	name := item.Actors[0].Username
	if item.ActorCount <= 1 {
		return name
	}
	return i18n.Tf("social.notif.andMore", map[string]any{"Name": name, "N": item.ActorCount - 1})
}

func (n *notifications) View(width, height int) string {
	if n.d.BBS() {
		return n.bbsView(width, height)
	}
	if v, ok := n.load.view(width, height); ok {
		return v
	}
	s := n.d.Styles
	title := n.Title()
	if n.unreadOnly {
		title += " · " + i18n.T("social.notif.unreadOnly")
	}
	head := ui.Header(s, n.d.Theme, title, width)
	if len(n.items) == 0 {
		return head + "\n" + ui.Empty(s, i18n.T("social.notif.empty"), width, height-1)
	}
	rows := n.list.View(s, n.d.Theme, width, n.d.Phase(), func(i, w int) string {
		item := n.items[i]
		dot := "  "
		if !item.Read {
			dot = s.Accent.Render("●") + " "
		}
		return dot + s.Text.Render(sentence(item)) + "  " + s.Muted.Render(relTime(n.d, item.UpdatedAt))
	})
	foot := ""
	if n.more {
		foot = s.Muted.Render(i18n.T("social.loadingMore"))
	}
	return head + "\n\n" + rows + "\n" + foot
}
