package forum

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/api/gen"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// bbsIndex is the state of the forum index in the BBS interface.
type bbsIndex struct {
	height int
	page   int
	online *api.OnlinePage
	stats  *api.SiteStats
}

// indexOnlineMsg is the result of the online list. It is nil on an error.
type indexOnlineMsg struct{ page *api.OnlinePage }

// indexStatsMsg is the result of the site totals. It is nil on an error.
type indexStatsMsg struct{ stats *api.SiteStats }

// loadExtras gets the online list and the site totals for the rows under
// the index. An error leaves the rows out.
func (s *nodes) loadExtras() tea.Cmd {
	c := s.d.API
	return tea.Batch(
		func() tea.Msg {
			p, _ := c.OnlineList(bg())
			return indexOnlineMsg{p}
		},
		func() tea.Msg {
			st, _ := c.SiteStats(bg())
			return indexStatsMsg{st}
		},
	)
}

// bbsUpdate handles the messages of the BBS index. It returns false for
// the messages that the modern code must also handle.
func (s *nodes) bbsUpdate(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case indexOnlineMsg:
		s.bbs.online = msg.page
		return nil, true
	case indexStatsMsg:
		s.bbs.stats = msg.stats
		return nil, true
	case tea.WindowSizeMsg:
		s.bbs.height = msg.Height
	}
	return nil, false
}

// input handles a key or a mouse wheel message. In the BBS interface the
// prompt chooses the forums, so that only the page keys work.
func (s *nodes) input(msg tea.Msg) tea.Cmd {
	k, isKey := msg.(tea.KeyPressMsg)
	switch {
	case s.d.BBS() && isKey:
		return s.bbsKey(k)
	case s.d.BBS():
		return nil
	case isKey:
		return s.key(k)
	}
	return s.move(msg)
}

func (s *nodes) bbsKey(k tea.KeyPressMsg) tea.Cmd {
	switch k.String() {
	case "pgup", "left":
		s.turnPage(-1)
	case "pgdown", "right":
		s.turnPage(1)
	}
	return nil
}

func (s *nodes) turnPage(dir int) {
	s.bbs.page = min(max(s.bbs.page+dir, 0), len(s.indexPages())-1)
}

// Prompt is the label of the BBS prompt.
func (s *nodes) Prompt() string { return i18n.T("bbs.prompt.command") }

// Commands are the BBS commands of the index.
func (s *nodes) Commands() []ui.Command {
	member := s.d.Me != nil
	cmds := []ui.Command{{Name: "#", Label: i18n.T("bbs.forum.cmd.enter"), Run: s.enter}}
	if member {
		cmds = append(cmds, ui.Command{Name: "N", Label: i18n.T("bbs.forum.cmd.unread"), Run: s.firstUnread, NoCount: true})
	}
	cmds = append(cmds,
		ui.Command{Name: "B", Label: i18n.T("bbs.global.search")},
		ui.Command{Name: "M", Label: i18n.T("bbs.global.messages")},
		ui.Command{Name: "U", Label: i18n.T("bbs.forum.cmd.users"), Run: func(string) tea.Cmd {
			return func() tea.Msg { return ui.OpenOnlineMsg{} }
		}},
		ui.Command{Name: "Q", Label: i18n.T("bbs.global.quit")},
		ui.Command{Name: "?", Label: i18n.T("bbs.global.help")},
	)
	if member {
		cmds = append(cmds, ui.Command{Name: "L", Label: i18n.T("bbs.forum.cmd.read"), Run: func(string) tea.Cmd { return s.markAll() }})
	}
	cmds = append(cmds, ui.Command{Name: "R", Label: i18n.T("bbs.forum.cmd.refresh"), Run: func(string) tea.Cmd {
		return tea.Batch(s.load(), s.loadExtras())
	}})
	if len(s.indexPages()) > 1 {
		cmds = append(cmds, pageCommands(func(dir int) tea.Cmd { s.turnPage(dir); return nil })...)
	}
	return cmds
}

// pageCommands returns the < and > commands with one hint for both.
func pageCommands(turn func(dir int) tea.Cmd) []ui.Command {
	return []ui.Command{
		{Name: "<", Label: i18n.T("bbs.forum.cmd.pages"), Run: func(string) tea.Cmd { return turn(-1) }},
		{Name: ">", Run: func(string) tea.Cmd { return turn(1) }},
	}
}

// enter opens the forum with the number arg.
func (s *nodes) enter(arg string) tea.Cmd {
	n, err := strconv.Atoi(arg)
	if err == nil && n > 0 {
		for i, num := range forumNumbers(s.items) {
			if num == n {
				id := s.items[i].Id
				return func() tea.Msg { return ui.OpenNodeMsg{NodeID: id} }
			}
		}
	}
	return ui.Toast(i18n.Tf("bbs.forum.noForum", map[string]any{"N": arg}), ui.ToastError)
}

// firstUnread opens the first forum with unread posts in the order of the
// index.
func (s *nodes) firstUnread(string) tea.Cmd {
	for _, n := range s.items {
		if n.Type != gen.NodeTypeCategory && s.unread[n.Id] {
			id := n.Id
			return func() tea.Msg { return ui.OpenNodeMsg{NodeID: id} }
		}
	}
	return toast("bbs.forum.noUnread")
}

