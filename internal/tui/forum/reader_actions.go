package forum

import (
	"html"
	"regexp"
	"slices"
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/api/gen"
	"github.com/albertgmz/sermo-cli/internal/bbcode"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

type reactedMsg struct {
	postID  int
	summary *gen.ReactionSummary
	err     error
}

type bookmarkedMsg struct {
	postID int
	on     bool
	err    error
}

// sourceMsg has the BB code of a post for a quote or an edit.
type sourceMsg struct {
	post  *gen.PostDetail
	quote bool
	err   error
}

type spoilersMsg struct {
	postID int
	html   string
	err    error
}

// readerKeys maps the action keys of the reader to their methods.
var readerKeys = map[string]func(*reader) tea.Cmd{
	"]":     func(r *reader) tea.Cmd { r.jumpTo(r.focus + 1); return nil },
	"[":     func(r *reader) tea.Cmd { r.jumpTo(r.focus - 1); return nil },
	"h":     func(r *reader) tea.Cmd { return r.turn(-1) },
	"left":  func(r *reader) tea.Cmd { return r.turn(-1) },
	"l":     func(r *reader) tea.Cmd { return r.turn(1) },
	"right": func(r *reader) tea.Cmd { return r.turn(1) },
	"g":     func(r *reader) tea.Cmd { r.jumpTo(0); return nil },
	"G":     func(r *reader) tea.Cmd { r.jumpTo(len(r.posts) - 1); r.vp.GotoBottom(); return nil },
	"u":     (*reader).firstUnread,
	"r":     (*reader).reply,
	"c":     (*reader).quote,
	"e":     (*reader).edit,
	"+":     (*reader).react,
	"*":     (*reader).bookmark,
	"w":     (*reader).watch,
	"o":     (*reader).links,
	"a":     (*reader).profile,
	"v":     (*reader).poll,
	"s":     (*reader).showSpoilers,
}

func (r *reader) key(k tea.KeyPressMsg) tea.Cmd {
	if r.picker != nil {
		return r.pick(k)
	}
	if r.err != nil {
		if k.String() == "r" {
			return r.open()
		}
		return nil
	}
	if r.view == nil || r.loading {
		return nil
	}
	if fn, ok := readerKeys[k.String()]; ok {
		return fn(r)
	}
	r.scroll(k)
	return nil
}

func (r *reader) updateAction(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case reactedMsg:
		return r.reacted(msg)
	case bookmarkedMsg:
		return r.bookmarkDone(msg)
	case watchedMsg:
		if msg.err == nil {
			r.watching = msg.on
		}
		return msg.cmd()
	case sourceMsg:
		return r.compose(msg)
	case cardsMsg:
		r.gotCards(msg)
	case spoilersMsg:
		if msg.err != nil {
			return ui.ErrorCmd(msg.err)
		}
		r.spoilers[msg.postID] = msg.html
		r.rerender(msg.postID)
	}
	return nil
}

func (r *reader) turn(dir int) tea.Cmd {
	p := r.page + dir
	if p < 1 || p > r.pages {
		return nil
	}
	return r.loadPage(p, 0)
}

func (r *reader) firstUnread() tea.Cmd {
	if !r.member() {
		return loginToast()
	}
	d, id := r.d, r.threadID
	r.loading = true
	return func() tea.Msg {
		u, err := d.API.LocateUnread(bg(), id)
		if err != nil {
			return readerMsg{err: err}
		}
		data, err := fetchPage(d, id, pageOf(u.Position), u.PostId)
		return readerMsg{page: data, err: err}
	}
}

// canWrite checks the guest state and a permission. It returns a toast
// Cmd when the action cannot run.
func (r *reader) canWrite(allowed bool, deniedID string) (tea.Cmd, bool) {
	switch {
	case !r.member():
		return loginToast(), false
	case !allowed:
		return toast(deniedID), false
	case r.focusedPost() == nil:
		return nil, false
	}
	return nil, true
}

func (r *reader) reply() tea.Cmd {
	if cmd, ok := r.canWrite(r.view.Permissions.CanReply, "forum.toast.cannotReply"); !ok {
		return cmd
	}
	return ui.Push(newReplyComposer(r.d, r.threadID, "", r))
}

func (r *reader) quote() tea.Cmd {
	if cmd, ok := r.canWrite(r.view.Permissions.CanReply, "forum.toast.cannotReply"); !ok {
		return cmd
	}
	return r.fetchSource(true)
}

