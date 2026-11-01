package forum

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/api/gen"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/render"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// The BBS reader draws the thread header in readerHeaderLines lines above
// the posts. Each post body is bodyIndent cells narrower than the screen.
const (
	readerHeaderLines = 4
	bodyIndent        = 6
	maxSignature      = 3
)

// cardsMsg holds the author cards of the members on a page.
type cardsMsg struct{ cards map[int]*api.Card }

// Prompt is the label of the BBS prompt.
func (r *reader) Prompt() string { return i18n.T("bbs.reader.prompt") }

// Commands are the BBS commands of the reader. A number goes to the post
// at that position. C, E, +, *, S and A take the number of a post on the
// page. O takes the number of a link of the post in focus.
func (r *reader) Commands() []ui.Command {
	if r.err != nil {
		return []ui.Command{{Name: "R", Label: i18n.T("bbs.reader.retry"), Run: func(string) tea.Cmd { return r.open() }}}
	}
	if r.picker != nil {
		return []ui.Command{
			{Name: "#", Label: i18n.T("bbs.reader.pick"), Run: r.ready(r.choose)},
			{Name: "", Run: func(string) tea.Cmd { return nil }},
			{Name: "V", Label: i18n.T("bbs.reader.cancel"), Run: func(string) tea.Cmd { r.picker = nil; return nil }},
		}
	}
	cmds := []ui.Command{
		{Name: "", Label: r.rangeLabel(), Run: r.ready(r.next)},
		{Name: "P", Label: i18n.T("bbs.reader.prev"), Run: r.ready(func(string) tea.Cmd { return r.turn(-1) })},
		{Name: "#", Label: i18n.T("bbs.reader.goto"), Run: r.ready(r.goTo)},
		{Name: "R", Label: i18n.T("bbs.reader.reply"), Run: r.onPost((*reader).reply)},
		{Name: "V", Label: i18n.T("bbs.global.back")},
		{Name: "C", Label: i18n.T("bbs.reader.quote"), Run: r.onPost((*reader).quote)},
		{Name: "E", Label: i18n.T("bbs.reader.edit"), Run: r.onPost((*reader).edit)},
		{Name: "+", Label: i18n.T("bbs.reader.react"), Run: r.onPost((*reader).react)},
		{Name: "*", Label: i18n.T("bbs.reader.bookmark"), Run: r.onPost((*reader).bookmark)},
		{Name: "W", Label: i18n.T("bbs.reader.watch"), Run: r.ready(func(string) tea.Cmd { return r.watch() })},
		{Name: "O", Label: i18n.T("bbs.reader.link"), Run: r.ready(r.openLink)},
		{Name: "S", Label: i18n.T("bbs.reader.spoilers"), Run: r.onPost((*reader).showSpoilers)},
		{Name: "U", Label: i18n.T("bbs.reader.unread"), Run: r.ready(func(string) tea.Cmd { return r.firstUnread() })},
		{Name: "A", Label: i18n.T("bbs.reader.author"), Run: r.onPost((*reader).profile)},
	}
	if r.view != nil && r.view.Poll.Exists {
		cmds = append(cmds, ui.Command{Name: "T", Label: i18n.T("bbs.reader.poll"), Run: r.ready(func(string) tea.Cmd { return r.poll() })})
	}
	return cmds
}

// ready wraps fn so that it does nothing while the thread loads.
func (r *reader) ready(fn func(arg string) tea.Cmd) func(string) tea.Cmd {
	return func(arg string) tea.Cmd {
		if r.view == nil || r.loading {
			return nil
		}
		return fn(arg)
	}
}

// onPost wraps an action on the post in focus. A number argument first
// puts the focus on the post at that position of the page.
func (r *reader) onPost(fn func(*reader) tea.Cmd) func(string) tea.Cmd {
	return r.ready(func(arg string) tea.Cmd {
		if arg != "" {
			i, cmd := r.postIndex(arg)
			if i < 0 {
				return cmd
			}
			r.jumpTo(i)
		}
		return fn(r)
	})
}

