package social

import (
	"strconv"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/api/gen"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

const (
	// maxLines is the largest number of lines of a text in the line
	// editor.
	maxLines = 200
	// lineCols is the largest number of characters on one line.
	lineCols = 72
	// editorFrame is the number of lines of the editor frame around the
	// text rows.
	editorFrame = 8
)

// The views of the line editor.
const (
	viewWrite = iota
	viewPreview
	viewHelp
)

// lineEditor is the editor of the BBS interface. The member types one
// line at a time. A line that starts with "/" and a letter is a command:
// /S sends, /A aborts, /C quotes, /V shows a preview, /E n edits line n,
// /D n deletes line n, /I n inserts before line n, /L n lists from line n
// and /? shows the help. The screen that owns it shows it in place of its
// content and gives it all messages while it is open.
type lineEditor struct {
	d       *ui.Deps
	heading string
	target  gen.PreviewTarget
	send    sendFunc
	// quote returns the Cmd that gets a quote, or nil when the place has
	// nothing to quote.
	quote func() tea.Cmd
	// asks are the labels of the lines before the body, for example the
	// recipients and the title of a new conversation.
	asks    []string
	answers []string
	lines   []string
	in      []rune
	edit    int // the index of the line under edit, or -1
	insert  int // the index for the next line, or -1 for the end
	top     int // the first listed line, or -1 to follow the input
	mode    int
	preview *previewState
	sending bool
	closed  bool
	w, h    int
}

// newLineEditor returns a line editor for a body with the start text.
func newLineEditor(d *ui.Deps, heading string, target gen.PreviewTarget, text string, send sendFunc) *lineEditor {
	e := &lineEditor{d: d, heading: heading, target: target, send: send, edit: -1, insert: -1, top: -1}
	if text != "" {
		e.addText(text)
	}
	return e
}

func (e *lineEditor) setSize(w, h int) { e.w, e.h = w, h }

// cols is the length limit of a line: lineCols, or less when the window
// is too narrow.
func (e *lineEditor) cols() int {
	return max(min(lineCols, e.w-11), 10)
}

func (e *lineEditor) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return e.key(msg)
	case tea.PasteMsg:
		return e.paste(msg.Content)
	case previewMsg:
		if e.preview != nil {
			e.preview = &previewState{html: msg.html, err: msg.err, done: true}
		}
		return unauthorized(msg.err)
	case sendFailedMsg:
		e.sending = false
		return ui.ErrorCmd(msg.err)
	}
	return nil
}

func (e *lineEditor) key(msg tea.KeyPressMsg) tea.Cmd {
	if e.mode != viewWrite {
		if k := msg.String(); k == "esc" || k == "enter" {
			e.mode, e.preview = viewWrite, nil
		}
		return nil
	}
	switch msg.String() {
	case "esc":
		e.closed = true
		return nil
	case "enter":
		return e.enter()
	case "backspace":
		e.backspace()
		return nil
	}
	if msg.Text != "" && msg.Mod&^tea.ModShift == 0 {
		return e.typeText(msg.Text)
	}
	return nil
}

// paste adds the text at the input. Each line break ends a line, and the
// editor keeps these lines as they are. The text after the last line
// break goes to the input.
func (e *lineEditor) paste(text string) tea.Cmd {
	parts := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for _, part := range parts[:len(parts)-1] {
		if !e.commit(string(e.in) + part) {
			return e.full()
		}
		e.in = nil
	}
	return e.typeText(parts[len(parts)-1])
}

// typeText adds text to the input line. When the line gets too long, the
// last word moves to a new line.
func (e *lineEditor) typeText(text string) tea.Cmd {
	e.in = append(e.in, []rune(text)...)
	for len(e.in) > e.cols() {
		cut := e.cols()
		if sp := strings.LastIndex(string(e.in[:cut]), " "); sp > 0 {
			cut = utf8.RuneCountInString(string(e.in[:cut])[:sp])
		}
		head := strings.TrimRight(string(e.in[:cut]), " ")
		rest := []rune(strings.TrimLeft(string(e.in[cut:]), " "))
		if !e.commit(head) {
			e.in = []rune(head)
			return e.full()
		}
		e.in = rest
	}
	return nil
}

// backspace removes the last character. On an empty new line at the end
// it takes back the last line.
func (e *lineEditor) backspace() {
	if len(e.in) > 0 {
		e.in = e.in[:len(e.in)-1]
		return
	}
	if e.edit < 0 && e.insert < 0 && len(e.answers) >= len(e.asks) && len(e.lines) > 0 {
		e.in = []rune(e.lines[len(e.lines)-1])
		e.lines = e.lines[:len(e.lines)-1]
	}
}

// enter runs the command on the input line, or adds the line.
func (e *lineEditor) enter() tea.Cmd {
	text := string(e.in)
	if name, arg, ok := lineCommand(text); ok {
		e.in = nil
		return e.run(name, arg)
	}
	if !e.commit(text) {
		return e.full()
	}
	e.in = nil
	return nil
}

// lineCommand splits a command line like "/D 3" into "D" and "3".
func lineCommand(text string) (string, string, bool) {
	rest, ok := strings.CutPrefix(strings.TrimSpace(text), "/")
	if !ok || rest == "" {
		return "", "", false
	}
	name, arg, _ := strings.Cut(rest, " ")
	if utf8.RuneCountInString(name) != 1 || !strings.Contains("SACVEDIL?", strings.ToUpper(name)) {
		return "", "", false
	}
	return strings.ToUpper(name), strings.TrimSpace(arg), true
}

