package social

import (
	"context"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/api/gen"
	"github.com/albertgmz/sermo-cli/internal/bbcode"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// draft is the text that the editor sends.
type draft struct {
	Body, Title string
	To          []int
	// Names are the typed usernames of the recipients in the line editor.
	Names []string
}

// sendFunc sends a draft. It returns the message for the screen on success.
type sendFunc func(ctx context.Context, v draft) (tea.Msg, error)

type previewMsg struct {
	html string
	err  error
}

type sendFailedMsg struct{ err error }

// wrapTags are the hotkeys that put the selection between BB code tags.
var wrapTags = map[string]string{"ctrl+b": "B", "ctrl+i": "I", "ctrl+u": "U", "ctrl+k": "URL"}

// editor is a small BB code editor for messages, profile posts and
// comments. For a new conversation it also has the recipients and the
// title. The screen that owns it shows it in place of its content and
// gives it all messages while it is open.
type editor struct {
	d       *ui.Deps
	heading string
	target  gen.PreviewTarget
	send    sendFunc
	to      *recipients // nil when the editor has only a body
	title   textinput.Model
	body    textarea.Model
	focus   int
	preview *previewState
	sending bool
	closed  bool
	w, h    int
	zid     string // the prefix of the zone IDs
}

type previewState struct {
	html string
	err  error
	done bool
}

// newEditor returns an editor for a body with the start text.
func newEditor(d *ui.Deps, heading string, target gen.PreviewTarget, text string, send sendFunc) *editor {
	e := &editor{d: d, heading: heading, target: target, send: send, body: newTextarea(d), zid: ui.ZonePrefix()}
	e.body.SetValue(text)
	e.focusField()
	return e
}

// newConversationEditor returns an editor with recipients, a title and a
// body.
func newConversationEditor(d *ui.Deps, send sendFunc) *editor {
	e := newEditor(d, i18n.T("social.editor.newConversation"), api.MessageTarget(0), "", send)
	e.to = newRecipients(d)
	e.title = newInput(d)
	e.title.Placeholder = i18n.T("social.editor.titlePlaceholder")
	e.focusField()
	return e
}

func newTextarea(d *ui.Deps) textarea.Model {
	t := textarea.New()
	st := textarea.DefaultStyles(d.Glamour != "light")
	st.Cursor.Blink = false
	t.SetStyles(st)
	t.ShowLineNumbers = false
	t.Prompt = ""
	t.MaxHeight = 0
	t.Placeholder = i18n.T("social.editor.placeholder")
	return t
}

func newInput(d *ui.Deps) textinput.Model {
	in := textinput.New()
	st := textinput.DefaultStyles(d.Glamour != "light")
	st.Cursor.Blink = false
	in.SetStyles(st)
	in.Prompt = ""
	return in
}

// fields returns the number of fields: the body only, or recipients,
// title and body.
func (e *editor) fields() int {
	if e.to == nil {
		return 1
	}
	return 3
}

func (e *editor) bodyFocused() bool { return e.focus == e.fields()-1 }

// focusField gives the focus to the field e.focus. The cursor does not
// blink, so the Cmds of Focus are not needed.
func (e *editor) focusField() {
	e.body.Blur()
	e.title.Blur()
	if e.to != nil {
		e.to.in.Blur()
	}
	switch {
	case e.bodyFocused():
		_ = e.body.Focus()
	case e.focus == 0:
		_ = e.to.in.Focus()
	default:
		_ = e.title.Focus()
	}
}

func (e *editor) setSize(w, h int) {
	e.w, e.h = w, h
	e.body.SetWidth(ui.ReadingWidth(w))
	e.body.SetHeight(max(h-e.reserved(), 3))
}

// reserved is the number of rows that the editor uses outside the body.
func (e *editor) reserved() int {
	if e.to == nil {
		return 4
	}
	return 6 + 3 + maxSuggestions
}

func (e *editor) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return e.key(msg)
	case tea.PasteMsg:
		return e.input(msg)
	case tea.MouseReleaseMsg:
		return e.click(msg)
	case previewMsg:
		if e.preview != nil {
			e.preview = &previewState{html: msg.html, err: msg.err, done: true}
		}
		return unauthorized(msg.err)
	case sendFailedMsg:
		e.sending = false
		return ui.ErrorCmd(msg.err)
	case debounceMsg, suggestMsg:
		if e.to != nil {
			return e.to.update(msg)
		}
	}
	return nil
}