// forumNumbers returns the number of each forum in the order of items,
// from 1. Categories get 0.
func forumNumbers(items []gen.Node) []int {
	out := make([]int, len(items))
	n := 0
	for i, it := range items {
		if it.Type != gen.NodeTypeCategory {
			n++
			out[i] = n
		}
	}
	return out
}

// forumIndents returns the indent level of each forum under its section.
// A forum right under a category has level 0.
func forumIndents(items []gen.Node) []int {
	out := make([]int, len(items))
	catDepth := -1
	for i, it := range items {
		if it.Type == gen.NodeTypeCategory {
			catDepth = it.Depth
			continue
		}
		out[i] = max(it.Depth-catDepth-1, 0)
	}
	return out
}

// indexFixed returns the number of lines of the index that are not tree
// rows: the frame, the title, the header and the rows under the tree.
func (s *nodes) indexFixed() int {
	n := 8
	if s.bbs.online != nil || s.bbs.stats != nil {
		n++
	}
	if s.bbs.online != nil {
		n++
	}
	if s.bbs.stats != nil {
		n++
	}
	return n
}

// indexPages splits the tree into pages that fit the height.
func (s *nodes) indexPages() [][]int {
	p := &indexPager{items: s.items, room: max(s.bbs.height-s.indexFixed(), 2), cat: -1}
	for _, b := range indexBlocks(s.items) {
		p.add(b)
	}
	p.flush()
	if len(p.pages) == 0 {
		return [][]int{nil}
	}
	return p.pages
}

// indexBlocks splits items into sections. Each section starts with its
// category, except the forums before the first category.
func indexBlocks(items []gen.Node) [][]int {
	var out [][]int
	for i, it := range items {
		if it.Type == gen.NodeTypeCategory || len(out) == 0 {
			out = append(out, nil)
		}
		out[len(out)-1] = append(out[len(out)-1], i)
	}
	return out
}

// indexPager puts the sections of the tree on pages. A section that fits
// on an empty page does not break. A longer section breaks between forums
// and its title shows again on the next page.
type indexPager struct {
	items []gen.Node
	room  int
	pages [][]int
	cur   []int
	used  int
	cat   int
}

func (p *indexPager) add(block []int) {
	cost := 0
	for k, i := range block {
		cost += p.lines(i, k > 0)
	}
	if p.used > 0 && p.used+1+cost > p.room && cost <= p.room {
		p.flush()
	}
	p.cat = -1
	for _, i := range block {
		p.put(i)
	}
}

func (p *indexPager) put(i int) {
	if p.items[i].Type == gen.NodeTypeCategory {
		p.cat = i
	}
	if h := p.lines(i, p.used > 0); p.used > 0 && p.used+h > p.room {
		p.flush()
		if p.cat >= 0 && p.cat != i {
			p.cur, p.used = []int{p.cat}, 1
		}
	}
	p.used += p.lines(i, p.used > 0)
	p.cur = append(p.cur, i)
}

// lines returns the lines of item i. A category after other rows also
// takes a separator line.
func (p *indexPager) lines(i int, after bool) int {
	if p.items[i].Type == gen.NodeTypeCategory {
		if after {
			return 2
		}
		return 1
	}
	if oneLine(p.items[i].Description) != "" {
		return 2
	}
	return 1
}

func (p *indexPager) flush() {
	if len(p.cur) > 0 {
		p.pages = append(p.pages, p.cur)
	}
	p.cur, p.used = nil, 0
}

// bbsView draws the index as an ASCII table.
func (s *nodes) bbsView(width, height int) string {
	pages := s.indexPages()
	page := min(s.bbs.page, len(pages)-1)
	t := ui.NewTable(width, s.indexColumns(width)...)
	t.Full(listSpread(i18n.T("bbs.forum.index.title"), s.indexRight(page, len(pages)), width-4)).Header()
	nums, indents := forumNumbers(s.items), forumIndents(s.items)
	total := slices.Max(nums)
	for k, i := range pages[page] {
		n := s.items[i]
		if n.Type == gen.NodeTypeCategory {
			if k > 0 {
				t.Rule()
			}
			t.Section(strings.ToUpper(n.Title))
			continue
		}
		pad := strings.Repeat("  ", indents[i])
		t.Row(ui.Num(nums[i], total), pad+s.indexMarker(n)+" "+n.Title,
			ui.Thousands(n.ThreadCount), ui.Thousands(n.PostCount), s.indexLast(n))
		if desc := oneLine(n.Description); desc != "" {
			t.Sub("", pad+desc)
		}
	}
	t.Full(i18n.T("bbs.forum.legend.index"))
	s.indexExtras(t, width-4)
	return t.Render(s.d.Styles)
}

