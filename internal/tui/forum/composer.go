package forum

import (
	"errors"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/api/gen"
	"github.com/albertgmz/sermo-cli/internal/bbcode"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// The limits of the server for a title and a body.
const (
	maxTitle = 150
	maxBody  = 10000
)

type composeMode int

const (
	modeUnsupported composeMode = iota
	modeThread
	modeReply
	modeEdit
)

// The fields of the composer in focus order.
const (
	fieldTitle = iota
	fieldPrefix
	fieldBody
)

// formatKeys maps the format hotkeys to their BB code tags. Many terminals
// send Ctrl+I as Tab, so Alt+I also gives italic.
var formatKeys = map[string]string{
	"ctrl+b": "B", "ctrl+i": "I", "alt+i": "I", "ctrl+u": "U", "ctrl+k": "URL", "ctrl+q": "QUOTE",
}

// composer writes a new thread, a reply or an edit in BB code.
type composer struct {
	d                        *ui.Deps
	mode                     composeMode
	nodeID, threadID, postID int
	// back is the reader that opened the composer, or nil.
	back *reader

	title         textinput.Model
	body          textarea.Model
	initial       string
	prefixes      []gen.ThreadPrefix
	prefix        int // index in prefixes, -1 for none
	requirePrefix bool
	field         int

	preview    bool
	previewing bool
	pv         viewport.Model
	sending    bool
	errText    string
	issues     []api.Issue
	confirm    bool
	w, h       int

	// ed is the line editor of the BBS interface, or nil.
	ed        *lineEditor
	nodeTitle string
}

type composeMetaMsg struct {
	prefixes []gen.ThreadPrefix
	require  bool
	node     string
}

type previewMsg struct {
	text string
	err  error
}

// sentMsg is the result of a send. post is the new or changed post.
type sentMsg struct {
	post   *gen.PostDetail
	thread *api.NewThread
	err    error
}

// NewComposer returns the composer for the target in m. It writes a new
// thread for m.NodeID or a reply for m.ThreadID with m.Body as start text.
func NewComposer(d *ui.Deps, m ui.ComposeMsg) ui.Screen {
	switch {
	case m.NodeID != 0:
		return newThreadComposer(d, m.NodeID)
	case m.ThreadID != 0:
		return newReplyComposer(d, m.ThreadID, m.Body, nil)
	}
	return newComposer(d, modeUnsupported, "")
}

func newThreadComposer(d *ui.Deps, nodeID int) *composer {
	c := newComposer(d, modeThread, "")
	c.nodeID = nodeID
	c.focus(fieldTitle)
	return c
}

func newReplyComposer(d *ui.Deps, threadID int, body string, back *reader) *composer {
	c := newComposer(d, modeReply, body)
	c.threadID, c.back = threadID, back
	return c
}

func newEditComposer(d *ui.Deps, threadID, postID int, body string, back *reader) *composer {
	c := newComposer(d, modeEdit, body)
	c.threadID, c.postID, c.back = threadID, postID, back
	return c
}

func newComposer(d *ui.Deps, mode composeMode, body string) *composer {
	ti := textinput.New()
	ti.CharLimit = maxTitle
	ti.Prompt = ""
	ti.Placeholder = i18n.T("forum.compose.titlePlaceholder")
	ta := textarea.New()
	ta.CharLimit = maxBody
	ta.MaxHeight = 0
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.Placeholder = i18n.T("forum.compose.bodyPlaceholder")
	ta.SetValue(body)
	setStyles(d, &ti, &ta)
	c := &composer{d: d, mode: mode, title: ti, body: ta, initial: body, prefix: -1, pv: viewport.New()}
	if d.BBS() {
		c.ed = newLineEditor(body)
		c.initial = c.text()
	}
	c.focus(fieldBody)
	return c
}

// setStyles gives the fields plain text in the theme colors and a cursor
// that does not blink. A writing view does not animate.
func setStyles(d *ui.Deps, ti *textinput.Model, ta *textarea.Model) {
	ts := ti.Styles()
	ts.Cursor.Blink, ts.Cursor.Color = false, d.Theme.Accent
	ts.Focused.Text, ts.Blurred.Text = d.Styles.Text, d.Styles.Text
	ts.Focused.Placeholder, ts.Blurred.Placeholder = d.Styles.Muted, d.Styles.Muted
	ti.SetStyles(ts)
	as := ta.Styles()
	as.Cursor.Blink, as.Cursor.Color = false, d.Theme.Accent
	for _, st := range []*textarea.StyleState{&as.Focused, &as.Blurred} {
		st.Text, st.Placeholder = d.Styles.Text, d.Styles.Muted
		st.CursorLine, st.EndOfBuffer = d.Styles.Text, d.Styles.Muted
	}
	ta.SetStyles(as)
}

func (c *composer) Init() tea.Cmd {
	if c.mode != modeThread {
		return nil
	}
	api, id := c.d.API, c.nodeID
	return func() tea.Msg {
		prefixes, _ := api.ThreadPrefixes(bg(), id)
		v, err := api.Node(bg(), id)
		if err != nil {
			return composeMetaMsg{prefixes: prefixes}
		}
		return composeMetaMsg{prefixes: prefixes, require: v.Node.Settings.RequirePrefix, node: v.Node.Title}
	}
}

func (c *composer) Title() string {
	ids := map[composeMode]string{
		modeThread: "forum.compose.newThread", modeReply: "forum.compose.reply",
		modeEdit: "forum.compose.edit", modeUnsupported: "forum.compose.reply",
	}
	return i18n.T(ids[c.mode])
}

func (c *composer) CapturesInput() bool { return c.mode != modeUnsupported }

func (c *composer) Keys() []key.Binding {
	b := func(keys []string, help, id string) key.Binding {
		return key.NewBinding(key.WithKeys(keys...), key.WithHelp(help, i18n.T(id)))
	}
	return []key.Binding{
		b([]string{"ctrl+s"}, "Ctrl+S", "forum.key.send"),
		b([]string{"ctrl+p"}, "Ctrl+P", "forum.key.preview"),
		b([]string{"tab"}, "Tab", "forum.key.field"),
		b([]string{"ctrl+b", "ctrl+i", "alt+i", "ctrl+u", "ctrl+k", "ctrl+q"}, "Ctrl+B I U K Q", "forum.key.format"),
		b([]string{"esc"}, "Esc", "forum.key.discard"),
	}
}

// changed reports whether the text differs from the start text.
func (c *composer) changed() bool {
	return c.text() != c.initial || c.title.Value() != "" || c.ed.pending()
}

func (c *composer) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.resize(msg.Width, msg.Height)
	case composeMetaMsg:
		c.prefixes, c.requirePrefix, c.nodeTitle = msg.prefixes, msg.require, msg.node
	case previewMsg:
		return c, c.previewed(msg)
	case sentMsg:
		return c, c.sent(msg)
	case tea.KeyPressMsg:
		return c, c.key(msg)
	default:
		if c.ed != nil {
			return c, c.edMsg(msg)
		}
		return c, c.forward(msg)
	}
	return c, nil
}

