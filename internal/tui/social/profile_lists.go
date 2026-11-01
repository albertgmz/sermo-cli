package social

import (
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/api/gen"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

type bookmarksMsg struct {
	page int
	// limit is the page size of the request.
	limit int
	data  *api.Page[gen.Bookmark]
	err   error
}

// bookmarksTab lists the bookmarks of the viewer, one page at a time.
type bookmarksTab struct {
	d     *ui.Deps
	load  loader
	items []gen.Bookmark
	page  int
	pages int
	// shown is the page size of the shown page, or 0.
	shown int
	list  ui.List
	pg    pager
	zid   string // the prefix of the zone IDs
}

func newBookmarksTab(d *ui.Deps) *bookmarksTab {
	t := &bookmarksTab{d: d, load: newLoader(d), page: 1, pages: 1, zid: ui.ZonePrefix()}
	t.list.SetZones(d, t.zid+"row")
	return t
}

func (t *bookmarksTab) name() string   { return i18n.T("social.profile.bookmarks") }
func (t *bookmarksTab) start() tea.Cmd { return t.fetch(1) }

func (t *bookmarksTab) fetch(page int) tea.Cmd {
	client, limit := t.d.API, t.limit()
	return tea.Batch(t.load.begin(), func() tea.Msg {
		data, err := client.Bookmarks(bg, api.PageOpts{Page: page, Limit: limit})
		return bookmarksMsg{page: page, limit: limit, data: data, err: err}
	})
}

func (t *bookmarksTab) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		return t.load.spin(msg)
	case bookmarksMsg:
		if cmd := t.load.done(msg.err); msg.err != nil {
			return cmd
		}
		t.page, t.items, t.shown = msg.page, msg.data.Items, msg.limit
		t.pages = pageCountOf(msg.data.Total, msg.page, msg.limit)
		t.list.Reset()
		t.pg.sel = 0
		t.list.SetLen(len(t.items))
		return t.refit()
	case tea.MouseWheelMsg:
		t.list.Update(msg)
	}
	return nil
}

func (t *bookmarksTab) key(msg tea.KeyPressMsg) tea.Cmd {
	if cmd, stop := t.load.blocked(msg, func() tea.Cmd {
		return t.fetch(t.page)
	}); stop {
		return cmd
	}
	switch msg.String() {
	case "h", "left":
		if t.page > 1 {
			return t.fetch(t.page - 1)
		}
		return nil
	case "l", "right":
		if t.page < t.pages {
			return t.fetch(t.page + 1)
		}
		return nil
	}
	if t.list.Update(msg) {
		return openBookmark(t.items[t.list.Cursor()])
	}
	return nil
}

// openBookmark returns the intent for the content of a bookmark.
func openBookmark(b gen.Bookmark) tea.Cmd {
	var msg tea.Msg
	switch b.ContentType {
	case gen.BookmarkContentTypePost:
		if b.ThreadId == nil {
			return nil
		}
		msg = ui.OpenThreadMsg{ThreadID: *b.ThreadId, PostID: b.ContentId}
	case gen.BookmarkContentTypeThread:
		msg = ui.OpenThreadMsg{ThreadID: b.ContentId}
	case gen.BookmarkContentTypeConversation:
		msg = ui.OpenConversationMsg{ID: b.ContentId}
	default:
		return nil
	}
	return func() tea.Msg { return msg }
}

func (t *bookmarksTab) view(width, height int) string {
	if v, ok := t.load.view(width, height); ok {
		return v
	}
	s := t.d.Styles
	if len(t.items) == 0 {
		return ui.Empty(s, i18n.T("social.profile.noBookmarks"), width, height)
	}
	t.list.SetHeight(max(height-2, 1))
	rows := t.list.View(s, t.d.Theme, width, t.d.Phase(), func(i, _ int) string {
		b := t.items[i]
		return s.Subtitle.Render(b.Title) + "  " + s.Text.Render(plainText(b.Excerpt)) +
			s.Muted.Render("  · "+api.AuthorName(b.Author)+" · "+relTime(t.d, b.CreatedAt))
	})
	page := s.Muted.Render(i18n.Tf("social.page", map[string]any{"Page": t.page, "Pages": t.pages}))
	return rows + "\n\n" + t.d.Mark(t.zid+"page", page)
}

