package forum

import (
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/bbcode"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// The limits of the line editor: the number of lines and the columns of
// a typed line.
const (
	maxLines = 200
	lineCols = 72
)

// The panels that the line editor can show in place of the lines.
const (
	panelNone = iota
	panelHelp
	panelPrefixes
)

// lineEditor holds the text of the BBS composer as lines. The member
// types one line and Enter adds it. A line that starts with / is a
// command.
type lineEditor struct {
	lines []string
	// cur is the line that the member types.
	cur string
	// at is the index for the next line. edit is the index of the line
	// in cur, or -1.
	at, edit int
	// top is the first line to show, or -1 to show the line in cur.
	top   int
	panel int
}

// editorQuoteMsg has the BB code of a quote, or the number of a post that
// the thread does not have.
type editorQuoteMsg struct {
	text    string
	missing int
	err     error
}

// slashCommand is one command of the line editor. args shows the
// arguments in the help panel and help is the i18n ID of the help text.
type slashCommand struct {
	name, args, help string
	run              func(c *composer, arg string) tea.Cmd
}

// slashCommands are the commands of the line editor in the order of the
// help panel.
var slashCommands = []slashCommand{
	{"S", "", "bbs.editor.cmd.send", func(c *composer, _ string) tea.Cmd { return c.send() }},
	{"A", "", "bbs.editor.cmd.abort", func(c *composer, _ string) tea.Cmd { return c.abort() }},
	{"C", "[n]", "bbs.editor.cmd.quote", (*composer).quoteLine},
	{"V", "", "bbs.editor.cmd.preview", func(c *composer, _ string) tea.Cmd { return c.togglePreview() }},
	{"E", "n", "bbs.editor.cmd.edit", (*composer).editLine},
	{"D", "n", "bbs.editor.cmd.delete", (*composer).deleteLine},
	{"I", "n", "bbs.editor.cmd.insert", (*composer).insertAt},
	{"L", "[n]", "bbs.editor.cmd.list", (*composer).listFrom},
	{"T", "", "bbs.editor.cmd.title", (*composer).setTitle},
	{"P", "[n]", "bbs.editor.cmd.prefix", (*composer).pickPrefix},
	{"?", "", "bbs.editor.cmd.help", func(c *composer, _ string) tea.Cmd { c.ed.panel = panelHelp; return nil }},
}

func newLineEditor(body string) *lineEditor {
	e := &lineEditor{edit: -1, top: -1}
	if body = strings.TrimSuffix(body, "\n"); body != "" {
		e.lines = strings.Split(body, "\n")
	}
	e.at = len(e.lines)
	return e
}

// text returns the body: the lines of the line editor joined by line
// breaks, or the text of the text area.
func (c *composer) text() string {
	if c.ed != nil {
		return strings.Join(c.ed.lines, "\n")
	}
	return c.body.Value()
}

// pending reports whether the line in cur is a change: a typed line, or
// a line under edit that is not the same as its stored text.
func (e *lineEditor) pending() bool {
	if e == nil {
		return false
	}
	if e.edit >= 0 {
		return e.cur != e.lines[e.edit]
	}
	return e.cur != ""
}

// previewText renders the HTML of a preview for the composer.
func previewText(d *ui.Deps, html string, w int, bbs bool) (string, error) {
	if bbs {
		text, _, err := d.Render.PostBBS(html, nil, w)
		return text, err
	}
	text, _, err := d.Render.Post(html, nil, w, d.Glamour)
	return text, err
}

// Prompt is the label of the BBS prompt.
func (c *composer) Prompt() string { return i18n.T("bbs.editor.prompt") }

// Commands give the hints of the line editor. The editor takes all keys,
// so the prompt runs none of them.
func (c *composer) Commands() []ui.Command {
	return []ui.Command{
		{Name: "", Label: i18n.T("bbs.editor.hintEnter")},
		{Name: "/S", Label: i18n.T("bbs.editor.hintSend")},
		{Name: "/A", Label: i18n.T("bbs.editor.hintAbort")},
		{Name: "/?", Label: i18n.T("bbs.editor.hintHelp")},
	}
}

func (c *composer) edKey(k tea.KeyPressMsg) tea.Cmd {
	if c.preview {
		switch k.String() {
		case "up", "down", "pgup", "pgdown":
			c.pv, _ = c.pv.Update(k)
		default:
			c.preview = false
		}
		return nil
	}
	e := c.ed
	e.panel = panelNone
	switch k.String() {
	case "enter":
		return c.submit()
	case "esc":
		return c.abort()
	case "backspace":
		_, size := lastRune(e.cur)
		e.cur = e.cur[:len(e.cur)-size]
		return nil
	}
	if k.Text != "" && k.Mod&^tea.ModShift == 0 {
		return c.typeRunes(k.Text)
	}
	return nil
}

func lastRune(s string) (rune, int) {
	if s == "" {
		return 0, 0
	}
	r := []rune(s)
	return r[len(r)-1], len(string(r[len(r)-1]))
}

// edMsg handles a paste and the result of a quote.
func (c *composer) edMsg(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.PasteMsg:
		return c.paste(msg.Content)
	case editorQuoteMsg:
		return c.quoted(msg)
	}
	return nil
}