func (c *composer) resize(w, h int) {
	c.w, c.h = w, h
	rw := ui.ReadingWidth(w)
	c.title.SetWidth(max(rw-12, 10))
	c.body.SetWidth(rw)
	c.body.SetHeight(max(h-c.chromeLines(), 3))
	c.pv.SetWidth(rw)
	c.pv.SetHeight(max(h-3, 1))
}

// chromeLines is the number of lines around the body field.
func (c *composer) chromeLines() int {
	n := 5
	if c.mode == modeThread {
		n += 2
	}
	return n
}

func (c *composer) key(k tea.KeyPressMsg) tea.Cmd {
	if c.ed != nil {
		return c.edKey(k)
	}
	s := k.String()
	if c.confirm {
		c.confirm = false
		if s == "esc" {
			return ui.Pop()
		}
		return nil
	}
	switch s {
	case "esc":
		if !c.changed() {
			return ui.Pop()
		}
		c.confirm = true
		return nil
	case "ctrl+s":
		return c.send()
	case "ctrl+p":
		return c.togglePreview()
	}
	if c.preview {
		c.pv, _ = c.pv.Update(k)
		return nil
	}
	return c.edit(k)
}

func (c *composer) edit(k tea.KeyPressMsg) tea.Cmd {
	s := k.String()
	switch {
	case s == "tab" || s == "shift+tab":
		c.nextField(s == "tab")
		return nil
	case c.field == fieldPrefix:
		c.cyclePrefix(s)
		return nil
	case c.field == fieldBody && formatKeys[s] != "":
		c.wrap(formatKeys[s])
		return nil
	}
	return c.forward(k)
}

// forward sends msg to the field in focus.
func (c *composer) forward(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch c.field {
	case fieldTitle:
		c.title, cmd = c.title.Update(msg)
	case fieldBody:
		c.body, cmd = c.body.Update(msg)
	}
	return cmd
}

func (c *composer) fields() []int {
	if c.mode != modeThread {
		return []int{fieldBody}
	}
	if len(c.prefixes) == 0 {
		return []int{fieldTitle, fieldBody}
	}
	return []int{fieldTitle, fieldPrefix, fieldBody}
}

