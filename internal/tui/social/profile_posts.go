package social

import (
	"context"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/api/gen"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/render"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

type postsMsg struct {
	page *api.CursorPage[gen.ProfilePost]
	more bool
	err  error
}

type commentsMsg struct {
	postID int
	page   *api.CursorPage[gen.ProfileComment]
	err    error
}

type profilePostSentMsg struct{}

type commentSentMsg struct{ postID int }

// postsTab lists the posts on a profile wall. Enter opens a post with its
// comments.
type postsTab struct {
	d        *ui.Deps
	userID   int
	owner    string
	canPost  bool
	load     loader
	items    []gen.ProfilePost
	next     *string
	more     bool
	list     ui.List
	open     int // the index of the open post, or -1
	comments []gen.ProfileComment
	detail   string
	links    []render.Link
	picker   linkPicker
	ed       *editor
	led      *lineEditor
	pg       pager
	// bbsDetail are the lines of the open post in the BBS view.
	bbsDetail []string
	w, h      int
}

func newPostsTab(d *ui.Deps, userID int) *postsTab {
	return &postsTab{d: d, userID: userID, load: newLoader(d), open: -1}
}

func (t *postsTab) name() string   { return i18n.T("social.profile.posts") }
func (t *postsTab) start() tea.Cmd { return t.fetch("") }

func (t *postsTab) resize(w, h int) {
	t.w, t.h = w, h
	t.pg.per = bbsRows(h, profileFrame)
	if t.ed != nil {
		t.ed.setSize(w, h)
	}
	if t.led != nil {
		t.led.setSize(w, h)
	}
	if t.open >= 0 {
		t.render()
	}
}

func (t *postsTab) fetch(cursor string) tea.Cmd {
	client, id, limit := t.d.API, t.userID, t.limit()
	var spin tea.Cmd
	if cursor == "" {
		spin = t.load.begin()
	} else {
		t.more = true
	}
	return tea.Batch(spin, func() tea.Msg {
		page, err := client.ProfilePosts(bg, id, cursor, limit)
		return postsMsg{page: page, more: cursor != "", err: err}
	})
}

func (t *postsTab) update(msg tea.Msg) tea.Cmd {
	if cmd, ok := routeEditor(&t.ed, msg); ok {
		return cmd
	}
	if cmd, ok := routeLineEditor(&t.led, msg); ok {
		return cmd
	}
	switch msg := msg.(type) {
	case spinner.TickMsg:
		return t.load.spin(msg)
	case postsMsg:
		return t.loaded(msg)
	case commentsMsg:
		return t.commentsLoaded(msg)
	case profilePostSentMsg:
		t.ed, t.led = nil, nil
		return tea.Batch(ui.Toast(i18n.T("social.sent"), ui.ToastSuccess), t.fetch(""))
	case commentSentMsg:
		t.ed, t.led = nil, nil
		return tea.Batch(ui.Toast(i18n.T("social.sent"), ui.ToastSuccess), t.commentAdded(msg.postID))
	case tea.MouseWheelMsg:
		t.list.Update(msg)
	}
	return nil
}

func (t *postsTab) loaded(msg postsMsg) tea.Cmd {
	t.more = false
	if msg.err != nil && msg.more {
		return ui.ErrorCmd(msg.err)
	}
	if cmd := t.load.done(msg.err); msg.err != nil {
		return cmd
	}
	if !msg.more {
		t.items, t.open = nil, -1
		t.list, t.pg = ui.List{}, pager{per: t.pg.per}
	}
	t.items = append(t.items, msg.page.Items...)
	t.next = msg.page.NextCursor
	t.list.SetLen(len(t.items))
	return nil
}

func (t *postsTab) commentsLoaded(msg commentsMsg) tea.Cmd {
	if t.open < 0 || t.items[t.open].Id != msg.postID {
		return nil
	}
	if msg.err != nil {
		return ui.ErrorCmd(msg.err)
	}
	t.comments = msg.page.Items
	t.render()
	return nil
}

// commentAdded counts the new comment and shows the post with its
// comments.
func (t *postsTab) commentAdded(postID int) tea.Cmd {
	for i := range t.items {
		if t.items[i].Id == postID {
			t.items[i].CommentCount++
			t.open = -1
			return t.toggle(i)
		}
	}
	return nil
}

func (t *postsTab) key(msg tea.KeyPressMsg) tea.Cmd {
	if t.picker.open {
		return t.picker.update(msg)
	}
	if cmd, stop := t.load.blocked(msg, func() tea.Cmd {
		return t.fetch("")
	}); stop {
		return cmd
	}
	switch msg.String() {
	case "w":
		return t.write()
	case "c":
		return t.comment()
	case "o":
		return t.picker.start(t.links)
	}
	if t.list.Update(msg) {
		return t.toggle(t.list.Cursor())
	}
	if t.next != nil && !t.more && t.list.Cursor() == t.list.Len()-1 {
		return t.fetch(*t.next)
	}
	return nil
}

// toggle opens the post i, or closes it when it is open. The comments
// load only when the post has some.
func (t *postsTab) toggle(i int) tea.Cmd {
	if t.open == i {
		t.open, t.links = -1, nil
		return nil
	}
	t.open, t.comments = i, nil
	t.render()
	post := t.items[i]
	if post.CommentCount == 0 {
		return nil
	}
	client := t.d.API
	return func() tea.Msg {
		page, err := client.ProfileComments(bg, post.Id, "", pageSize)
		return commentsMsg{postID: post.Id, page: page, err: err}
	}
}

// render makes the text of the open post and its comments.
func (t *postsTab) render() {
	if t.d.BBS() {
		t.renderBBS()
		return
	}
	s, post := t.d.Styles, t.items[t.open]
	width := ui.ReadingWidth(t.w)
	text, links, err := t.d.Render.Post(post.BodyHtml, attachments(post.Attachments), width, t.d.Glamour)
	if err != nil {
		text = s.Error.Render(err.Error())
	}
	t.links = links
	lines := []string{strings.TrimRight(text, "\n")}
	for _, c := range t.comments {
		lines = append(lines, "  "+userName(t.d, api.AuthorName(c.Author), nil)+s.Muted.Render("  "+relTime(t.d, c.CreatedAt)),
			"  "+s.Text.Render(plainText(c.BodyHtml)))
	}
	t.detail = strings.Join(lines, "\n")
}

func (t *postsTab) write() tea.Cmd {
	if t.d.Me == nil {
		return loginNeeded()
	}
	if !t.canPost {
		return ui.Toast(i18n.T("social.profile.cannotPost"), ui.ToastError)
	}
	client, id := t.d.API, t.userID
	heading := i18n.Tf("social.editor.profilePost", map[string]any{"Name": t.owner})
	t.openEditor(heading, func(ctx context.Context, v draft) (tea.Msg, error) {
		_, err := client.CreateProfilePost(ctx, id, v.Body)
		return profilePostSentMsg{}, err
	})
	return nil
}

// openEditor opens the editor of the interface: the line editor in the
// BBS interface, else the BB code editor.
func (t *postsTab) openEditor(heading string, send sendFunc) {
	if t.d.BBS() {
		t.led = newLineEditor(t.d, heading, api.MessageTarget(0), "", send)
		t.led.setSize(t.w, t.h)
		return
	}
	t.ed = newEditor(t.d, heading, api.MessageTarget(0), "", send)
	t.ed.setSize(t.w, t.h)
}

func (t *postsTab) comment() tea.Cmd {
	if t.d.Me == nil {
		return loginNeeded()
	}
	if len(t.items) == 0 {
		return nil
	}
	return t.commentOn(t.list.Cursor())
}

// commentOn opens the editor for a comment on the post i.
func (t *postsTab) commentOn(i int) tea.Cmd {
	post := t.items[i]
	if !post.CanComment {
		return ui.Toast(i18n.T("social.profile.cannotComment"), ui.ToastError)
	}
	client := t.d.API
	heading := i18n.Tf("social.editor.comment", map[string]any{"Name": api.AuthorName(post.Author)})
	t.openEditor(heading, func(ctx context.Context, v draft) (tea.Msg, error) {
		_, err := client.CreateProfileComment(ctx, post.Id, v.Body)
		return commentSentMsg{postID: post.Id}, err
	})
	return nil
}

func (t *postsTab) view(width, height int) string {
	if v, ok := t.load.view(width, height); ok {
		return v
	}
	if t.picker.open {
		return t.picker.view(t.d, width)
	}
	if len(t.items) == 0 {
		return ui.Empty(t.d.Styles, i18n.T("social.profile.noPosts"), width, height)
	}
	listH := height
	if t.open >= 0 {
		listH = min(len(t.items), max(height/3, 3))
	}
	t.list.SetHeight(listH)
	rows := t.list.View(t.d.Styles, t.d.Theme, width, t.d.Phase(), func(i, _ int) string { return t.row(t.items[i]) })
	if t.open < 0 {
		return rows
	}
	detail := firstLines(strings.Split(t.detail, "\n"), height-listH-1)
	return rows + "\n\n" + strings.Join(detail, "\n")
}

func (t *postsTab) row(p gen.ProfilePost) string {
	s := t.d.Styles
	meta := relTime(t.d, p.CreatedAt)
	if p.CommentCount > 0 {
		meta += " · " + i18n.Plural("social.profile.comments", p.CommentCount, nil)
	}
	return userName(t.d, api.AuthorName(p.Author), nil) + "  " + s.Text.Render(plainText(p.BodyHtml)) + "  " + s.Muted.Render(meta)
}