func (e *editor) key(msg tea.KeyPressMsg) tea.Cmd {
	if cmd, ok := e.command(msg.String()); ok {
		return cmd
	}
	return e.input(msg)
}

// command runs the editor action of the key k. It returns false when k
// is not an action key.
func (e *editor) command(k string) (tea.Cmd, bool) {
	switch k {
	case "esc":
		e.escape()
		return nil, true
	case "ctrl+s":
		return e.submit(), true
	case "ctrl+p":
		return e.togglePreview(), true
	case "tab":
		e.focus = (e.focus + 1) % e.fields()
		e.focusField()
		return nil, true
	case "shift+tab":
		e.focus = (e.focus + e.fields() - 1) % e.fields()
		e.focusField()
		return nil, true
	}
	if tag, ok := wrapTags[k]; ok && e.bodyFocused() && e.preview == nil {
		e.wrap(tag)
		return nil, true
	}
	return nil, false
}

// escape closes the preview, or the editor when no preview is open.
func (e *editor) escape() {
	if e.preview != nil {
		e.preview = nil
		return
	}
	e.closed = true
}

// input gives a key or a paste to the field with the focus.
func (e *editor) input(msg tea.Msg) tea.Cmd {
	if e.preview != nil {
		return nil
	}
	var cmd tea.Cmd
	switch {
	case e.bodyFocused():
		e.body, cmd = e.body.Update(msg)
	case e.focus == 0:
		cmd = e.to.update(msg)
	default:
		e.title, cmd = e.title.Update(msg)
	}
	return cmd
}

// wrap puts the selection, or the cursor, between the tags.
func (e *editor) wrap(tag string) {
	text := e.body.Value()
	lines := strings.Split(text, "\n")
	start := runeOffset(lines, e.body.Line(), e.body.Column())
	end := start
	if a, b, ok := e.body.Selection(); ok {
		start, end = runeOffset(lines, a.Row, a.Col), runeOffset(lines, b.Row, b.Col)
	}
	out, cursor := bbcode.Wrap(text, tag, start, end)
	e.body.SetValue(out)
	e.moveCursor(out, cursor)
}

// runeOffset returns the offset in runes of the column col of the line row.
func runeOffset(lines []string, row, col int) int {
	off := 0
	for i := 0; i < row && i < len(lines); i++ {
		off += len([]rune(lines[i])) + 1
	}
	return off + col
}

// moveCursor puts the cursor of the body at the rune offset off of text.
// SetValue leaves the cursor at the end, so it moves up to the line.
func (e *editor) moveCursor(text string, off int) {
	before := string([]rune(text)[:off])
	row := strings.Count(before, "\n")
	col := len([]rune(before[strings.LastIndex(before, "\n")+1:]))
	for i := 0; e.body.Line() > row && i <= len(before); i++ {
		e.body.CursorUp()
	}
	e.body.SetCursorColumn(col)
}

func (e *editor) togglePreview() tea.Cmd {
	if e.preview != nil {
		e.preview = nil
		return nil
	}
	body := e.body.Value()
	if strings.TrimSpace(body) == "" {
		return nil
	}
	e.preview = &previewState{}
	client, target := e.d.API, e.target
	return func() tea.Msg {
		html, err := client.Preview(bg, body, target)
		return previewMsg{html: html, err: err}
	}
}

func (e *editor) draft() draft {
	v := draft{Body: e.body.Value()}
	if e.to != nil {
		v.Title, v.To = strings.TrimSpace(e.title.Value()), e.to.ids()
	}
	return v
}

// problem returns the i18n ID of the reason why the draft cannot go, or "".
func (e *editor) problem(v draft) string {
	switch {
	case e.to != nil && len(v.To) == 0:
		return "social.editor.noRecipients"
	case e.to != nil && v.Title == "":
		return "social.editor.noTitle"
	case strings.TrimSpace(v.Body) == "":
		return "social.editor.empty"
	}
	return ""
}

func (e *editor) submit() tea.Cmd {
	if e.sending {
		return nil
	}
	v := e.draft()
	if id := e.problem(v); id != "" {
		return ui.Toast(i18n.T(id), ui.ToastError)
	}
	e.sending = true
	send := e.send
	return func() tea.Msg {
		msg, err := send(bg, v)
		if err != nil {
			return sendFailedMsg{err: err}
		}
		return msg
	}
}

