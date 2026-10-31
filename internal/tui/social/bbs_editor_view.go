package social

import (
	"strconv"
	"strings"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// editorRow is one numbered row of the line editor. label is the line
// number, or ">" for the input at an insert point.
type editorRow struct {
	label string
	text  string
	input bool
}

// helpIDs are the i18n IDs of the editor commands for the help view.
var helpIDs = []string{"S", "A", "C", "V", "E", "D", "I", "L", "?"}

// rows returns the numbered rows and the index of the input row, or -1
// while the editor asks the lines before the body. The numbers are the
// numbers that the commands use.
func (e *lineEditor) rows() ([]editorRow, int) {
	asking := len(e.answers) < len(e.asks)
	out := make([]editorRow, 0, len(e.lines)+1)
	at := -1
	input := func(label string) {
		at = len(out)
		out = append(out, editorRow{label: label, text: string(e.in) + "_", input: true})
	}
	for i, l := range e.lines {
		switch {
		case asking:
		case i == e.edit:
			input(strconv.Itoa(i + 1))
			continue
		case i == e.insert:
			input(">")
		}
		out = append(out, editorRow{label: strconv.Itoa(i + 1), text: l})
	}
	if !asking && at < 0 {
		input(strconv.Itoa(len(e.lines) + 1))
	}
	return out, at
}

// rowOf returns the index of the row of line n, or 0.
func rowOf(rows []editorRow, n int) int {
	label := strconv.Itoa(n)
	for i, r := range rows {
		if r.label == label {
			return i
		}
	}
	return 0
}

func (e *lineEditor) view() string {
	s := e.d.Styles
	t := ui.NewTable(e.w, ui.Column{Width: 3, Align: ui.AlignRight}, ui.Column{})
	t.Full(s.Subtitle.Render(e.heading)).Rule()
	switch e.mode {
	case viewHelp:
		return e.helpView(t)
	case viewPreview:
		return e.previewView(t)
	}
	t.Full(i18n.T("bbs.social.editor.commands"))
	e.askRows(t)
	rows, at := e.rows()
	avail := e.h - editorFrame - len(e.asks)
	if len(e.asks) > 0 {
		avail--
	}
	avail = max(avail, 1)
	start := max(0, at-avail+1)
	if e.top >= 0 {
		start = rowOf(rows, e.top+1)
	}
	shown := 0
	for i := start; i < len(rows) && shown < avail; i++ {
		r := rows[i]
		t.Row(r.label, r.text)
		if r.input {
			t.Select()
		}
		shown++
	}
	for ; shown < avail; shown++ {
		t.Row()
	}
	t.Full(e.status())
	return t.Render(s)
}

// askRows adds the lines before the body, with the input on the line
// that the editor asks now.
func (e *lineEditor) askRows(t *ui.Table) {
	if len(e.asks) > 0 {
		t.Rule()
	}
	for i, label := range e.asks {
		switch {
		case i < len(e.answers):
			t.Full(label + " " + e.answers[i])
		case i == len(e.answers):
			t.Full(e.d.Styles.Key.Render(label + " " + string(e.in) + "_"))
		default:
			t.Full(label)
		}
	}
}

// status is the line under the text: the line and the column of the
// input and the BB code tags.
func (e *lineEditor) status() string {
	if e.sending {
		return i18n.T("social.editor.sending")
	}
	line := len(e.answers) + 1
	switch {
	case len(e.answers) < len(e.asks):
	case e.edit >= 0:
		line = e.edit + 1
	case e.insert >= 0:
		line = e.insert + 1
	default:
		line = len(e.lines) + 1
	}
	return i18n.Tf("bbs.social.editor.status", map[string]any{
		"Line": line, "Lines": maxLines, "Col": len(e.in) + 1, "Cols": e.cols(),
	})
}

func (e *lineEditor) helpView(t *ui.Table) string {
	t.Full(i18n.T("bbs.social.editor.helpTitle")).Rule()
	for _, id := range helpIDs {
		t.Full(i18n.T("bbs.social.editor.help." + id))
	}
	t.Rule().Full(i18n.T("bbs.social.editor.backToText"))
	return t.Render(e.d.Styles)
}

func (e *lineEditor) previewView(t *ui.Table) string {
	s := e.d.Styles
	t.Full(i18n.T("social.key.preview")).Rule()
	avail := max(e.h-7, 1)
	var lines []string
	switch p := e.preview; {
	case !p.done:
		lines = []string{i18n.T("social.editor.previewing")}
	case p.err != nil:
		lines = []string{s.Error.Render(ui.ErrorText(p.err))}
	default:
		text, _, err := e.d.Render.PostBBS(p.html, nil, max(e.w-4, 10))
		if err != nil {
			text = err.Error()
		}
		lines = strings.Split(strings.TrimRight(text, "\n"), "\n")
	}
	for _, l := range firstLines(lines, avail) {
		t.Full(l)
	}
	t.Rule().Full(i18n.T("bbs.social.editor.backToText"))
	return t.Render(s)
}