// commit puts a finished line in its place. It returns false when the
// text has no room for one more line.
func (e *lineEditor) commit(text string) bool {
	e.top = -1
	switch {
	case len(e.answers) < len(e.asks):
		e.answers = append(e.answers, strings.TrimSpace(text))
	case e.edit >= 0:
		e.lines[e.edit] = text
		e.insert, e.edit = e.edit+1, -1
		if e.insert >= len(e.lines) {
			e.insert = -1
		}
	case len(e.lines) >= maxLines:
		return false
	case e.insert >= 0:
		e.lines = append(e.lines[:e.insert], append([]string{text}, e.lines[e.insert:]...)...)
		e.insert++
	default:
		e.lines = append(e.lines, text)
	}
	return true
}

// addText adds the lines of text at the input place, for example a quote.
// The editor keeps the lines as they are: it does not wrap them.
func (e *lineEditor) addText(text string) tea.Cmd {
	if text == "" {
		return nil
	}
	for _, l := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		if !e.commit(l) {
			return e.full()
		}
	}
	return nil
}

func (e *lineEditor) full() tea.Cmd {
	return ui.Toast(i18n.Tf("bbs.social.editor.full", map[string]any{"N": maxLines}), ui.ToastError)
}

// run runs the editor command name with arg.
func (e *lineEditor) run(name, arg string) tea.Cmd {
	switch name {
	case "S":
		return e.submit()
	case "A":
		e.closed = true
	case "C":
		if e.quote == nil {
			return ui.Toast(i18n.T("bbs.social.editor.noQuote"), ui.ToastInfo)
		}
		return e.quote()
	case "V":
		return e.startPreview()
	case "?":
		e.mode = viewHelp
	default:
		return e.numbered(name, arg)
	}
	return nil
}

// numbered runs a command that takes a line number. Without a number,
// /D takes the last line, /L lists from the first line and /I goes back
// to the end.
func (e *lineEditor) numbered(name, arg string) tea.Cmd {
	switch {
	case name == "I" && arg == "":
		e.insert = -1
		return nil
	case name == "L" && arg == "":
		e.top = 0
		return nil
	}
	n, ok := e.lineNumber(arg)
	if !ok {
		return ui.Toast(i18n.T("bbs.social.editor.noLine"), ui.ToastError)
	}
	switch name {
	case "L":
		e.top = n - 1
	case "E":
		e.edit, e.insert, e.in = n-1, -1, []rune(e.lines[n-1])
	case "D":
		e.lines = append(e.lines[:n-1], e.lines[n:]...)
		e.edit, e.insert = -1, -1
	case "I":
		e.insert, e.edit = n-1, -1
	}
	return nil
}

// lineNumber returns the line number in arg, or the last line without
// arg. It returns false when no line has the number.
func (e *lineEditor) lineNumber(arg string) (int, bool) {
	if arg == "" {
		return len(e.lines), len(e.lines) > 0
	}
	n, err := strconv.Atoi(arg)
	return n, err == nil && n >= 1 && n <= len(e.lines)
}

func (e *lineEditor) body() string { return strings.Join(e.lines, "\n") }

func (e *lineEditor) startPreview() tea.Cmd {
	body := e.body()
	if strings.TrimSpace(body) == "" {
		return ui.Toast(i18n.T("social.editor.empty"), ui.ToastError)
	}
	e.mode, e.preview = viewPreview, &previewState{}
	client, target := e.d.API, e.target
	return func() tea.Msg {
		html, err := client.Preview(bg, body, target)
		return previewMsg{html: html, err: err}
	}
}

// draft returns the text to send. The first answer holds the names of
// the recipients and the second the title.
func (e *lineEditor) draft() draft {
	v := draft{Body: e.body()}
	if len(e.answers) > 0 {
		for _, name := range strings.Split(e.answers[0], ",") {
			if name = strings.TrimSpace(name); name != "" {
				v.Names = append(v.Names, name)
			}
		}
	}
	if len(e.answers) > 1 {
		v.Title = e.answers[1]
	}
	return v
}

// problem returns the i18n ID of the reason why the draft cannot go, or "".
func (e *lineEditor) problem(v draft) string {
	switch {
	case len(e.asks) > 0 && len(v.Names) == 0:
		return "social.editor.noRecipients"
	case len(e.asks) > 1 && v.Title == "":
		return "social.editor.noTitle"
	case strings.TrimSpace(v.Body) == "":
		return "social.editor.empty"
	}
	return ""
}

func (e *lineEditor) submit() tea.Cmd {
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

// routeLineEditor gives msg to the open line editor *ed when msg belongs
// to it, and removes the editor when it closes.
func routeLineEditor(ed **lineEditor, msg tea.Msg) (tea.Cmd, bool) {
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

// editorHints are the hints while the line editor *ed is open. A click on
// a hint runs its editor command like a typed command line. The line
// under input does not change. The editor closes on /A.
func editorHints(ed **lineEditor) []ui.Command {
	run := func(name string) func(string) tea.Cmd {
		return func(string) tea.Cmd {
			e := *ed
			if e == nil {
				return nil
			}
			cmd := e.run(name, "")
			if e.closed {
				*ed = nil
			}
			return cmd
		}
	}
	cmds := []ui.Command{
		{Name: "/S", Label: i18n.T("bbs.social.editor.hintSend"), Run: run("S")},
		{Name: "/A", Label: i18n.T("bbs.social.editor.hintAbort"), Run: run("A")},
	}
	if e := *ed; e != nil && e.quote != nil {
		cmds = append(cmds, ui.Command{Name: "/C", Label: i18n.T("bbs.social.editor.hintQuote"), Run: run("C")})
	}
	return append(cmds,
		ui.Command{Name: "/V", Label: i18n.T("bbs.social.editor.hintPreview"), Run: run("V")},
		ui.Command{Name: "/?", Label: i18n.T("bbs.social.editor.hintHelp"), Run: run("?")},
	)
}
