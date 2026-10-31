package forum

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/paginator"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/api/gen"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// maxDots is the largest page count that shows as paginator dots.
const maxDots = 20

// threadList shows one page of the threads of a forum.
type threadList struct {
	d        *ui.Deps
	nodeID   int
	sp       spinner.Model
	loading  bool
	err      error
	view     *api.NodeView
	watching bool
	prefixes []gen.ThreadPrefix
	prefixID int
	page     int
	pages    int
	rows     []gen.Thread
	list     ui.List
	picker   *picker
}

type threadsMsg struct {
	view *api.NodeView
	list *api.ThreadList
	page int
	err  error
	// refresh is true for a new load of the shown page. It keeps the
	// cursor and the rows on an error.
	refresh bool
}

type prefixesMsg struct{ items []gen.ThreadPrefix }

type forumMarkedMsg struct{ err error }

// watchedMsg is the result of a watch or unwatch call.
type watchedMsg struct {
	on, forum bool
	err       error
}

func (m watchedMsg) cmd() tea.Cmd {
	if m.err != nil {
		return ui.ErrorCmd(m.err)
	}
	ids := map[[2]bool]string{
		{true, true}: "forum.toast.watchingForum", {false, true}: "forum.toast.unwatchedForum",
		{true, false}: "forum.toast.watchingThread", {false, false}: "forum.toast.unwatchedThread",
	}
	return ui.Toast(i18n.T(ids[[2]bool{m.on, m.forum}]), ui.ToastSuccess)
}

// setForumWatch watches the forum when on is true, else it stops the watch.
func setForumWatch(c *api.Client, nodeID int, on bool) watchedMsg {
	var err error
	if on {
		err = c.WatchNode(bg(), nodeID, gen.NodesWatchJSONBodyModeThreads, false)
	} else {
		err = c.UnwatchNode(bg(), nodeID)
	}
	return watchedMsg{on: on, forum: true, err: err}
}

// watchForum reads the watch state of a forum and changes it.
func watchForum(d *ui.Deps, nodeID int) tea.Cmd {
	if d.Me == nil {
		return loginToast()
	}
	c := d.API
	return func() tea.Msg {
		v, err := c.Node(bg(), nodeID)
		if err != nil {
			return watchedMsg{forum: true, err: err}
		}
		return setForumWatch(c, nodeID, v.Watching == nil)
	}
}

// NewNode returns the thread list of the node nodeID.
func NewNode(d *ui.Deps, nodeID int) ui.Screen {
	s := &threadList{d: d, nodeID: nodeID, sp: ui.NewSpinner(d.Styles), loading: true, page: 1, pages: 1}
	s.list.SetClock(d.Now)
	return s
}

func (s *threadList) Init() tea.Cmd {
	cmds := []tea.Cmd{s.load(1), s.loadPrefixes()}
	if s.d.Animate() {
		cmds = append(cmds, s.sp.Tick)
	}
	return tea.Batch(cmds...)
}

func (s *threadList) Title() string {
	if s.view != nil {
		return s.view.Node.Title
	}
	return i18n.T("forum.node.title")
}

func (s *threadList) CapturesInput() bool { return s.picker != nil }
func (s *threadList) Animating() bool {
	return s.d.Animate() && !s.loading && s.list.Animating(s.d.Now())
}

func (s *threadList) Keys() []key.Binding {
	return append(ui.ListKeys(),
		key.NewBinding(key.WithKeys("h", "l"), key.WithHelp("h l", i18n.T("forum.key.pages"))),
		key.NewBinding(key.WithKeys("x"), key.WithHelp("X", i18n.T("forum.key.prefix"))),
		key.NewBinding(key.WithKeys("t"), key.WithHelp("T", i18n.T("forum.key.newThread"))),
		key.NewBinding(key.WithKeys("a"), key.WithHelp("A", i18n.T("forum.key.markRead"))),
		key.NewBinding(key.WithKeys("w"), key.WithHelp("W", i18n.T("forum.key.watch"))),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("R", i18n.T("forum.key.retry"))),
	)
}

// load gets one page. The first call also gets the node.
func (s *threadList) load(page int) tea.Cmd {
	s.loading, s.err = true, nil
	c, id, prefix, view := s.d.API, s.nodeID, s.prefixID, s.view
	return func() tea.Msg {
		if view == nil {
			v, err := c.Node(bg(), id)
			if err != nil {
				return threadsMsg{err: err}
			}
			view = v
		}
		list, err := c.Threads(bg(), id, prefix, api.PageOpts{Page: page})
		return threadsMsg{view: view, list: list, page: page, err: err}
	}
}