// paste types the text. Each line break of the text adds a line.
func (c *composer) paste(text string) tea.Cmd {
	parts := strings.Split(strings.ReplaceAll(text, "\r", ""), "\n")
	for i, p := range parts {
		if cmd := c.typeRunes(p); cmd != nil {
			return cmd
		}
		if i < len(parts)-1 {
			if cmd := c.commit(c.ed.cur); cmd != nil {
				return cmd
			}
		}
	}
	return nil
}

// typeRunes adds text to the typed line. A line that gets wider than
// lineCols breaks at its last space, or at lineCols without a space.
func (c *composer) typeRunes(text string) tea.Cmd {
	e := c.ed
	for _, r := range text {
		e.cur += string(r)
		if ansi.StringWidth(e.cur) <= lineCols {
			continue
		}
		line, rest := breakLine(e.cur)
		if cmd := c.commit(line); cmd != nil {
			e.cur = line
			return cmd
		}
		e.cur = rest
	}
	return nil
}

// breakLine cuts s into a line of lineCols cells or less and the rest.
func breakLine(s string) (string, string) {
	if i := strings.LastIndex(s, " "); i > 0 {
		return s[:i], s[i+1:]
	}
	line := ansi.Truncate(s, lineCols, "")
	return line, s[len(line):]
}

// submit adds the typed line or runs it as a command. A line that starts
// with // gives a line that starts with /.
func (c *composer) submit() tea.Cmd {
	text := c.ed.cur
	switch {
	case strings.HasPrefix(text, "//"):
		return c.commit(text[1:])
	case strings.HasPrefix(text, "/"):
		c.ed.cur, c.ed.edit = "", -1
		return c.runSlash(text[1:])
	}
	return c.commit(text)
}

// commit puts line in place of the line in edit, or adds it at the
// insert point.
func (c *composer) commit(line string) tea.Cmd {
	e := c.ed
	if e.edit >= 0 {
		e.lines[e.edit] = line
		e.at, e.edit = e.edit+1, -1
	} else {
		if len(e.lines) >= maxLines {
			return toast("bbs.editor.full")
		}
		e.lines = slices.Insert(e.lines, e.at, line)
		e.at++
	}
	e.cur, e.top, c.confirm = "", -1, false
	return nil
}

func (c *composer) runSlash(text string) tea.Cmd {
	name, arg, _ := strings.Cut(strings.TrimSpace(text), " ")
	name, arg = strings.ToUpper(name), strings.TrimSpace(arg)
	if name != "A" {
		c.confirm = false
	}
	for _, sc := range slashCommands {
		if sc.name == name {
			return sc.run(c, arg)
		}
	}
	return toast("bbs.editor.unknown")
}

// abort closes the editor. With changes it asks first, and a second abort
// closes it.
func (c *composer) abort() tea.Cmd {
	if !c.changed() || c.confirm {
		return ui.Pop()
	}
	c.confirm = true
	return nil
}

// lineNumber reads the line number arg from 1 to last.
func lineNumber(arg string, last int) (int, bool) {
	n, err := strconv.Atoi(arg)
	return n - 1, err == nil && n >= 1 && n <= last
}

func (c *composer) editLine(arg string) tea.Cmd {
	i, ok := lineNumber(arg, len(c.ed.lines))
	if !ok {
		return toast("bbs.editor.noLine")
	}
	c.ed.edit, c.ed.cur = i, c.ed.lines[i]
	return nil
}

