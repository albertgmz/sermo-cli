package forum

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// editorFixed is the number of lines of the line editor table without the
// rows of the text: the frame lines, the heading, the help and the status.
const editorFixed = 8

// editorRow is one row of the text table. input marks the row of the
// typed line.
type editorRow struct {
	num, text string
	input     bool
}

// edView draws the line editor: the heading, the help, the numbered
// lines and the status in one table.
func (c *composer) edView(width, height int) string {
	s := c.d.Styles
	if c.preview {
		return c.previewView(width, height)
	}
	t := ui.NewTable(width, ui.Column{Width: 3, Align: ui.AlignRight}, ui.Column{})
	t.Full(s.Subtitle.Render(c.edHeading()))
	extra := c.threadRows()
	for _, l := range extra {
		t.Full(l)
	}
	t.Rule().Full(s.Muted.Render(i18n.T("bbs.editor.help")))
	msgs := c.edMessages(width)
	rows := max(height-editorFixed-len(extra)-len(msgs), 1)
	switch c.ed.panel {
	case panelHelp:
		c.helpRows(t, rows)
	case panelPrefixes:
		c.prefixRows(t, rows)
	default:
		c.textRows(t, width, rows)
	}
	t.Full(c.edStatus())
	for _, m := range msgs {
		t.Full(m)
	}
	return t.Render(s)
}

// edHeading is the target of the text, for example
// `RESPONDER  >  #1208  "title"`.
func (c *composer) edHeading() string {
	switch c.mode {
	case modeThread:
		return i18n.T("bbs.editor.thread") + "  >  " + c.nodeTitle
	case modeEdit:
		h := i18n.T("bbs.editor.edit")
		if c.back != nil {
			for _, p := range c.back.posts {
				if p.Id == c.postID {
					h += "  >  #" + strconv.Itoa(p.Position+1)
				}
			}
		}
		return h
	}
	h := i18n.T("bbs.editor.reply") + "  >  #" + strconv.Itoa(c.threadID)
	if c.back != nil && c.back.view != nil {
		h += "  \"" + c.back.view.Thread.Title + "\""
	}
	return h
}

// threadRows show the title and the prefix of a new thread.
func (c *composer) threadRows() []string {
	if c.mode != modeThread {
		return nil
	}
	rows := []string{i18n.Tf("bbs.editor.title", map[string]any{"Title": c.title.Value()})}
	if len(c.prefixes) > 0 {
		value := i18n.T("bbs.editor.noPrefix")
		if c.prefix >= 0 {
			value = "[" + c.prefixes[c.prefix].Label + "]"
		}
		rows = append(rows, i18n.Tf("bbs.editor.prefix", map[string]any{"Prefix": value}))
	}
	return rows
}

// textRows adds rows of the lines to t. The rows show the typed line, or
// the lines from the line top after /L. Empty rows fill the rest.
func (c *composer) textRows(t *ui.Table, width, rows int) {
	all, input := c.editorRows(max(width-10, 1))
	start := max(input-rows+1, 0)
	if c.ed.top >= 0 {
		start = 0
		for i, r := range all {
			if r.num == strconv.Itoa(c.ed.top+1) {
				start = i
				break
			}
		}
	}
	s := c.d.Styles
	for i := start; i < start+rows; i++ {
		if i >= len(all) {
			t.Row("", "")
			continue
		}
		text := all[i].text
		if all[i].input && i == input {
			text = s.Text.Render(strings.TrimSuffix(text, "_")) + s.Accent.Render("_")
		}
		t.Row(all[i].num, text)
	}
}

// editorRows returns the rows of the lines and the typed line, cut to w
// cells, and the index of the last row of the typed line. The numbers are
// the numbers that the commands use. The typed line at an insert point
// before the last line shows ">" in place of a number.
func (c *composer) editorRows(w int) ([]editorRow, int) {
	e := c.ed
	var out []editorRow
	input := 0
	for i := 0; i <= len(e.lines); i++ {
		if e.edit < 0 && i == e.at {
			label := ">"
			if i == len(e.lines) {
				label = strconv.Itoa(i + 1)
			}
			out = appendRows(out, label, e.cur+"_", w, true)
			input = len(out) - 1
		}
		if i == len(e.lines) {
			break
		}
		text, in := e.shown(i)
		out = appendRows(out, strconv.Itoa(i+1), text, w, in)
		if in {
			input = len(out) - 1
		}
	}
	return out, input
}