// refresh loads the shown page again without the spinner.
func (s *threadList) refresh() tea.Cmd {
	if s.loading || s.err != nil || s.view == nil {
		return nil
	}
	c, id, prefix, view, page := s.d.API, s.nodeID, s.prefixID, s.view, s.page
	return func() tea.Msg {
		list, err := c.Threads(bg(), id, prefix, api.PageOpts{Page: page})
		return threadsMsg{view: view, list: list, page: page, err: err, refresh: true}
	}
}

func (s *threadList) loadPrefixes() tea.Cmd {
	c, id := s.d.API, s.nodeID
	return func() tea.Msg {
		items, _ := c.ThreadPrefixes(bg(), id)
		return prefixesMsg{items}
	}
}

func (s *threadList) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.list.SetHeight(msg.Height - 4)
	case threadsMsg:
		return s, s.loaded(msg)
	case ui.ResumedMsg:
		return s, s.refresh()
	case prefixesMsg:
		s.prefixes = msg.items
	case forumMarkedMsg:
		return s, s.marked(msg.err)
	case watchedMsg:
		return s, s.watched(msg)
	case spinner.TickMsg:
		return s, spin(s.d, &s.sp, s.loading, msg)
	case tea.KeyPressMsg:
		return s, s.key(msg)
	case tea.MouseWheelMsg:
		s.list.Update(msg)
	}
	return s, nil
}

// watched sets the watch state of the forum. A result for a thread does
// not change it.
func (s *threadList) watched(msg watchedMsg) tea.Cmd {
	if msg.err == nil && msg.forum {
		s.watching = msg.on
	}
	return msg.cmd()
}

func (s *threadList) loaded(msg threadsMsg) tea.Cmd {
	s.loading = false
	if msg.err != nil {
		if !msg.refresh {
			s.err = msg.err
		}
		return ui.ErrorCmd(msg.err)
	}
	if s.view == nil {
		s.watching = msg.view.Watching != nil
	}
	s.view, s.page = msg.view, msg.page
	s.rows = append(append([]gen.Thread{}, msg.list.Sticky...), msg.list.Items...)
	switch {
	case msg.list.Total != nil:
		s.pages = pages(*msg.list.Total)
	case msg.list.NextCursor != nil:
		s.pages = max(s.pages, s.page+1)
	}
	s.list.SetLen(len(s.rows))
	if msg.refresh {
		return nil
	}
	s.list.Select(0)
	return s.warmNextPage()
}

func (s *threadList) marked(err error) tea.Cmd {
	if err != nil {
		return ui.ErrorCmd(err)
	}
	for i := range s.rows {
		s.rows[i].IsUnread = false
	}
	return toast("forum.toast.markedRead")
}

// threadListKeys maps the action keys of the thread list to their methods.
var threadListKeys = map[string]func(*threadList) tea.Cmd{
	"h": func(s *threadList) tea.Cmd { return s.turn(-1) }, "left": func(s *threadList) tea.Cmd { return s.turn(-1) },
	"pgup": func(s *threadList) tea.Cmd { return s.turn(-1) },
	"l":    func(s *threadList) tea.Cmd { return s.turn(1) }, "right": func(s *threadList) tea.Cmd { return s.turn(1) },
	"pgdown": func(s *threadList) tea.Cmd { return s.turn(1) },
	"x":      (*threadList).openPicker,
	"t":      (*threadList).newThread,
	"a":      (*threadList).markRead,
	"w":      (*threadList).watch,
}

func (s *threadList) key(k tea.KeyPressMsg) tea.Cmd {
	switch {
	case s.picker != nil:
		return s.pick(k)
	case s.err != nil:
		if k.String() == "r" {
			return s.load(s.page)
		}
		return nil
	case s.loading:
		return nil
	}
	if fn, ok := threadListKeys[k.String()]; ok {
		return fn(s)
	}
	if s.list.Update(k) && len(s.rows) > 0 {
		id := s.rows[s.list.Cursor()].Id
		return func() tea.Msg { return ui.OpenThreadMsg{ThreadID: id} }
	}
	return nil
}

func (s *threadList) turn(dir int) tea.Cmd {
	p := s.page + dir
	if p < 1 || p > s.pages {
		return nil
	}
	return s.load(p)
}

