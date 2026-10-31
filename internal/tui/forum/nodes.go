// Package forum holds the screens for forum nodes, thread lists, threads and
// the composer.
package forum

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

// nodes is the home screen: the forum tree.
type nodes struct {
	d       *ui.Deps
	sp      spinner.Model
	loading bool
	err     error
	items   []gen.Node
	trees   []string
	unread  map[int]bool
	list    ui.List
}

type nodesMsg struct {
	items  []gen.Node
	unread []int
	err    error
}

type nodesMarkedMsg struct{ err error }

// unreadMsg is the result of a new load of the unread forums.
type unreadMsg struct {
	ids []int
	err error
}

// NewNodes returns the screen with the forum tree. It is the home screen.
func NewNodes(d *ui.Deps) ui.Screen {
	s := &nodes{d: d, sp: ui.NewSpinner(d.Styles), loading: true}
	s.list.SetClock(d.Now)
	return s
}

func (s *nodes) Init() tea.Cmd {
	cmd := s.load()
	if s.d.Animate() {
		return tea.Batch(cmd, s.sp.Tick)
	}
	return cmd
}

func (s *nodes) Title() string       { return i18n.T("forum.nodes.title") }
func (s *nodes) CapturesInput() bool { return false }
func (s *nodes) Animating() bool {
	return s.d.Animate() && !s.loading && s.list.Animating(s.d.Now())
}

func (s *nodes) Keys() []key.Binding {
	return append(ui.ListKeys(),
		key.NewBinding(key.WithKeys("a"), key.WithHelp("A", i18n.T("forum.key.markAllRead"))),
		key.NewBinding(key.WithKeys("w"), key.WithHelp("W", i18n.T("forum.key.watch"))),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("R", i18n.T("forum.key.retry"))),
	)
}

// load gets the tree and, with a credential, the unread forums. An error
// of the unread call does not stop the tree, except a 401.
func (s *nodes) load() tea.Cmd {
	s.loading, s.err = true, nil
	c, member := s.d.API, s.d.Cred.Token != ""
	return func() tea.Msg {
		items, err := c.Nodes(bg())
		if err != nil {
			return nodesMsg{err: err}
		}
		var ids []int
		if member {
			ids, err = c.UnreadNodeIDs(bg())
			if api.IsUnauthorized(err) {
				return nodesMsg{err: err}
			}
		}
		return nodesMsg{items: items, unread: ids}
	}
}

func (s *nodes) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.list.SetHeight(msg.Height - 2)
	case nodesMsg:
		return s, s.loaded(msg)
	case nodesMarkedMsg:
		return s, s.marked(msg.err)
	case ui.ResumedMsg:
		return s, s.loadUnread()
	case unreadMsg:
		return s, s.gotUnread(msg)
	case watchedMsg:
		return s, msg.cmd()
	case spinner.TickMsg:
		return s, spin(s.d, &s.sp, s.loading, msg)
	case tea.KeyPressMsg:
		return s, s.key(msg)
	case tea.MouseWheelMsg:
		return s, s.move(msg)
	}
	return s, nil
}

// loadUnread gets the unread forums again for a member. It does not load
// the tree again.
func (s *nodes) loadUnread() tea.Cmd {
	if s.loading || s.err != nil || s.d.Cred.Token == "" {
		return nil
	}
	c := s.d.API
	return func() tea.Msg {
		ids, err := c.UnreadNodeIDs(bg())
		return unreadMsg{ids: ids, err: err}
	}
}

// gotUnread sets the unread markers. An error keeps the old markers, except
// a 401.
func (s *nodes) gotUnread(msg unreadMsg) tea.Cmd {
	if msg.err != nil {
		if api.IsUnauthorized(msg.err) {
			return ui.ErrorCmd(msg.err)
		}
		return nil
	}
	s.setUnread(msg.ids)
	return nil
}

func (s *nodes) setUnread(ids []int) {
	s.unread = map[int]bool{}
	for _, id := range ids {
		s.unread[id] = true
	}
}

func (s *nodes) marked(err error) tea.Cmd {
	if err != nil {
		return ui.ErrorCmd(err)
	}
	s.unread = map[int]bool{}
	return toast("forum.toast.markedRead")
}

func (s *nodes) loaded(msg nodesMsg) tea.Cmd {
	s.loading = false
	if msg.err != nil {
		s.err = msg.err
		if api.IsUnauthorized(msg.err) {
			return ui.ErrorCmd(msg.err)
		}
		return nil
	}
	s.items, s.trees = msg.items, treeGlyphs(msg.items)
	s.setUnread(msg.unread)
	s.list.SetLen(len(s.items))
	s.skipCategory(1)
	return nil
}