func (c *composer) nextField(forward bool) {
	fs := c.fields()
	i := 0
	for j, f := range fs {
		if f == c.field {
			i = j
		}
	}
	step := 1
	if !forward {
		step = len(fs) - 1
	}
	c.focus(fs[(i+step)%len(fs)])
}

func (c *composer) focus(f int) {
	c.field = f
	c.title.Blur()
	c.body.Blur()
	switch f {
	case fieldTitle:
		c.title.Focus()
	case fieldBody:
		c.body.Focus()
	}
}

func (c *composer) cyclePrefix(s string) {
	n := len(c.prefixes) + 1
	switch s {
	case "right", "l", "space":
		c.prefix = (c.prefix+2)%n - 1
	case "left", "h":
		c.prefix = (c.prefix+n)%n - 1
	}
}

// wrap puts the tag around the selection, or puts the pair of tags at the
// cursor.
func (c *composer) wrap(tag string) {
	lines := strings.Split(c.body.Value(), "\n")
	start := offset(lines, c.body.Line(), c.body.Column())
	end := start
	if a, b, ok := c.body.Selection(); ok {
		start, end = offset(lines, a.Row, a.Col), offset(lines, b.Row, b.Col)
	}
	text, cur := bbcode.Wrap(c.body.Value(), tag, start, end)
	c.body.SetValue(text)
	c.setCursor(text, cur)
}

// offset returns the rune offset of row and col in lines.
func offset(lines []string, row, col int) int {
	n := 0
	for i := 0; i < row && i < len(lines); i++ {
		n += len([]rune(lines[i])) + 1
	}
	return n + col
}

// setCursor moves the cursor of the body to the rune offset off.
func (c *composer) setCursor(text string, off int) {
	before := []rune(text)[:off]
	row := strings.Count(string(before), "\n")
	col := len(before)
	if i := strings.LastIndex(string(before), "\n"); i >= 0 {
		col = len([]rune(string(before)[i+1:]))
	}
	c.body.MoveToBegin()
	for guard := len(before); c.body.Line() < row && guard > 0; guard-- {
		c.body.CursorDown()
	}
	c.body.SetCursorColumn(col)
}

func (c *composer) target() gen.PreviewTarget {
	if c.mode == modeThread {
		return api.NewThreadTarget(c.nodeID)
	}
	return api.ReplyTarget(c.threadID)
}

func (c *composer) togglePreview() tea.Cmd {
	if c.preview {
		c.preview = false
		return nil
	}
	body := c.text()
	if strings.TrimSpace(body) == "" {
		c.errText = i18n.T("forum.compose.bodyRequired")
		return nil
	}
	c.previewing = true
	d, target, w, bbs := c.d, c.target(), ui.ReadingWidth(c.w), c.ed != nil
	if bbs {
		w = max(c.w-bodyIndent, 10)
	}
	return func() tea.Msg {
		html, err := d.API.Preview(bg(), body, target)
		if err != nil {
			return previewMsg{err: err}
		}
		text, err := previewText(d, html, w, bbs)
		return previewMsg{text: text, err: err}
	}
}

func (c *composer) previewed(msg previewMsg) tea.Cmd {
	c.previewing = false
	if msg.err != nil {
		return c.fail(msg.err)
	}
	c.errText, c.issues = "", nil
	c.preview = true
	c.pv.SetContent(strings.Trim(msg.text, "\n"))
	c.pv.GotoTop()
	return nil
}

// validate returns the text of the first local problem, or "".
func (c *composer) validate() string {
	switch {
	case c.mode == modeThread && strings.TrimSpace(c.title.Value()) == "":
		return i18n.T("forum.compose.titleRequired")
	case c.mode == modeThread && c.requirePrefix && c.prefix < 0:
		return i18n.T("forum.compose.prefixRequired")
	case strings.TrimSpace(c.text()) == "":
		return i18n.T("forum.compose.bodyRequired")
	}
	return ""
}

func (c *composer) send() tea.Cmd {
	if c.sending || c.mode == modeUnsupported {
		return nil
	}
	if text := c.validate(); text != "" {
		c.errText, c.issues = text, nil
		return nil
	}
	c.sending = true
	return c.sendCmd()
}