func (s *threadList) openPicker() tea.Cmd {
	if len(s.prefixes) == 0 {
		return toast("forum.toast.noPrefixes")
	}
	items := []string{i18n.T("forum.node.allPrefixes")}
	cursor := 0
	for i, p := range s.prefixes {
		items = append(items, prefixTag(s.d, &gen.ThreadPrefixTag{Label: p.Label, Color: p.Color}))
		if p.Id == s.prefixID {
			cursor = i + 1
		}
	}
	s.picker = newPicker(i18n.T("forum.node.pickPrefix"), items, cursor)
	return nil
}

func (s *threadList) pick(msg tea.Msg) tea.Cmd {
	i, done := s.picker.update(msg)
	if !done {
		return nil
	}
	s.picker = nil
	if i < 0 {
		return nil
	}
	s.prefixID = 0
	if i > 0 {
		s.prefixID = s.prefixes[i-1].Id
	}
	return s.load(1)
}

func (s *threadList) newThread() tea.Cmd {
	switch {
	case s.d.Me == nil:
		return loginToast()
	case !s.view.Node.Permissions.CanPost:
		return toast("forum.toast.cannotPost")
	}
	return ui.Push(newThreadComposer(s.d, s.nodeID))
}

func (s *threadList) markRead() tea.Cmd {
	if s.d.Me == nil {
		return loginToast()
	}
	c, id := s.d.API, s.nodeID
	return func() tea.Msg { return forumMarkedMsg{c.MarkNodeRead(bg(), id)} }
}

func (s *threadList) watch() tea.Cmd {
	if s.d.Me == nil {
		return loginToast()
	}
	c, id, on := s.d.API, s.nodeID, !s.watching
	return func() tea.Msg { return setForumWatch(c, id, on) }
}

func (s *threadList) View(width, height int) string {
	switch {
	case s.loading && s.view == nil:
		return ui.Loading(s.d.Styles, s.sp, "", width, height)
	case s.err != nil:
		return errorView(s.d, s.err, width, height)
	case s.picker != nil:
		return s.picker.view(s.d, width, height)
	}
	st, t := s.d.Styles, s.d.Theme
	body := ui.Empty(st, i18n.T("forum.node.empty"), width, max(height-4, 1))
	if len(s.rows) > 0 {
		body = s.list.View(st, t, width, s.d.Phase(), s.row)
	}
	lines := strings.Count(body, "\n") + 1
	return ui.Header(st, t, s.breadcrumbs(), width) + "\n\n" + body +
		strings.Repeat("\n", max(height-3-lines, 1)) + s.footer()
}

func (s *threadList) breadcrumbs() string {
	parts := make([]string, 0, len(s.view.Breadcrumbs)+1)
	for _, b := range s.view.Breadcrumbs {
		parts = append(parts, b.Title)
	}
	return strings.Join(append(parts, s.view.Node.Title), " › ")
}

func (s *threadList) footer() string {
	st := s.d.Styles
	out := ""
	if s.pages > 1 && s.pages <= maxDots {
		p := paginator.New(paginator.WithTotalPages(s.pages))
		p.Type = paginator.Dots
		p.Page = s.page - 1
		p.ActiveDot = st.Accent.Render("•")
		p.InactiveDot = st.Muted.Render("•")
		out = p.View() + "  "
	}
	out += st.Text.Render(i18n.Tf("forum.thread.page", map[string]any{"Page": s.page, "Pages": s.pages}))
	if s.prefixID != 0 {
		for _, p := range s.prefixes {
			if p.Id == s.prefixID {
				out += st.Muted.Render("  " + i18n.Tf("forum.node.filter", map[string]any{"Label": p.Label}))
			}
		}
	}
	if s.watching {
		out += st.Accent.Render("  " + i18n.T("forum.node.watching"))
	}
	return out
}

func (s *threadList) row(i, w int) string {
	th := s.rows[i]
	st := s.d.Styles
	marks := " "
	if th.IsUnread {
		marks = st.Accent.Bold(true).Render(glyphUnread)
	}
	if th.IsSticky {
		marks += st.Accent.Render(glyphSticky)
	} else {
		marks += " "
	}
	if th.IsLocked {
		marks += st.Warning.Render(glyphLocked)
	} else {
		marks += " "
	}
	title := st.Text.Render(th.Title)
	if th.IsUnread {
		title = st.Subtitle.Render(th.Title)
	}
	left := marks + " " + prefixTag(s.d, th.Prefix) + title + st.Muted.Render(" · "+authorName(th.Author))
	right := st.Muted.Render(i18n.Plural("forum.threads.replies", th.ReplyCount, nil))
	if w >= 70 {
		right += st.Muted.Render("  " + authorName(th.LastPost.User) + " · " + ago(s.d.Now(), th.LastPost.PostedAt))
	}
	return fit(left, right, w)
}