// postIndex returns the index on the page of the post with the number
// arg, or -1 and a toast.
func (r *reader) postIndex(arg string) (int, tea.Cmd) {
	n, err := strconv.Atoi(arg)
	if err != nil || n < 1 || n > r.view.Thread.ReplyCount+1 {
		return -1, ui.Toast(i18n.Tf("bbs.reader.noPost", map[string]any{"N": arg}), ui.ToastInfo)
	}
	for i, p := range r.posts {
		if p.Position == n-1 {
			return i, nil
		}
	}
	return -1, ui.Toast(i18n.Tf("bbs.reader.notOnPage", map[string]any{"N": n}), ui.ToastInfo)
}

// next scrolls one screen down. At the end of the page it loads the next
// page.
func (r *reader) next(string) tea.Cmd {
	if r.vp.AtBottom() {
		return r.turn(1)
	}
	r.scroll(tea.KeyPressMsg{Code: tea.KeyPgDown})
	return nil
}

// goTo shows the post with the number arg. It loads the page of the post
// when the post is on another page.
func (r *reader) goTo(arg string) tea.Cmd {
	n, err := strconv.Atoi(arg)
	if err != nil || n < 1 || n > r.view.Thread.ReplyCount+1 {
		return ui.Toast(i18n.Tf("bbs.reader.noPost", map[string]any{"N": arg}), ui.ToastInfo)
	}
	if i, _ := r.postIndex(arg); i >= 0 {
		r.jumpTo(i)
		return nil
	}
	r.loading = true
	d, id, pos := r.d, r.threadID, n-1
	return func() tea.Msg {
		data, err := fetchPage(d, id, pageOf(pos), 0)
		for _, p := range data.posts {
			if p.Position == pos {
				data.focusID = p.Id
			}
		}
		return readerMsg{page: data, err: err}
	}
}

// openLink opens the link with the number arg of the post in focus.
// Without a number it shows the links to choose from.
func (r *reader) openLink(arg string) tea.Cmd {
	if arg == "" {
		return r.links()
	}
	n, err := strconv.Atoi(arg)
	if r.focusedPost() == nil || err != nil || n < 1 || n > len(r.bodies[r.focus].links) {
		return toast("bbs.reader.noLink")
	}
	return ui.Open(r.bodies[r.focus].links[n-1].URL)
}

// choose takes the item with the number arg of the open choice.
func (r *reader) choose(arg string) tea.Cmd {
	n, err := strconv.Atoi(arg)
	if err != nil || n < 1 || n > len(r.picker.items) {
		return toast("bbs.reader.noChoice")
	}
	r.picker = nil
	return r.chosen(n - 1)
}

func (r *reader) rangeLabel() string {
	from, to, total := 0, 0, 0
	if r.view != nil {
		total = r.view.Thread.ReplyCount + 1
	}
	if len(r.posts) > 0 {
		from, to = r.posts[0].Position+1, r.posts[len(r.posts)-1].Position+1
	}
	return i18n.Tf("bbs.reader.next", map[string]any{"From": from, "To": to, "Total": ui.Thousands(total)})
}