func (s *nodes) indexColumns(width int) []ui.Column {
	tw, pw := len(i18n.T("bbs.forum.col.threads")), len(i18n.T("bbs.forum.col.posts"))
	for _, n := range s.items {
		tw = max(tw, len(ui.Thousands(n.ThreadCount)))
		pw = max(pw, len(ui.Thousands(n.PostCount)))
	}
	numW := len(ui.Num(1, slices.Max(forumNumbers(s.items))))
	return []ui.Column{
		{Title: i18n.T("bbs.forum.col.num"), Width: numW, Align: ui.AlignRight},
		{Title: i18n.T("bbs.forum.col.forum")},
		{Title: i18n.T("bbs.forum.col.threads"), Width: tw, Align: ui.AlignRight},
		{Title: i18n.T("bbs.forum.col.posts"), Width: pw, Align: ui.AlignRight},
		{Title: i18n.T("bbs.forum.col.last"), Width: min(max(width/5, 16), 24)},
	}
}

// indexRight is the right side of the title row: the page and, for a
// member, the count of forums with unread posts.
func (s *nodes) indexRight(page, pages int) string {
	var parts []string
	if pages > 1 {
		parts = append(parts, i18n.Tf("bbs.forum.page", map[string]any{"Page": page + 1, "Pages": pages}))
	}
	if s.d.Me != nil {
		n := 0
		for _, it := range s.items {
			if it.Type != gen.NodeTypeCategory && s.unread[it.Id] {
				n++
			}
		}
		parts = append(parts, i18n.Tf("bbs.forum.index.unread", map[string]any{"Count": ui.Thousands(n)}))
	}
	return strings.Join(parts, "   ")
}

// indexMarker is (*) for unread posts, (L) for a read-only forum and ( )
// for no news.
func (s *nodes) indexMarker(n gen.Node) string {
	switch {
	case s.unread[n.Id]:
		return "(*)"
	case n.Settings.IsReadOnly:
		return "(L)"
	}
	return "( )"
}

// indexLast is the age and the author of the last post.
func (s *nodes) indexLast(n gen.Node) string {
	if n.LastPost == nil {
		return "--"
	}
	age := listAgeShort(s.d.Now(), n.LastPost.PostedAt)
	return age + strings.Repeat(" ", max(5-ansi.StringWidth(age), 1)) + authorName(n.LastPost.User)
}

// indexExtras adds the online row and the totals row when they loaded.
func (s *nodes) indexExtras(t *ui.Table, w int) {
	if s.bbs.online == nil && s.bbs.stats == nil {
		return
	}
	t.Rule()
	if s.bbs.online != nil {
		t.Full(onlineLine(s.bbs.online, w))
	}
	if st := s.bbs.stats; st != nil {
		t.Full(i18n.Tf("bbs.forum.stats", map[string]any{
			"Members": ui.Thousands(st.Members), "Threads": ui.Thousands(st.Threads), "Posts": ui.Thousands(st.Posts),
		}))
	}
}

// onlineLine lists the first members on line that fit in w cells and the
// count of the other visitors.
func onlineLine(p *api.OnlinePage, w int) string {
	var names []string
	for _, v := range p.Items {
		if v.Card != nil {
			names = append(names, v.Card.Username)
		}
	}
	best := i18n.Tf("bbs.forum.online.none", map[string]any{"Count": ui.Thousands(p.Total)})
	for k := 1; k <= len(names); k++ {
		line := i18n.Tf("bbs.forum.online", map[string]any{"Count": ui.Thousands(p.Total), "Names": strings.Join(names[:k], ", ")})
		if rest := p.Total - k; rest > 0 {
			line += i18n.Tf("bbs.forum.online.more", map[string]any{"Count": ui.Thousands(rest)})
		}
		if ansi.StringWidth(line) > w {
			break
		}
		best = line
	}
	return best
}

// listSpread puts left and right on one line of w cells, with spaces
// between them.
func listSpread(left, right string, w int) string {
	gap := w - ansi.StringWidth(left) - ansi.StringWidth(right)
	if right == "" || gap < 1 {
		return left
	}
	return left + strings.Repeat(" ", gap) + right
}

// listAgeShort returns the time from t to now in a short form, for example
// "3m", "2h" or "4d". A time older than 30 days gives the day.
func listAgeShort(now, t time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Hour:
		return i18n.Tf("bbs.forum.age.minutes", map[string]any{"N": max(int(d.Minutes()), 0)})
	case d < 24*time.Hour:
		return i18n.Tf("bbs.forum.age.hours", map[string]any{"N": int(d.Hours())})
	case d < 30*24*time.Hour:
		return i18n.Tf("bbs.forum.age.days", map[string]any{"N": int(d.Hours() / 24)})
	}
	return listDay(now, t)
}

// listDay returns the day of t, for example "12-Ene". A day of an other
// year gives the month and the year, for example "Ene-2025".
func listDay(now, t time.Time) string {
	t = t.In(now.Location())
	month := t.Month().String()[:3]
	if months := strings.Split(i18n.T("bbs.months"), ","); len(months) == 12 {
		month = months[t.Month()-1]
	}
	if t.Year() != now.Year() {
		return month + "-" + strconv.Itoa(t.Year())
	}
	return fmt.Sprintf("%02d-%s", t.Day(), month)
}