func (e *editor) view() string {
	s, t := e.d.Styles, e.d.Theme
	lines := []string{ui.Header(s, t, e.heading, e.w), ""}
	if e.to != nil {
		lines = append(lines, e.fieldsView()...)
	}
	area := e.h - len(lines) - 2
	if e.preview != nil {
		lines = append(lines, e.previewView(area))
	} else {
		lines = append(lines, e.body.View())
	}
	return pad(lines, e.h-2) + "\n\n" + ansiTrim(e.helpView(), e.w)
}

// helpSep is the text between two parts of the help line.
const helpSep = " · "

// helpParts splits the help line into its parts. The first word of a
// part is the key of its action, for example "Ctrl+S".
func (e *editor) helpParts() []string { return strings.Split(e.help(), helpSep) }

// helpKey returns the key of the action of a help part, for example
// "ctrl+s" for "Ctrl+S enviar".
func helpKey(part string) string {
	k, _, _ := strings.Cut(part, " ")
	return strings.ToLower(k)
}

// helpView paints the help line and puts each part in a zone.
func (e *editor) helpView() string {
	s := e.d.Styles
	parts := e.helpParts()
	out := make([]string, len(parts))
	for j, p := range parts {
		out[j] = e.d.Mark(e.zid+"hint."+strconv.Itoa(j), s.Muted.Render(p))
	}
	return strings.Join(out, s.Muted.Render(helpSep))
}

// click runs the action of a clicked part of the help line, like its key.
func (e *editor) click(msg tea.MouseReleaseMsg) tea.Cmd {
	for j, p := range e.helpParts() {
		if e.d.Clicked(e.zid+"hint."+strconv.Itoa(j), msg) {
			cmd, _ := e.command(helpKey(p))
			return cmd
		}
	}
	return nil
}

func (e *editor) fieldsView() []string {
	s := e.d.Styles
	label := func(i int, id string) string {
		if e.focus == i {
			return s.Key.Render(i18n.T(id) + " ")
		}
		return s.Muted.Render(i18n.T(id) + " ")
	}
	to := e.to.view(e.w-12, e.focus == 0)
	lines := []string{label(0, "social.editor.to") + to[0]}
	for _, l := range to[1:] {
		lines = append(lines, "   "+l)
	}
	for range maxSuggestions + 1 - len(to) {
		lines = append(lines, "")
	}
	e.title.SetWidth(max(e.w-12, 8))
	return append(lines, label(1, "social.editor.title")+e.title.View(), "")
}

func (e *editor) previewView(height int) string {
	s := e.d.Styles
	p := e.preview
	switch {
	case !p.done:
		return s.Muted.Render(i18n.T("social.editor.previewing"))
	case p.err != nil:
		return s.Error.Render(ui.ErrorText(p.err))
	}
	text, _, err := e.d.Render.Post(p.html, nil, ui.ReadingWidth(e.w), e.d.Glamour)
	if err != nil {
		return s.Error.Render(err.Error())
	}
	return strings.Join(firstLines(strings.Split(text, "\n"), height), "\n")
}

func (e *editor) help() string {
	if e.sending {
		return i18n.T("social.editor.sending")
	}
	if e.preview != nil {
		return i18n.T("social.editor.previewHelp")
	}
	if e.to != nil {
		return i18n.T("social.editor.helpFields")
	}
	return i18n.T("social.editor.help")
}

func firstLines(lines []string, n int) []string {
	return lines[:min(len(lines), max(n, 0))]
}

// ansiTrim cuts s to width columns.
func ansiTrim(s string, width int) string {
	return lipgloss.NewStyle().MaxWidth(max(width, 1)).Render(s)
}

// routeEditor gives msg to the open editor *ed when msg belongs to it,
// and removes the editor when it closes.
func routeEditor(ed **editor, msg tea.Msg) (tea.Cmd, bool) {
	e := *ed
	if e == nil || !forEditor(msg) {
		return nil, false
	}
	cmd := e.update(msg)
	if e.closed {
		*ed = nil
	}
	return cmd, true
}

// forEditor reports whether msg belongs to an open editor.
func forEditor(msg tea.Msg) bool {
	switch msg.(type) {
	case tea.KeyPressMsg, tea.PasteMsg, tea.MouseReleaseMsg, previewMsg, sendFailedMsg, debounceMsg, suggestMsg:
		return true
	}
	return false
}