// loadCards gets the cards of the member authors of the page that the
// reader does not have yet. Only the BBS interface shows the cards.
func (r *reader) loadCards() tea.Cmd {
	if !r.d.BBS() {
		return nil
	}
	seen := map[int]bool{}
	var ids []int
	for _, p := range r.posts {
		id := api.AuthorID(p.Author)
		if id != 0 && !seen[id] && r.cards[id] == nil {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	c := r.d.API
	return func() tea.Msg {
		out := make(map[int]*api.Card, len(ids))
		var mu sync.Mutex
		var wg sync.WaitGroup
		for _, id := range ids {
			wg.Go(func() {
				if card, err := c.AuthorCard(bg(), id); err == nil && card != nil {
					mu.Lock()
					out[id] = card
					mu.Unlock()
				}
			})
		}
		wg.Wait()
		return cardsMsg{cards: out}
	}
}

func (r *reader) gotCards(msg cardsMsg) {
	if r.cards == nil {
		r.cards = map[int]*api.Card{}
	}
	for id, c := range msg.cards {
		r.cards[id] = c
	}
	if len(r.posts) > 0 {
		r.keepOffset(r.assemble)
	}
}

// renderPost renders a body for the reader. The BBS interface gets plain
// ASCII text at the width inside the post box.
func (r *reader) renderPost(html string, atts []render.Attachment) (string, []render.Link, error) {
	if r.d.BBS() {
		return r.d.Render.PostBBS(html, atts, max(r.w-bodyIndent, 10))
	}
	return r.d.Render.Post(html, atts, ui.ReadingWidth(max(r.w, 20)), r.d.Glamour)
}

// headerLines is the number of lines above the viewport.
func (r *reader) headerLines() int {
	if r.d.BBS() {
		return readerHeaderLines
	}
	return 3
}

func (r *reader) bbsView(width, height int) string {
	switch {
	case r.loading && r.view == nil:
		return ui.Loading(r.d.Styles, r.sp, "", width, height)
	case r.err != nil:
		return errorView(r.d, r.err, width, height)
	case r.picker != nil:
		return r.choiceView(width, height)
	}
	body := r.markBody(r.vp.View())
	if len(r.posts) == 0 {
		body = ui.Empty(r.d.Styles, i18n.T("forum.thread.empty"), width, max(height-readerHeaderLines, 1))
	}
	return r.bbsHeader(width) + "\n" + body
}

// bbsHeader draws the forum and the thread number, the title and the
// totals, and a line of = under them.
func (r *reader) bbsHeader(width int) string {
	s, th := r.d.Styles, r.view.Thread
	crumb := i18n.Tf("bbs.reader.crumb", map[string]any{"Forum": r.view.Node.Title, "ID": th.Id})
	title := th.Title
	if th.Prefix != nil {
		title = "[" + th.Prefix.Label + "] " + title
	}
	return strings.Join([]string{
		framedRow(s, width, crumb),
		framedRow(s, width, s.Subtitle.Render(title)),
		framedRow(s, width, s.Muted.Render(strings.Join(r.threadFacts(), " . "))),
		ui.Rule(s, width, '='),
	}, "\n")
}

// threadFacts are the totals, the tags and the states of the thread.
func (r *reader) threadFacts() []string {
	th := r.view.Thread
	facts := []string{
		i18n.Plural("bbs.reader.replies", th.ReplyCount, map[string]any{"N": ui.Thousands(th.ReplyCount)}),
		i18n.Plural("bbs.reader.views", th.ViewCount, map[string]any{"N": ui.Thousands(th.ViewCount)}),
	}
	if len(r.view.Tags) > 0 {
		facts = append(facts, i18n.Tf("bbs.reader.tags", map[string]any{"Tags": strings.Join(r.view.Tags, ", ")}))
	}
	states := []struct {
		on bool
		id string
	}{
		{th.IsLocked, "bbs.reader.locked"}, {th.IsSticky, "bbs.reader.sticky"},
		{r.watching, "bbs.reader.watching"}, {r.view.Poll.Exists, "bbs.reader.hasPoll"},
	}
	for _, st := range states {
		if st.on {
			facts = append(facts, i18n.T(st.id))
		}
	}
	return facts
}

// block draws post i for the viewport.
func (r *reader) block(i int) string {
	if r.d.BBS() {
		return r.bbsPostBlock(i)
	}
	return r.postBlock(i)
}

// bbsPostBlock draws post i: the title row, the card row, the body with
// the signature in a box, the footer row and a line of = under them.
func (r *reader) bbsPostBlock(i int) string {
	s, w, p := r.d.Styles, r.w, r.posts[i]
	lines := []string{r.postTitleRow(i)}
	if row := r.cardRow(p); row != "" {
		lines = append(lines, row)
	}
	lines = append(lines, ui.Rule(s, w, '-'), framedRow(s, w, ""))
	for _, l := range strings.Split(r.bodies[i].text, "\n") {
		lines = append(lines, framedRow(s, w, " "+l))
	}
	lines = append(lines, framedRow(s, w, ""))
	lines = append(lines, r.signature(p)...)
	lines = append(lines, ui.Rule(s, w, '-'), r.postFooter(i), ui.Rule(s, w, '='))
	return strings.Join(lines, "\n")
}

// postTitleRow shows the number, the author and the group badge on the
// left and the date on the right. The number of the post in focus gets
// the Key style.
func (r *reader) postTitleRow(i int) string {
	s, p := r.d.Styles, r.posts[i]
	num := s.Muted.Render(fmt.Sprintf("%-4s", "#"+strconv.Itoa(p.Position+1)))
	if i == r.focus {
		num = s.Key.Render(fmt.Sprintf("%-4s", "#"+strconv.Itoa(p.Position+1)))
	}
	left := num + " " + s.Accent.Render(authorName(p.Author))
	if c := r.cards[api.AuthorID(p.Author)]; c != nil && c.CustomTitle != "" && c.GroupTitle != "" {
		left += "  " + s.Subtitle.Render("<<"+strings.ToUpper(c.GroupTitle)+">>")
	}
	return framedRow(s, r.w, sides(left, s.Text.Render(postStamp(p.CreatedAt)), r.w-4))
}

// cardRow shows the title, the post count, the join date and the
// reaction score of the author. It is "" while the card is not loaded and
// for a guest author.
func (r *reader) cardRow(p gen.Post) string {
	c := r.cards[api.AuthorID(p.Author)]
	if c == nil {
		return ""
	}
	var parts []string
	if title := c.CustomTitle; title != "" {
		parts = append(parts, title)
	} else if c.GroupTitle != "" {
		parts = append(parts, c.GroupTitle)
	}
	score := ui.Thousands(c.ReactionScore)
	if c.ReactionScore >= 0 {
		score = "+" + score
	}
	parts = append(parts,
		i18n.Plural("bbs.reader.posts", c.PostCount, map[string]any{"N": ui.Thousands(c.PostCount)}),
		i18n.Tf("bbs.reader.since", map[string]any{"Date": monthYear(c.JoinedAt)}),
		i18n.Tf("bbs.reader.rep", map[string]any{"Score": score}))
	return framedRow(r.d.Styles, r.w, r.d.Styles.Muted.Render("     "+strings.Join(parts, " . ")))
}

// signature returns the rows of the signature of the author: a "--" row
// and at most maxSignature rows of text.
func (r *reader) signature(p gen.Post) []string {
	id := api.AuthorID(p.Author)
	c := r.cards[id]
	if c == nil || c.SignatureHTML == "" {
		return nil
	}
	if r.sigs == nil {
		r.sigs = map[int][]string{}
	}
	text, ok := r.sigs[id]
	if !ok {
		out, _, err := r.d.Render.PostBBS(c.SignatureHTML, nil, max(r.w-bodyIndent, 10))
		if err == nil {
			text = strings.Split(strings.Trim(out, "\n"), "\n")
		}
		text = text[:min(len(text), maxSignature)]
		r.sigs[id] = text
	}
	if len(text) == 0 {
		return nil
	}
	s := r.d.Styles
	rows := []string{framedRow(s, r.w, s.Muted.Render(" --"))}
	for _, l := range text {
		rows = append(rows, framedRow(s, r.w, s.Muted.Render(" "+l)))
	}
	return rows
}

// postFooter shows the reactions and the marks on the left and the
// actions on the right.
func (r *reader) postFooter(i int) string {
	s, p := r.d.Styles, r.posts[i]
	left := r.reactionCounts(p.Reactions)
	if left != "" {
		left = s.Text.Render(left)
	}
	var marks []string
	if p.EditedAt != nil {
		marks = append(marks, i18n.T("bbs.reader.edited"))
	}
	if r.bookmarked[p.Id] {
		marks = append(marks, i18n.T("bbs.reader.bookmarked"))
	}
	if len(marks) > 0 {
		if left != "" {
			left += "   "
		}
		left += s.Accent.Render(strings.Join(marks, "  "))
	}
	var actions []string
	if r.view.Permissions.CanReply {
		actions = append(actions, i18n.T("bbs.reader.quote"), i18n.T("bbs.reader.reply"))
	}
	if p.CanEdit {
		actions = append(actions, i18n.T("bbs.reader.edit"))
	}
	return framedRow(s, r.w, sides(left, paintKeys(s, strings.Join(actions, "  ")), r.w-4))
}

// reactionCounts shows the count of each reaction type like "[Like 12]".
// A star marks the reaction of the viewer.
func (r *reader) reactionCounts(sum gen.ReactionSummary) string {
	ids := make([]int, 0, len(sum.Counts))
	for k := range sum.Counts {
		if id, err := strconv.Atoi(k); err == nil {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(a, b int) bool { return r.typeIndex(ids[a]) < r.typeIndex(ids[b]) })
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		text := r.reactionTitle(id) + " " + ui.Thousands(sum.Counts[strconv.Itoa(id)])
		if sum.Mine != nil && *sum.Mine == id {
			text += "*"
		}
		parts = append(parts, "["+text+"]")
	}
	return strings.Join(parts, " ")
}

func (r *reader) reactionTitle(id int) string {
	for _, t := range r.rtypes {
		if t.Id == id {
			return t.Title
		}
	}
	return "#" + strconv.Itoa(id)
}

// choiceView shows the open choice of the reader as a numbered table.
func (r *reader) choiceView(width, height int) string {
	s := r.d.Styles
	labels, title := r.choiceLabels()
	t := ui.NewTable(width, ui.Column{Title: i18n.T("bbs.help.number"), Width: 2, Align: ui.AlignRight},
		ui.Column{Title: i18n.T("bbs.reader.choice")})
	t.Full(s.Subtitle.Render(title)).Header()
	for i, l := range labels[:min(len(labels), max(height-6, 1))] {
		t.Row(ui.Num(i+1, len(labels)), l).Zone(r.d, r.picker.list.RowZone(i))
		if i == r.picker.list.Cursor() {
			t.Select()
		}
	}
	return t.Render(s)
}

// choiceLabels returns the ASCII labels and the title of the open choice.
func (r *reader) choiceLabels() ([]string, string) {
	if r.pickKind == pickLink {
		links := r.bodies[r.focus].links
		out := make([]string, len(links))
		for i, l := range links {
			out[i] = l.Label + "  " + r.d.Styles.Muted.Render(l.URL)
		}
		return out, i18n.T("bbs.reader.pickLink")
	}
	out := make([]string, len(r.pickIDs))
	for i, id := range r.pickIDs {
		out[i] = r.reactionTitle(id)
		for _, t := range r.rtypes {
			if t.Id == id && t.Access == gen.Locked {
				out[i] = r.d.Styles.Muted.Render(out[i])
			}
		}
	}
	return out, i18n.T("bbs.reader.pickReaction")
}

// framedRow puts text between two side lines in a row of width cells. A
// text that is too long ends in "...". A plain text gets the Text style.
func framedRow(s ui.Styles, width int, text string) string {
	inner := max(width-4, 0)
	if ansi.StringWidth(text) > inner {
		text = ansi.Truncate(text, inner, "...")
	}
	pad := strings.Repeat(" ", inner-ansi.StringWidth(text))
	if text != "" && !strings.Contains(text, "\x1b") {
		text = s.Text.Render(text)
	}
	bar := s.Muted.Render("|")
	return bar + " " + text + pad + " " + bar
}

// sides puts left and right on one line of w cells. It cuts left when the
// line is too narrow.
func sides(left, right string, w int) string {
	rw := ansi.StringWidth(right)
	if rw+2 > w {
		return ansi.Truncate(left, w, "...")
	}
	if ansi.StringWidth(left) > w-rw-1 {
		left = ansi.Truncate(left, w-rw-1, "...")
	}
	return left + strings.Repeat(" ", w-rw-ansi.StringWidth(left)) + right
}

// paintKeys gives the text in brackets the Key style, the brackets the
// Muted style and the rest the Text style.
func paintKeys(s ui.Styles, label string) string {
	var b strings.Builder
	for label != "" {
		open, end := strings.Index(label, "["), strings.Index(label, "]")
		if open < 0 || end < open {
			b.WriteString(s.Text.Render(label))
			break
		}
		if open > 0 {
			b.WriteString(s.Text.Render(label[:open]))
		}
		b.WriteString(s.Muted.Render("[") + s.Key.Render(label[open+1:end]) + s.Muted.Render("]"))
		label = label[end+1:]
	}
	return b.String()
}

// postStamp formats t like "31-Oct-2026  14:02" in the local time zone
// with the month names of the current language.
func postStamp(t time.Time) string {
	t = t.Local()
	return fmt.Sprintf("%02d-%s-%d  %s", t.Day(), monthName(t.Month()), t.Year(), t.Format("15:04"))
}

// monthYear formats t like "Mar-2026".
func monthYear(t time.Time) string {
	t = t.Local()
	return monthName(t.Month()) + "-" + strconv.Itoa(t.Year())
}

func monthName(m time.Month) string {
	if months := strings.Split(i18n.T("bbs.months"), ","); len(months) == 12 {
		return months[m-1]
	}
	return m.String()[:3]
}