func (c *composer) deleteLine(arg string) tea.Cmd {
	e := c.ed
	i, ok := lineNumber(arg, len(e.lines))
	if !ok {
		return toast("bbs.editor.noLine")
	}
	e.lines = slices.Delete(e.lines, i, i+1)
	if i < e.at {
		e.at--
	}
	return nil
}

func (c *composer) insertAt(arg string) tea.Cmd {
	i, ok := lineNumber(arg, len(c.ed.lines)+1)
	if !ok {
		return toast("bbs.editor.noLine")
	}
	c.ed.at = i
	return nil
}

func (c *composer) listFrom(arg string) tea.Cmd {
	if arg == "" {
		c.ed.top = 0
		return nil
	}
	i, ok := lineNumber(arg, len(c.ed.lines))
	if !ok {
		return toast("bbs.editor.noLine")
	}
	c.ed.top = i
	return nil
}

func (c *composer) setTitle(arg string) tea.Cmd {
	if c.mode != modeThread {
		return toast("bbs.editor.threadOnly")
	}
	c.title.SetValue(arg)
	return nil
}

// pickPrefix shows the prefixes, or chooses the prefix with the number
// arg. The number 0 removes the prefix.
func (c *composer) pickPrefix(arg string) tea.Cmd {
	switch {
	case c.mode != modeThread:
		return toast("bbs.editor.threadOnly")
	case len(c.prefixes) == 0:
		return toast("bbs.editor.noPrefixes")
	case arg == "":
		c.ed.panel = panelPrefixes
		return nil
	}
	n, err := strconv.Atoi(arg)
	if err != nil || n < 0 || n > len(c.prefixes) {
		return toast("bbs.reader.noChoice")
	}
	c.prefix = n - 1
	return nil
}

// quoteLine quotes the post with the number arg, or the post in focus in
// the reader, at the insert point.
func (c *composer) quoteLine(arg string) tea.Cmd {
	if c.mode == modeThread || c.threadID == 0 {
		return toast("bbs.editor.noQuote")
	}
	id, pos := 0, -1
	if arg == "" {
		if c.back == nil || c.back.focusedPost() == nil {
			return toast("bbs.editor.noQuote")
		}
		id = c.back.focusedPost().Id
	} else {
		n, err := strconv.Atoi(arg)
		if err != nil || n < 1 {
			return toast("bbs.editor.noQuote")
		}
		pos, id = n-1, c.postAt(n-1)
	}
	a, threadID := c.d.API, c.threadID
	return func() tea.Msg { return fetchQuote(a, threadID, id, pos) }
}

// postAt returns the ID of the post at position on the page of the
// reader, or 0.
func (c *composer) postAt(position int) int {
	if c.back == nil {
		return 0
	}
	for _, p := range c.back.posts {
		if p.Position == position {
			return p.Id
		}
	}
	return 0
}

// fetchQuote gets the post id, or the post at position when id is 0, and
// makes its quote.
func fetchQuote(a *api.Client, threadID, id, position int) editorQuoteMsg {
	if id == 0 {
		list, err := a.Posts(bg(), threadID, api.PageOpts{Page: pageOf(position)})
		if err != nil {
			return editorQuoteMsg{err: err}
		}
		for _, p := range list.Items {
			if p.Position == position {
				id = p.Id
			}
		}
		if id == 0 {
			return editorQuoteMsg{missing: position + 1}
		}
	}
	p, err := a.Post(bg(), id)
	if err != nil {
		return editorQuoteMsg{err: err}
	}
	return editorQuoteMsg{text: bbcode.Quote(api.AuthorName(p.Author), p.Id, api.AuthorID(p.Author), p.BodySource)}
}

// quoted adds the lines of the quote at the insert point.
func (c *composer) quoted(msg editorQuoteMsg) tea.Cmd {
	switch {
	case msg.err != nil:
		return ui.ErrorCmd(msg.err)
	case msg.missing > 0:
		return ui.Toast(i18n.Tf("bbs.reader.noPost", map[string]any{"N": msg.missing}), ui.ToastInfo)
	}
	for _, l := range strings.Split(msg.text, "\n") {
		if cmd := c.commit(l); cmd != nil {
			return cmd
		}
	}
	return nil
}