func (c *composer) sendCmd() tea.Cmd {
	a, body := c.d.API, c.text()
	switch c.mode {
	case modeThread:
		req := gen.ThreadsCreateJSONRequestBody{Title: strings.TrimSpace(c.title.Value()), Body: body}
		if c.prefix >= 0 {
			id := c.prefixes[c.prefix].Id
			req.PrefixId = &id
		}
		id := c.nodeID
		return func() tea.Msg {
			t, err := a.CreateThread(bg(), id, req)
			return sentMsg{thread: t, err: err}
		}
	case modeEdit:
		id := c.postID
		return func() tea.Msg {
			p, err := a.EditPost(bg(), id, body)
			return sentMsg{post: p, err: err}
		}
	}
	id := c.threadID
	return func() tea.Msg {
		p, err := a.Reply(bg(), id, body)
		return sentMsg{post: p, err: err}
	}
}

func (c *composer) sent(msg sentMsg) tea.Cmd {
	c.sending = false
	if msg.err != nil {
		return c.fail(msg.err)
	}
	if msg.thread != nil {
		t := msg.thread
		return tea.Batch(ui.Toast(i18n.T("forum.toast.sent"), ui.ToastSuccess),
			ui.Replace(newThreadAt(c.d, t.Thread.Id, t.Post.Id, t.Post.Position)))
	}
	done := "forum.toast.sent"
	if c.mode == modeEdit {
		done = "forum.toast.saved"
	}
	p := msg.post
	next := ui.Replace(newThreadAt(c.d, c.threadID, p.Id, p.Position))
	if c.back != nil {
		next = tea.Sequence(ui.Pop(), c.back.reloadAt(p.Position, p.Id))
	}
	return tea.Batch(ui.Toast(i18n.T(done), ui.ToastSuccess), next)
}

// fail shows the error in the composer. A 401 also goes to the app.
func (c *composer) fail(err error) tea.Cmd {
	c.errText, c.issues = ui.ErrorText(err), nil
	var e *api.Error
	if errors.As(err, &e) && len(e.Issues) > 1 {
		c.issues = e.Issues
	}
	if api.IsUnauthorized(err) {
		return ui.ErrorCmd(err)
	}
	return nil
}

func (c *composer) View(width, height int) string {
	st, t := c.d.Styles, c.d.Theme
	if c.mode == modeUnsupported {
		return ui.Empty(st, i18n.T("forum.compose.unsupported"), width, height)
	}
	if c.ed != nil {
		return c.edView(width, height)
	}
	lines := []string{ui.Header(st, t, c.Title(), width)}
	if c.preview {
		return strings.Join(append(lines, st.Muted.Render(i18n.T("forum.compose.previewHint")), "", c.pv.View()), "\n")
	}
	if c.mode == modeThread {
		lines = append(lines, c.label(fieldTitle, "forum.compose.titleLabel")+c.title.View(), c.prefixLine())
	}
	lines = append(lines, "", c.body.View(), "", c.status(width))
	lines = append(lines, c.messages()...)
	return strings.Join(lines, "\n")
}

func (c *composer) label(field int, id string) string {
	text := i18n.T(id) + ": "
	if c.field == field {
		return c.d.Styles.Key.Render(text)
	}
	return c.d.Styles.Muted.Render(text)
}

func (c *composer) prefixLine() string {
	if len(c.prefixes) == 0 {
		return ""
	}
	value := i18n.T("forum.compose.noPrefix")
	if c.prefix >= 0 {
		p := c.prefixes[c.prefix]
		value = prefixTag(c.d, &gen.ThreadPrefixTag{Label: p.Label, Color: p.Color})
	}
	return c.label(fieldPrefix, "forum.compose.prefixLabel") + "◀ " + value + " ▶"
}

func (c *composer) status(width int) string {
	st := c.d.Styles
	n := c.body.Length()
	count := strconv.Itoa(n) + "/" + strconv.Itoa(maxBody)
	countText := st.Muted.Render(count)
	if n >= maxBody {
		countText = st.Error.Render(count)
	}
	hint := i18n.T("forum.compose.hint")
	switch {
	case c.sending:
		hint = i18n.T("forum.compose.sending")
	case c.previewing:
		hint = i18n.T("forum.compose.previewing")
	}
	return fit(st.Muted.Render(hint), countText, ui.ReadingWidth(width))
}

func (c *composer) messages() []string {
	st := c.d.Styles
	var out []string
	if c.confirm {
		out = append(out, st.Warning.Render(i18n.T("forum.compose.discard")))
	}
	if c.errText != "" {
		out = append(out, st.Error.Render(ansi.Wrap(c.errText, ui.ReadingWidth(c.w), "")))
	}
	for _, is := range c.issues[min(1, len(c.issues)):] {
		out = append(out, st.Error.Render("· "+is.Message))
	}
	return out
}
