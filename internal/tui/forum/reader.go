package forum

import (
	"image/color"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/api/gen"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/render"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// The kinds of picker in the reader.
const (
	pickReaction = iota
	pickLink
)

// reader shows one page of the posts of a thread in a viewport.
type reader struct {
	d        *ui.Deps
	threadID int
	// startPost and startPos select the first page. A startPos of -1 is
	// not known.
	startPost, startPos int

	sp      spinner.Model
	loading bool
	err     error

	view     *api.ThreadView
	watching bool
	rtypes   []gen.ReactionType
	accents  []color.Color

	page, pages int
	posts       []gen.Post
	bodies      []renderedBody
	starts      []int
	focus       int
	bookmarked  map[int]bool
	spoilers    map[int]string
	marked      map[int]bool
	// cards and sigs hold the author cards and the rendered signatures
	// of the BBS interface by user ID.
	cards map[int]*api.Card
	sigs  map[int][]string

	vp       viewport.Model
	w, h     int
	picker   *picker
	pickKind int
	pickIDs  []int
	zid      string // the prefix of the zone IDs
}

type renderedBody struct {
	text  string
	links []render.Link
}

// pageData is one loaded page of posts. focusID is the post to show.
type pageData struct {
	page       int
	posts      []gen.Post
	bookmarked []int
	focusID    int
}

// readerMsg is the result of a load. A nil view keeps the thread data.
type readerMsg struct {
	view    *api.ThreadView
	rtypes  []gen.ReactionType
	accents []color.Color
	page    pageData
	err     error
}

// NewThread returns the reader for the thread threadID. A postID of 0 opens
// the first unread post.
func NewThread(d *ui.Deps, threadID, postID int) ui.Screen {
	return newReader(d, threadID, postID, -1)
}

// newThreadAt returns the reader on the page of the post at position.
func newThreadAt(d *ui.Deps, threadID, postID, position int) ui.Screen {
	return newReader(d, threadID, postID, position)
}

func newReader(d *ui.Deps, threadID, postID, position int) *reader {
	vp := viewport.New()
	vp.KeyMap = viewport.KeyMap{
		PageDown:     key.NewBinding(key.WithKeys("pgdown", "space")),
		PageUp:       key.NewBinding(key.WithKeys("pgup")),
		HalfPageDown: key.NewBinding(key.WithKeys("ctrl+d")),
		HalfPageUp:   key.NewBinding(key.WithKeys("ctrl+u")),
		Down:         key.NewBinding(key.WithKeys("down", "j")),
		Up:           key.NewBinding(key.WithKeys("up", "k")),
	}
	return &reader{
		d: d, threadID: threadID, startPost: postID, startPos: position,
		sp: ui.NewSpinner(d.Styles), loading: true, vp: vp, page: 1, pages: 1,
		bookmarked: map[int]bool{}, spoilers: map[int]string{}, marked: map[int]bool{},
		zid: ui.ZonePrefix(),
	}
}

func (r *reader) Init() tea.Cmd {
	if r.d.Animate() {
		return tea.Batch(r.open(), r.sp.Tick)
	}
	return r.open()
}

func (r *reader) Title() string {
	if r.view != nil {
		return r.view.Thread.Title
	}
	return i18n.T("forum.thread.title")
}

func (r *reader) CapturesInput() bool { return r.picker != nil && !r.d.BBS() }

func (r *reader) Keys() []key.Binding {
	b := func(keys []string, help, id string) key.Binding {
		return key.NewBinding(key.WithKeys(keys...), key.WithHelp(help, i18n.T(id)))
	}
	return []key.Binding{
		b([]string{"j", "k", "space", "ctrl+d", "ctrl+u"}, "j k Space Ctrl+D Ctrl+U", "forum.key.scroll"),
		b([]string{"[", "]"}, "[ ]", "forum.key.posts"),
		b([]string{"h", "l"}, "h l", "forum.key.pages"),
		b([]string{"u"}, "U", "forum.key.unread"),
		b([]string{"r"}, "R", "forum.key.reply"),
		b([]string{"c"}, "C", "forum.key.quote"),
		b([]string{"e"}, "E", "forum.key.edit"),
		b([]string{"+"}, "+", "forum.key.react"),
		b([]string{"*"}, "*", "forum.key.bookmark"),
		b([]string{"w"}, "W", "forum.key.watch"),
		b([]string{"o"}, "O", "forum.key.link"),
		b([]string{"a"}, "A", "forum.key.author"),
		b([]string{"v"}, "V", "forum.key.poll"),
		b([]string{"s"}, "S", "forum.key.spoilers"),
	}
}

func (r *reader) member() bool { return r.d.Me != nil }

// open loads the thread, finds the first page and loads it.
func (r *reader) open() tea.Cmd {
	r.loading, r.err = true, nil
	d, id, postID, pos, member := r.d, r.threadID, r.startPost, r.startPos, r.member()
	return func() tea.Msg {
		view, err := d.API.Thread(bg(), id)
		if err != nil {
			return readerMsg{err: err}
		}
		page, focus, err := startPage(d.API, id, postID, pos, member)
		if err != nil {
			return readerMsg{err: err}
		}
		data, err := fetchPage(d, id, page, focus)
		msg := readerMsg{view: view, page: data, err: err}
		msg.rtypes, _ = d.API.ReactionTypes(bg())
		if pal, err := d.API.Palette(bg(), gen.PaletteColorsListParamsKindAccent); err == nil {
			msg.accents = paletteColors(pal)
		}
		return msg
	}
}

// startPage returns the page and the post to show first.
func startPage(c *api.Client, threadID, postID, pos int, member bool) (int, int, error) {
	switch {
	case pos >= 0:
		return pageOf(pos), postID, nil
	case postID != 0:
		p, err := c.Post(bg(), postID)
		if err != nil {
			return 0, 0, err
		}
		return pageOf(p.Position), postID, nil
	case member:
		u, err := c.LocateUnread(bg(), threadID)
		if err != nil {
			return 1, 0, nil
		}
		return pageOf(u.Position), u.PostId, nil
	}
	return 1, 0, nil
}

// fetchPage loads one page of posts and, for a member, their bookmarks.
func fetchPage(d *ui.Deps, threadID, page, focusID int) (pageData, error) {
	list, err := d.API.Posts(bg(), threadID, api.PageOpts{Page: page})
	if err != nil {
		return pageData{}, err
	}
	data := pageData{page: page, posts: list.Items, focusID: focusID}
	if d.Me != nil && len(list.Items) > 0 {
		ids := make([]int, len(list.Items))
		for i, p := range list.Items {
			ids[i] = p.Id
		}
		data.bookmarked, _ = d.API.BookmarkStatus(bg(), gen.BookmarksStatusParamsContentTypePost, ids)
	}
	return data, nil
}

// loadPage loads page and shows the post focusID, or the first post.
func (r *reader) loadPage(page, focusID int) tea.Cmd {
	r.loading = true
	d, id := r.d, r.threadID
	return func() tea.Msg {
		data, err := fetchPage(d, id, page, focusID)
		return readerMsg{page: data, err: err}
	}
}

// reloadAt loads the thread again and shows the post at position. The
// composer uses it after a reply or an edit.
func (r *reader) reloadAt(position, postID int) tea.Cmd {
	d, id := r.d, r.threadID
	return func() tea.Msg {
		view, err := d.API.Thread(bg(), id)
		if err != nil {
			return readerMsg{err: err}
		}
		data, err := fetchPage(d, id, pageOf(position), postID)
		return readerMsg{view: view, page: data, err: err}
	}
}

func (r *reader) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.resize(msg.Width, msg.Height)
	case readerMsg:
		return r, r.loaded(msg)
	case spinner.TickMsg:
		return r, spin(r.d, &r.sp, r.loading, msg)
	case tea.KeyPressMsg:
		return r, r.key(msg)
	case tea.MouseWheelMsg:
		r.scroll(msg)
	case tea.MouseReleaseMsg:
		return r, r.click(msg)
	default:
		return r, r.updateAction(msg)
	}
	return r, nil
}