func (r *reader) edit() tea.Cmd {
	p := r.focusedPost()
	if cmd, ok := r.canWrite(p != nil && p.CanEdit, "forum.toast.cannotEdit"); !ok {
		return cmd
	}
	return r.fetchSource(false)
}

func (r *reader) fetchSource(quote bool) tea.Cmd {
	c, id := r.d.API, r.focusedPost().Id
	return func() tea.Msg {
		p, err := c.Post(bg(), id)
		return sourceMsg{post: p, quote: quote, err: err}
	}
}

func (r *reader) compose(msg sourceMsg) tea.Cmd {
	if msg.err != nil {
		return ui.ErrorCmd(msg.err)
	}
	p := msg.post
	if !msg.quote {
		return ui.Push(newEditComposer(r.d, r.threadID, p.Id, p.BodySource, r))
	}
	body := bbcode.Quote(api.AuthorName(p.Author), p.Id, api.AuthorID(p.Author), p.BodySource) + "\n"
	return ui.Push(newReplyComposer(r.d, r.threadID, body, r))
}

// react opens the picker with the reaction types that the viewer can give
// in this forum.
func (r *reader) react() tea.Cmd {
	p := r.focusedPost()
	if cmd, ok := r.canWrite(p != nil && p.CanReact, "forum.toast.cannotReact"); !ok {
		return cmd
	}
	var items []string
	r.pickIDs = nil
	cursor := 0
	for _, t := range r.rtypes {
		if !r.reactionAllowed(t) {
			continue
		}
		if p.Reactions.Mine != nil && *p.Reactions.Mine == t.Id {
			cursor = len(items)
		}
		label := t.Emoji + " " + t.Title
		if t.Access == gen.Locked {
			label = r.d.Styles.Muted.Render(label)
		}
		items = append(items, label)
		r.pickIDs = append(r.pickIDs, t.Id)
	}
	if len(items) == 0 {
		return toast("forum.toast.cannotReact")
	}
	r.picker, r.pickKind = newPicker(r.d, i18n.T("forum.pick.reaction"), items, cursor), pickReaction
	return nil
}

func (r *reader) reactionAllowed(t gen.ReactionType) bool {
	if !t.IsActive || t.Access == gen.Hidden {
		return false
	}
	return len(t.ForumIds) == 0 || slices.Contains(t.ForumIds, r.view.Thread.NodeId)
}

func (r *reader) pick(msg tea.Msg) tea.Cmd {
	i, done := r.picker.update(msg)
	if !done {
		return nil
	}
	r.picker = nil
	if i < 0 {
		return nil
	}
	return r.chosen(i)
}

// chosen runs the action of the picker for the item i.
func (r *reader) chosen(i int) tea.Cmd {
	if r.pickKind == pickLink {
		return ui.Open(r.bodies[r.focus].links[i].URL)
	}
	return r.sendReaction(r.pickIDs[i])
}

func (r *reader) sendReaction(typeID int) tea.Cmd {
	for _, t := range r.rtypes {
		if t.Id == typeID && t.Access == gen.Locked && t.LockHint != nil {
			return ui.Toast(*t.LockHint, ui.ToastInfo)
		}
	}
	p := r.focusedPost()
	c, id, remove := r.d.API, p.Id, p.Reactions.Mine != nil && *p.Reactions.Mine == typeID
	return func() tea.Msg {
		var s *gen.ReactionSummary
		var err error
		if remove {
			s, err = c.RemoveReaction(bg(), api.ContentPost, id)
		} else {
			s, err = c.SetReaction(bg(), api.ContentPost, id, typeID)
		}
		return reactedMsg{postID: id, summary: s, err: err}
	}
}

func (r *reader) reacted(msg reactedMsg) tea.Cmd {
	if msg.err != nil {
		return ui.ErrorCmd(msg.err)
	}
	for i := range r.posts {
		if r.posts[i].Id == msg.postID {
			r.posts[i].Reactions = *msg.summary
		}
	}
	r.keepOffset(r.assemble)
	return nil
}

func (r *reader) bookmark() tea.Cmd {
	if cmd, ok := r.canWrite(true, ""); !ok {
		return cmd
	}
	c, id := r.d.API, r.focusedPost().Id
	on := !r.bookmarked[id]
	return func() tea.Msg {
		err := c.SetBookmark(bg(), gen.BookmarksSetJSONBodyContentTypePost, id, on)
		return bookmarkedMsg{postID: id, on: on, err: err}
	}
}

