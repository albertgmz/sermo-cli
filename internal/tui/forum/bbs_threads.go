package forum

import (
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/api/gen"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// hotReplies is the reply count from which a thread is hot.
const hotReplies = 100

// threadListFixed is the number of lines of the BBS thread list that are
// not thread rows.
const threadListFixed = 9

// maxBBSPage is the largest page size that the BBS thread list asks for.
const maxBBSPage = 50

// bbsList is the state of the thread list in the BBS interface.
type bbsList struct {
	height int
	// sticky is the largest count of sticky threads on page 1 so far.
	// Sticky threads take lines of the first page only, but all pages
	// must have the same size.
	sticky int
	// limit is the page size of the shown page, or 0 before the first
	// page.
	limit     int
	filtering bool
}

// pageLimit returns the page size for the height. It is 0 in the modern
// interface, so that the server uses its default.
func (s *threadList) pageLimit() int {
	if !s.d.BBS() {
		return 0
	}
	return min(max(s.bbs.height-threadListFixed-s.bbs.sticky, 1), maxBBSPage)
}

// pageCount returns the number of pages for total threads.
func (s *threadList) pageCount(total int) int {
	limit := s.pageLimit()
	if limit == 0 {
		return pages(total)
	}
	return max(1, (total+limit-1)/limit)
}

// fitPage makes the page fit the height in the BBS interface. The sticky
// threads of page 1 make the page size smaller. Then the rows of the
// first page that do not fit are on page 2. limit is the page size of
// the request.
func (s *threadList) fitPage(list *api.ThreadList, page, limit int) {
	if !s.d.BBS() {
		return
	}
	s.bbs.limit = limit
	if page == 1 {
		s.bbs.sticky = max(s.bbs.sticky, len(list.Sticky))
		s.bbs.limit = min(limit, s.pageLimit())
	}
	if limit := s.pageLimit(); len(list.Items) > limit {
		list.Items = list.Items[:limit]
	}
}

// refit loads the page that shows the first thread of the shown page when
// the page size is not the page size of the shown page, for example after
// a resize.
func (s *threadList) refit() tea.Cmd {
	limit := s.pageLimit()
	if s.loading || s.view == nil || s.bbs.limit == 0 || s.bbs.limit == limit {
		return nil
	}
	return s.load((s.page-1)*s.bbs.limit/limit + 1)
}

// Prompt is the label of the BBS prompt: the forum title.
func (s *threadList) Prompt() string { return s.Title() }

// Commands are the BBS commands of the thread list.
func (s *threadList) Commands() []ui.Command {
	if s.bbs.filtering {
		return []ui.Command{
			{Name: "#", Label: i18n.T("bbs.forum.cmd.pick"), Run: s.choosePrefix},
			{Name: "F", Label: i18n.T("bbs.forum.cmd.cancel"), Run: s.filter},
		}
	}
	member := s.d.Me != nil
	cmds := []ui.Command{{Name: "#", Label: i18n.T("bbs.forum.cmd.readThread"), Run: openThreadID}}
	if member && s.view != nil && s.view.Node.Permissions.CanPost {
		cmds = append(cmds, ui.Command{Name: "N", Label: i18n.T("bbs.forum.cmd.newThread"), Run: func(string) tea.Cmd { return s.newThread() }, NoCount: true})
	}
	if s.pages > 1 {
		cmds = append(cmds, pageCommands(s.turn)...)
	}
	cmds = append(cmds,
		ui.Command{Name: "F", Label: i18n.T("bbs.forum.cmd.filter"), Run: s.filter},
		ui.Command{Name: "I", Label: i18n.T("bbs.forum.cmd.index"), Run: func(string) tea.Cmd { return ui.Pop() }},
	)
	if member {
		watch := "bbs.forum.cmd.watch"
		if s.watching {
			watch = "bbs.forum.cmd.unwatch"
		}
		cmds = append(cmds,
			ui.Command{Name: "L", Label: i18n.T("bbs.forum.cmd.read"), Run: func(string) tea.Cmd { return s.markRead() }},
			ui.Command{Name: "W", Label: i18n.T(watch), Run: func(string) tea.Cmd { return s.watch() }},
		)
	}
	return append(cmds, ui.Command{Name: "R", Label: i18n.T("bbs.forum.cmd.refresh"), Run: s.reload})
}

// openThreadID opens the thread with the id arg.
func openThreadID(arg string) tea.Cmd {
	id, err := strconv.Atoi(arg)
	if err != nil || id <= 0 {
		return ui.Toast(i18n.T("bbs.unknown"), ui.ToastError)
	}
	return func() tea.Msg { return ui.OpenThreadMsg{ThreadID: id} }
}

// reload loads the page again after an error, else it refreshes it.
func (s *threadList) reload(string) tea.Cmd {
	if s.err != nil {
		return s.load(s.page)
	}
	return s.refresh()
}

// filter shows the numbered prefixes. With a number it selects that
// prefix. Without a number while the prefixes show, it closes them.
func (s *threadList) filter(arg string) tea.Cmd {
	switch {
	case len(s.prefixes) == 0:
		return toast("forum.toast.noPrefixes")
	case arg != "":
		return s.choosePrefix(arg)
	}
	s.bbs.filtering = !s.bbs.filtering
	return nil
}

// choosePrefix filters by the prefix with the number arg. 0 shows all
// threads.
func (s *threadList) choosePrefix(arg string) tea.Cmd {
	n, err := strconv.Atoi(arg)
	if err != nil || n < 0 || n > len(s.prefixes) {
		return ui.Toast(i18n.Tf("bbs.forum.noPrefix", map[string]any{"N": arg}), ui.ToastError)
	}
	s.bbs.filtering = false
	s.prefixID = 0
	if n > 0 {
		s.prefixID = s.prefixes[n-1].Id
	}
	return s.load(1)
}

// bbsView draws the thread list, or the prefix list, as an ASCII table.
func (s *threadList) bbsView(width int) string {
	if s.bbs.filtering {
		return s.prefixView(width)
	}
	t := ui.NewTable(width, s.threadColumns(width)...)
	t.Full(listSpread(s.bbsCrumbs(), s.bbsRight(), width-4)).Header()
	if len(s.rows) == 0 {
		t.Full(i18n.T("forum.node.empty"))
	}
	for k, th := range s.rows {
		if k > 0 && s.rows[k-1].IsSticky && !th.IsSticky {
			t.Rule()
		}
		t.Row(threadMark(th), prefixLabel(th.Prefix)+th.Title, authorName(th.Author),
			ui.Thousands(th.ReplyCount), listAgeLong(s.d.Now(), th.LastPost.PostedAt)).Zone(s.d, s.list.RowZone(k))
		if k == s.list.Cursor() {
			t.Select()
		}
	}
	t.Full(i18n.T("bbs.forum.legend.threads"))
	return t.Render(s.d.Styles)
}

func (s *threadList) threadColumns(width int) []ui.Column {
	idW, rw := 4, len(i18n.T("bbs.forum.col.replies"))
	for _, th := range s.rows {
		idW = max(idW, len(strconv.Itoa(th.Id))+1)
		rw = max(rw, len(ui.Thousands(th.ReplyCount)))
	}
	return []ui.Column{
		{Title: i18n.T("bbs.forum.col.id"), Width: idW},
		{Title: i18n.T("bbs.forum.col.topic")},
		{Title: i18n.T("bbs.forum.col.author"), Width: min(max(width/8, 10), 16)},
		{Title: i18n.T("bbs.forum.col.replies"), Width: rw, Align: ui.AlignRight},
		{Title: i18n.T("bbs.forum.col.lastShort"), Width: min(max(width/10, 8), 12)},
	}
}

// bbsCrumbs is the path of the forum from the index.
func (s *threadList) bbsCrumbs() string {
	parts := []string{i18n.T("bbs.forum.home")}
	for _, b := range s.view.Breadcrumbs {
		parts = append(parts, b.Title)
	}
	return strings.Join(append(parts, s.view.Node.Title), " > ")
}

// bbsRight is the right side of the path row: the prefix filter and the
// page.
func (s *threadList) bbsRight() string {
	out := s.d.Mark(s.zid+"page", i18n.Tf("bbs.forum.page", map[string]any{"Page": s.page, "Pages": s.pages}))
	for _, p := range s.prefixes {
		if p.Id == s.prefixID && s.prefixID != 0 {
			out = i18n.Tf("bbs.forum.filter.active", map[string]any{"Label": p.Label}) + "   " + out
		}
	}
	return out
}

// threadMark is the text of the # column: [!] for a sticky thread, else
// the thread id and a mark: * unread, x locked or # hot.
func threadMark(th gen.Thread) string {
	id := strconv.Itoa(th.Id)
	switch {
	case th.IsSticky:
		return "[!]"
	case th.IsUnread:
		return id + "*"
	case th.IsLocked:
		return id + "x"
	case th.ReplyCount >= hotReplies:
		return id + "#"
	}
	return id
}

// prefixLabel is the prefix of a thread in brackets, without color.
func prefixLabel(p *gen.ThreadPrefixTag) string {
	if p == nil {
		return ""
	}
	return "[" + p.Label + "] "
}

// prefixView lists the prefixes with a number. 0 shows all threads.
func (s *threadList) prefixView(width int) string {
	t := ui.NewTable(width, ui.Column{Title: i18n.T("bbs.forum.col.num"), Width: len(ui.Num(0, len(s.prefixes))), Align: ui.AlignRight},
		ui.Column{Title: i18n.T("bbs.forum.filter.title")})
	t.Full(listSpread(s.bbsCrumbs(), s.bbsRight(), width-4)).Header()
	t.Row(ui.Num(0, len(s.prefixes)), i18n.T("bbs.forum.filter.all")).Zone(s.d, s.prefixZone(0))
	for i, p := range s.prefixes {
		t.Row(ui.Num(i+1, len(s.prefixes)), "["+p.Label+"]").Zone(s.d, s.prefixZone(i+1))
		if p.Id == s.prefixID {
			t.Select()
		}
	}
	return t.Render(s.d.Styles)
}

// prefixZone is the zone ID of the row with the number n of the prefix
// list.
func (s *threadList) prefixZone(n int) string { return s.zid + "prefix." + strconv.Itoa(n) }

// clickPrefix filters by the clicked prefix, like its number.
func (s *threadList) clickPrefix(msg tea.MouseReleaseMsg) tea.Cmd {
	for n := range len(s.prefixes) + 1 {
		if s.d.Clicked(s.prefixZone(n), msg) {
			return s.choosePrefix(strconv.Itoa(n))
		}
	}
	return nil
}

// listAgeLong returns the time from t to now for the thread list, for
// example "hace 3m", "ayer" or "29-Oct".
func listAgeLong(now, t time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return i18n.T("bbs.forum.ago.now")
	case d < time.Hour:
		return i18n.Tf("bbs.forum.ago.minutes", map[string]any{"N": int(d.Minutes())})
	case d < 24*time.Hour:
		return i18n.Tf("bbs.forum.ago.hours", map[string]any{"N": int(d.Hours())})
	}
	y := now.AddDate(0, 0, -1)
	if lt := t.In(now.Location()); lt.Year() == y.Year() && lt.YearDay() == y.YearDay() {
		return i18n.T("bbs.forum.ago.yesterday")
	}
	return listDay(now, t)
}