type watchedMsg struct {
	page *api.CursorPage[gen.ThreadWatch]
	more bool
	err  error
}

type unwatchedMsg struct {
	threadID int
	err      error
}

// watchedTab lists the threads that the viewer watches. The next page
// loads at the end of the list.
type watchedTab struct {
	d     *ui.Deps
	load  loader
	items []gen.ThreadWatch
	next  *string
	more  bool
	list  ui.List
	pg    pager
	zid   string // the prefix of the zone IDs
}

func newWatchedTab(d *ui.Deps) *watchedTab {
	t := &watchedTab{d: d, load: newLoader(d), zid: ui.ZonePrefix()}
	t.list.SetZones(d, t.zid+"row")
	return t
}

func (t *watchedTab) name() string   { return i18n.T("social.profile.watched") }
func (t *watchedTab) start() tea.Cmd { return t.fetch("") }

func (t *watchedTab) fetch(cursor string) tea.Cmd {
	client, limit := t.d.API, t.limit()
	var spin tea.Cmd
	if cursor == "" {
		spin = t.load.begin()
	} else {
		t.more = true
	}
	return tea.Batch(spin, func() tea.Msg {
		page, err := client.WatchedThreads(bg, cursor, limit)
		return watchedMsg{page: page, more: cursor != "", err: err}
	})
}

func (t *watchedTab) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		return t.load.spin(msg)
	case watchedMsg:
		return t.loaded(msg)
	case unwatchedMsg:
		return t.unwatched(msg)
	case tea.MouseWheelMsg:
		t.list.Update(msg)
	}
	return nil
}

func (t *watchedTab) loaded(msg watchedMsg) tea.Cmd {
	t.more = false
	if msg.err != nil && msg.more {
		return ui.ErrorCmd(msg.err)
	}
	if cmd := t.load.done(msg.err); msg.err != nil {
		return cmd
	}
	if !msg.more {
		t.items, t.pg = nil, pager{per: t.pg.per}
		t.list.Reset()
	}
	t.items = append(t.items, msg.page.Items...)
	t.next = msg.page.NextCursor
	t.list.SetLen(len(t.items))
	return nil
}

func (t *watchedTab) unwatched(msg unwatchedMsg) tea.Cmd {
	if msg.err != nil {
		return ui.ErrorCmd(msg.err)
	}
	for i, w := range t.items {
		if w.ThreadId == msg.threadID {
			t.items = append(t.items[:i], t.items[i+1:]...)
			break
		}
	}
	t.list.SetLen(len(t.items))
	return ui.Toast(i18n.T("social.profile.unwatched"), ui.ToastSuccess)
}

func (t *watchedTab) key(msg tea.KeyPressMsg) tea.Cmd {
	if cmd, stop := t.load.blocked(msg, func() tea.Cmd {
		return t.fetch("")
	}); stop {
		return cmd
	}
	if msg.String() == "x" && len(t.items) > 0 {
		client, id := t.d.API, t.items[t.list.Cursor()].ThreadId
		return func() tea.Msg { return unwatchedMsg{threadID: id, err: client.UnwatchThread(bg, id)} }
	}
	if t.list.Update(msg) {
		id := t.items[t.list.Cursor()].ThreadId
		return func() tea.Msg { return ui.OpenThreadMsg{ThreadID: id} }
	}
	if t.next != nil && !t.more && t.list.Cursor() == t.list.Len()-1 {
		return t.fetch(*t.next)
	}
	return nil
}

func (t *watchedTab) view(width, height int) string {
	if v, ok := t.load.view(width, height); ok {
		return v
	}
	s := t.d.Styles
	if len(t.items) == 0 {
		return ui.Empty(s, i18n.T("social.profile.noWatched"), width, height)
	}
	t.list.SetHeight(max(height-1, 1))
	rows := t.list.View(s, t.d.Theme, width, t.d.Phase(), func(i, _ int) string {
		w := t.items[i]
		last := i18n.Tf("social.profile.lastPost", map[string]any{"When": relTime(t.d, w.LastPostAt)})
		return s.Text.Render(w.Title) + s.Muted.Render("  · "+last)
	})
	if t.more {
		rows += "\n" + s.Muted.Render(i18n.T("social.loadingMore"))
	}
	return rows
}