func (r *reader) resize(w, h int) {
	r.w, r.h = w, h
	r.vp.SetWidth(w)
	r.vp.SetHeight(max(h-r.headerLines(), 1))
	if len(r.posts) > 0 {
		r.renderAll()
	}
}

func (r *reader) loaded(msg readerMsg) tea.Cmd {
	r.loading = false
	if msg.err != nil {
		r.err = msg.err
		return ui.ErrorCmd(msg.err)
	}
	r.err = nil
	if msg.view != nil {
		r.view = msg.view
		r.watching = msg.view.Watching != nil
		r.pages = pages(msg.view.Thread.ReplyCount + 1)
	}
	if msg.rtypes != nil {
		r.rtypes, r.accents = msg.rtypes, msg.accents
	}
	r.page, r.posts = msg.page.page, msg.page.posts
	r.bookmarked = map[int]bool{}
	for _, id := range msg.page.bookmarked {
		r.bookmarked[id] = true
	}
	r.focus = 0
	for i, p := range r.posts {
		if p.Id == msg.page.focusID {
			r.focus = i
		}
	}
	r.renderAll()
	r.jumpTo(r.focus)
	return tea.Batch(r.markRead(r.nextPostsWarm()), r.loadCards())
}

// markRead marks the thread read up to the last post of the page. It runs
// once for each page and only for a member. Then it calls then, if not nil.
// A write clears the page cache, so a warm must start after the write.
func (r *reader) markRead(then func()) tea.Cmd {
	if !r.member() || len(r.posts) == 0 || r.marked[r.page] {
		return warmCmd(then)
	}
	r.marked[r.page] = true
	c, id, pos := r.d.API, r.threadID, r.posts[len(r.posts)-1].Position
	return func() tea.Msg {
		_, err := c.MarkThreadRead(bg(), id, pos)
		if api.IsUnauthorized(err) {
			return ui.UnauthorizedMsg{}
		}
		if then != nil {
			then()
		}
		return nil
	}
}

// focusedPost returns the post with the focus marker.
func (r *reader) focusedPost() *gen.Post {
	if r.focus < 0 || r.focus >= len(r.posts) {
		return nil
	}
	return &r.posts[r.focus]
}

// jumpTo puts the focus on post i and scrolls to it.
func (r *reader) jumpTo(i int) {
	if len(r.posts) == 0 {
		return
	}
	r.focus = max(0, min(i, len(r.posts)-1))
	r.assemble()
	r.vp.SetYOffset(r.starts[r.focus])
}

// scroll moves the viewport. When the offset changes, the focus goes to
// the post at the top of the viewport.
func (r *reader) scroll(msg tea.Msg) {
	before := r.vp.YOffset()
	r.vp, _ = r.vp.Update(msg)
	if r.vp.YOffset() == before {
		return
	}
	focus := 0
	for i, s := range r.starts {
		if s <= r.vp.YOffset() {
			focus = i
		}
	}
	if focus != r.focus {
		r.focus = focus
		y := r.vp.YOffset()
		r.assemble()
		r.vp.SetYOffset(y)
	}
}