// shown returns the text of line i and whether it is the typed line.
func (e *lineEditor) shown(i int) (string, bool) {
	if i == e.edit {
		return e.cur + "_", true
	}
	return e.lines[i], false
}

// appendRows adds the rows of one line, cut to w cells. Only the first
// row shows the label.
func appendRows(out []editorRow, label, text string, w int, input bool) []editorRow {
	for j, part := range strings.Split(ansi.Hardwrap(text, w, true), "\n") {
		if j > 0 {
			label = ""
		}
		out = append(out, editorRow{num: label, text: part, input: input})
	}
	return out
}

// helpRows adds the commands of the editor to t.
func (c *composer) helpRows(t *ui.Table, rows int) {
	n := 0
	for _, sc := range slashCommands {
		if sc.name == "?" || n >= rows {
			continue
		}
		t.Row("/"+sc.name, strings.TrimSpace(sc.args+"  "+i18n.T(sc.help)))
		n++
	}
	if n < rows {
		t.Row("//", i18n.T("bbs.editor.cmd.slash"))
		n++
	}
	for ; n < rows; n++ {
		t.Row("", "")
	}
}

// prefixRows adds the numbered prefixes of the forum to t.
func (c *composer) prefixRows(t *ui.Table, rows int) {
	t.Row("", c.d.Styles.Muted.Render(i18n.T("bbs.editor.prefixes")))
	n := 1
	for i, p := range c.prefixes {
		if n >= rows {
			break
		}
		t.Row(ui.Num(i+1, len(c.prefixes)), p.Label)
		n++
	}
	for ; n < rows; n++ {
		t.Row("", "")
	}
}

// edStatus shows the line and the column of the typed line and the BB
// code tags.
func (c *composer) edStatus() string {
	e := c.ed
	line := e.at + 1
	if e.edit >= 0 {
		line = e.edit + 1
	}
	text := i18n.Tf("bbs.editor.status", map[string]any{
		"Line": line, "Lines": maxLines, "Col": ansi.StringWidth(e.cur) + 1, "Cols": lineCols,
	}) + "   " + i18n.T("bbs.editor.tags")
	switch {
	case c.sending:
		text += "   " + i18n.T("bbs.editor.sending")
	case c.previewing:
		text += "   " + i18n.T("bbs.editor.previewing")
	}
	return c.d.Styles.Muted.Render(text)
}

// edMessages are the rows of the question before a discard and of the
// errors of the server.
func (c *composer) edMessages(width int) []string {
	s := c.d.Styles
	var out []string
	if c.confirm {
		out = append(out, s.Warning.Render(i18n.T("bbs.editor.discard")))
	}
	if c.errText != "" {
		for _, l := range strings.Split(lipgloss.Wrap(c.errText, max(width-4, 1), " "), "\n") {
			out = append(out, s.Error.Render(l))
		}
	}
	for _, is := range c.issues[min(1, len(c.issues)):] {
		out = append(out, s.Error.Render("- "+is.Message))
	}
	return out
}

// previewView shows the preview of the server in the frame of the
// editor.
func (c *composer) previewView(width, height int) string {
	s := c.d.Styles
	t := ui.NewTable(width, ui.Column{})
	t.Full(s.Subtitle.Render(c.edHeading())).Rule().Full(s.Muted.Render(i18n.T("bbs.editor.previewHint"))).Rule()
	lines := strings.Split(strings.Trim(c.pv.GetContent(), "\n"), "\n")
	start := min(c.pv.YOffset(), max(len(lines)-1, 0))
	rows := max(height-6, 1)
	for i := start; i < start+rows; i++ {
		text := ""
		if i < len(lines) {
			text = " " + lines[i]
		}
		t.Full(text)
	}
	return t.Render(s)
}