func (s *nodes) key(k tea.KeyPressMsg) tea.Cmd {
	switch k.String() {
	case "r":
		if s.err != nil {
			return s.load()
		}
	case "a":
		return s.markAll()
	case "w":
		if n, ok := s.selected(); ok {
			return watchForum(s.d, n.Id)
		}
	default:
		return s.move(k)
	}
	return nil
}

func (s *nodes) markAll() tea.Cmd {
	if s.d.Me == nil {
		return loginToast()
	}
	c := s.d.API
	return func() tea.Msg { return nodesMarkedMsg{c.MarkAllRead(bg())} }
}

// move moves the cursor and skips the category rows in the same
// direction. It opens the forum on Enter.
func (s *nodes) move(msg tea.Msg) tea.Cmd {
	if s.loading || s.err != nil {
		return nil
	}
	before := s.list.Cursor()
	chosen := s.list.Update(msg)
	s.skipCategory(s.list.Cursor() - before)
	n, ok := s.selected()
	if !chosen || !ok {
		return nil
	}
	return func() tea.Msg { return ui.OpenNodeMsg{NodeID: n.Id} }
}

func (s *nodes) skipCategory(dir int) {
	step := 1
	if dir < 0 {
		step = -1
	}
	for _, st := range []int{step, -step} {
		for i := s.list.Cursor(); i >= 0 && i < len(s.items); i += st {
			if s.items[i].Type != gen.NodeTypeCategory {
				s.list.Select(i)
				return
			}
		}
	}
}

func (s *nodes) selected() (gen.Node, bool) {
	i := s.list.Cursor()
	if i < 0 || i >= len(s.items) || s.items[i].Type == gen.NodeTypeCategory {
		return gen.Node{}, false
	}
	return s.items[i], true
}

func (s *nodes) View(width, height int) string {
	switch {
	case s.loading:
		return ui.Loading(s.d.Styles, s.sp, "", width, height)
	case s.err != nil:
		return errorView(s.d, s.err, width, height)
	case len(s.items) == 0:
		return ui.Empty(s.d.Styles, i18n.T("forum.nodes.empty"), width, height)
	}
	st, t := s.d.Styles, s.d.Theme
	return ui.Header(st, t, s.Title(), width) + "\n\n" + s.list.View(st, t, width, s.d.Phase(), s.row)
}

func (s *nodes) row(i, w int) string {
	n := s.items[i]
	st := s.d.Styles
	if n.Type == gen.NodeTypeCategory {
		return ui.Header(st, s.d.Theme, n.Title, w)
	}
	marker, title := st.Muted.Render(glyphRead), st.Text.Render(n.Title)
	if s.unread[n.Id] {
		marker, title = st.Accent.Bold(true).Render(glyphUnread), st.Subtitle.Render(n.Title)
	}
	left := st.Muted.Render(s.trees[i]) + marker + " " + title
	if desc := oneLine(n.Description); desc != "" {
		left += st.Muted.Render(" — " + desc)
	}
	return fit(left, s.stats(n, w), w)
}

// stats returns the counts and the last post. A narrow row shows only the
// counts.
func (s *nodes) stats(n gen.Node, w int) string {
	st := s.d.Styles
	out := st.Muted.Render(i18n.Tf("forum.nodes.counts", map[string]any{"Threads": n.ThreadCount, "Posts": n.PostCount}))
	if n.LastPost == nil || w < 90 {
		return out
	}
	lp := n.LastPost
	title := []rune(lp.ThreadTitle)
	if len(title) > 22 {
		title = append(title[:21], '…')
	}
	return out + st.Muted.Render(" │ ") + st.Text.Render(string(title)) +
		st.Muted.Render(" · "+authorName(lp.User)+" · "+ago(s.d.Now(), lp.PostedAt))
}

// treeGlyphs returns the tree lines in front of each node of the flat
// depth-first list. Depth 0 has no glyph.
func treeGlyphs(items []gen.Node) []string {
	out := make([]string, len(items))
	open := map[int]bool{} // open[k] is true while level k has more siblings
	for i, n := range items {
		if n.Depth == 0 {
			continue
		}
		last := isLastSibling(items, i)
		var b strings.Builder
		for k := 1; k < n.Depth; k++ {
			if open[k] {
				b.WriteString("│  ")
			} else {
				b.WriteString("   ")
			}
		}
		if last {
			b.WriteString("└─ ")
		} else {
			b.WriteString("├─ ")
		}
		open[n.Depth] = !last
		out[i] = b.String()
	}
	return out
}

func isLastSibling(items []gen.Node, i int) bool {
	for j := i + 1; j < len(items); j++ {
		if items[j].Depth < items[i].Depth {
			return true
		}
		if items[j].Depth == items[i].Depth {
			return false
		}
	}
	return true
}