func (r *reader) bookmarkDone(msg bookmarkedMsg) tea.Cmd {
	if msg.err != nil {
		return ui.ErrorCmd(msg.err)
	}
	r.bookmarked[msg.postID] = msg.on
	r.keepOffset(r.assemble)
	if msg.on {
		return ui.Toast(i18n.T("forum.toast.bookmarked"), ui.ToastSuccess)
	}
	return ui.Toast(i18n.T("forum.toast.unbookmarked"), ui.ToastSuccess)
}

func (r *reader) watch() tea.Cmd {
	if !r.member() {
		return loginToast()
	}
	c, id, on := r.d.API, r.threadID, !r.watching
	return func() tea.Msg {
		var err error
		if on {
			err = c.WatchThread(bg(), id, false)
		} else {
			err = c.UnwatchThread(bg(), id)
		}
		return watchedMsg{on: on, err: err}
	}
}

func (r *reader) links() tea.Cmd {
	if r.focusedPost() == nil || len(r.bodies[r.focus].links) == 0 {
		return toast("forum.toast.noLinks")
	}
	links := r.bodies[r.focus].links
	items := make([]string, len(links))
	for i, l := range links {
		items[i] = l.Label + "  " + r.d.Styles.Muted.Render(l.URL)
	}
	r.picker, r.pickKind = newPicker(r.d, i18n.T("forum.pick.link"), items, 0), pickLink
	return nil
}

func (r *reader) profile() tea.Cmd {
	p := r.focusedPost()
	if p == nil || !api.IsMember(p.Author) {
		return nil
	}
	id := api.AuthorID(p.Author)
	return func() tea.Msg { return ui.OpenProfileMsg{UserID: id} }
}

func (r *reader) poll() tea.Cmd {
	if !r.view.Poll.Exists {
		return toast("forum.toast.noPoll")
	}
	return ui.Push(newPoll(r.d, r.threadID))
}

// spoilerTag matches a spoiler placeholder of a body.
var (
	spoilerTag   = regexp.MustCompile(`<(div|span)\s[^>]*class="bb-spoiler"[^>]*>\s*</(?:div|span)>`)
	spoilerPos   = regexp.MustCompile(`data-spoiler="(\d+)"`)
	spoilerTitle = regexp.MustCompile(`data-spoiler-title="([^"]*)"`)
)

// showSpoilers gets the content of all spoiler placeholders of the focused
// post and puts it in the body.
func (r *reader) showSpoilers() tea.Cmd {
	p := r.focusedPost()
	if p == nil || !spoilerTag.MatchString(p.BodyHtml) {
		return toast("forum.toast.noSpoilers")
	}
	c, id, body := r.d.API, p.Id, p.BodyHtml
	return func() tea.Msg {
		out, err := expandSpoilers(body, func(pos int) (string, error) {
			return c.Spoiler(bg(), api.ContentPost, id, pos)
		})
		return spoilersMsg{postID: id, html: out, err: err}
	}
}

// expandSpoilers changes each placeholder into a spoiler with its content.
// The renderer shows such a spoiler with its title and its content.
func expandSpoilers(body string, get func(pos int) (string, error)) (string, error) {
	var firstErr error
	out := spoilerTag.ReplaceAllStringFunc(body, func(tag string) string {
		m := spoilerPos.FindStringSubmatch(tag)
		if m == nil || firstErr != nil {
			return tag
		}
		pos, _ := strconv.Atoi(m[1])
		content, err := get(pos)
		if err != nil {
			firstErr = err
			return tag
		}
		title := ""
		if t := spoilerTitle.FindStringSubmatch(tag); t != nil {
			title = t[1]
		}
		if spoilerTag.FindStringSubmatch(tag)[1] == "span" {
			return `<span class="spoiler">` + content + `</span>`
		}
		return `<details class="spoiler"><summary>` + html.EscapeString(html.UnescapeString(title)) + `</summary>` + content + `</details>`
	})
	return out, firstErr
}

func (r *reader) rerender(postID int) {
	for i, p := range r.posts {
		if p.Id == postID {
			r.bodies[i] = r.renderBody(p)
		}
	}
	r.keepOffset(r.assemble)
}

// keepOffset runs fn and keeps the scroll offset of the viewport.
func (r *reader) keepOffset(fn func()) {
	y := r.vp.YOffset()
	fn()
	r.vp.SetYOffset(y)
}
