package ui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/albertgmz/sermo-cli/internal/i18n"
)

// The ASCII helpers draw the tables and boxes of the BBS interface with the
// glyphs + - = | only. Each line fills the given width exactly. Widths count
// terminal cells, so text with accents, wide glyphs or color codes fits too.
// A cell text that is too long ends in "...". The borders get the Muted
// style. A text without color codes gets a style of its kind. A text with
// color codes keeps its colors.

// Align sets the position of the text in a Column.
type Align int

// The alignments of a Column.
const (
	AlignLeft Align = iota
	AlignRight
	AlignCenter
)

// Column is one column of a Table. Width is the text width in cells
// without the padding. A Width of 0 takes the remaining width. Columns with
// a Width of 0 share the remaining width.
type Column struct {
	Title string
	Width int
	Align Align
}

type itemKind int

const (
	kindRow itemKind = iota
	kindHeader
	kindSub
	kindSection
	kindFull
	kindRule
)

type tableItem struct {
	kind     itemKind
	cells    []string
	selected bool
}

// Table is an ASCII table of a fixed width. Add the items in order, then
// call Render. The methods return the table, so that calls can chain.
//
//	+----+---------------+-------+
//	| ## | Foro          | Hilos |      Header
//	+====+===============+=======+
//	|    |  ===  GENERAL  ===    |      Section
//	| 01 | Anuncios      |    24 |      Row
//	|    | Noticias      |       |      Sub
//	+----+---------------+-------+      Rule
//	| (*) mensajes nuevos        |      Full
//	+----------------------------+
//
// The table draws its top and bottom lines. It also draws a line between a
// Full row and a row with columns.
type Table struct {
	width  int
	cols   []Column
	widths []int
	items  []tableItem
}

// NewTable returns an empty table of width cells with the columns cols.
func NewTable(width int, cols ...Column) *Table {
	t := &Table{width: width, cols: cols, widths: make([]int, len(cols))}
	rest, free := width-1-3*len(cols), 0
	for i, c := range cols {
		t.widths[i] = c.Width
		rest -= c.Width
		if c.Width == 0 {
			free++
		}
	}
	for i, c := range cols {
		if c.Width == 0 {
			t.widths[i] = max(rest/free, 1)
			rest -= t.widths[i]
			free--
		}
	}
	return t
}

func (t *Table) add(kind itemKind, cells ...string) *Table {
	t.items = append(t.items, tableItem{kind: kind, cells: cells})
	return t
}

// Header adds the row of the column titles and a line of = under it.
func (t *Table) Header() *Table {
	titles := make([]string, len(t.cols))
	for i, c := range t.cols {
		titles[i] = c.Title
	}
	return t.add(kindHeader, titles...)
}

// Row adds a data row with one text for each column. Missing cells are
// empty.
func (t *Table) Row(cells ...string) *Table { return t.add(kindRow, cells...) }

// Sub adds a second line to the row above it, for example a description.
// It gets the Muted style.
func (t *Table) Sub(cells ...string) *Table { return t.add(kindSub, cells...) }

// Section adds a section title. It spans all columns after the first.
func (t *Table) Section(title string) *Table { return t.add(kindSection, title) }

// Rule adds a separator line.
func (t *Table) Rule() *Table { return t.add(kindRule) }

// Full adds a row that spans the full width, for example a legend or a
// footer. Full rows that follow each other have no line between them.
func (t *Table) Full(text string) *Table { return t.add(kindFull, text) }

// Wrap adds Full rows that hold text broken into lines of the row width.
func (t *Table) Wrap(text string) *Table {
	for _, l := range strings.Split(lipgloss.Wrap(text, max(t.width-4, 1), " "), "\n") {
		t.Full(l)
	}
	return t
}

// Select marks the last item as the selected row. Its plain texts get the
// Key style.
func (t *Table) Select() *Table {
	if n := len(t.items); n > 0 {
		t.items[n-1].selected = true
	}
	return t
}

// tableDraw keeps the state of one Render call.
type tableDraw struct {
	t         *Table
	s         Styles
	lines     []string
	prevCols  bool // the last content item has columns
	afterRule bool // the last line is a separator line
	pending   bool // a Rule item waits for the next content item
}

// Render draws the table. Each line is exactly the table width.
func (t *Table) Render(s Styles) string {
	d := &tableDraw{t: t, s: s, afterRule: true}
	for _, it := range t.items {
		if it.kind == kindRule {
			d.pending = true
			continue
		}
		d.separate(len(d.lines) == 0, it.kind != kindFull)
		d.lines = append(d.lines, t.line(s, it))
		d.afterRule, d.pending = false, false
		if it.kind == kindHeader {
			d.lines = append(d.lines, t.rule(s, '=', true))
			d.afterRule = true
		}
	}
	if len(d.lines) > 0 && !d.afterRule {
		d.lines = append(d.lines, t.rule(s, '-', d.prevCols))
	}
	return strings.Join(d.lines, "\n")
}

// separate draws the line before a content item when the item needs one.
func (d *tableDraw) separate(first, cols bool) {
	switch {
	case first:
		d.lines = append(d.lines, d.t.rule(d.s, '-', cols))
	case !d.afterRule && (d.pending || cols != d.prevCols):
		d.lines = append(d.lines, d.t.rule(d.s, '-', cols || d.prevCols))
	}
	d.prevCols = cols
}

func (t *Table) rule(s Styles, ch rune, joins bool) string {
	if !joins {
		return Rule(s, t.width, ch)
	}
	parts := make([]string, len(t.widths))
	for i, w := range t.widths {
		parts[i] = strings.Repeat(string(ch), w+2)
	}
	return s.Muted.Render("+" + strings.Join(parts, "+") + "+")
}

func (t *Table) line(s Styles, it tableItem) string {
	bar := s.Muted.Render("|")
	st := t.style(s, it)
	switch it.kind {
	case kindFull:
		return bar + " " + cell(st, it.cells[0], t.width-4, AlignLeft) + " " + bar
	case kindSection:
		first := strings.Repeat(" ", t.widths[0]+2)
		span := cell(st, "  ===  "+it.cells[0]+"  ===", t.width-3-len(first), AlignLeft)
		return bar + first + bar + span + bar
	}
	parts := make([]string, len(t.cols))
	for i, c := range t.cols {
		text := ""
		if i < len(it.cells) {
			text = it.cells[i]
		}
		align := c.Align
		if it.kind == kindHeader {
			align = AlignLeft
		}
		parts[i] = " " + cell(st, text, t.widths[i], align) + " "
	}
	return bar + strings.Join(parts, bar) + bar
}

func (t *Table) style(s Styles, it tableItem) lipgloss.Style {
	switch {
	case it.selected:
		return s.Key
	case it.kind == kindHeader || it.kind == kindSection:
		return s.Subtitle
	case it.kind == kindSub:
		return s.Muted
	}
	return s.Text
}

// cell cuts text to w cells, puts it in its place in w cells and gives a
// plain text the style st.
func cell(st lipgloss.Style, text string, w int, align Align) string {
	if ansi.StringWidth(text) > w {
		text = ansi.Truncate(text, w, "...")
	}
	gap := max(w-ansi.StringWidth(text), 0)
	left := 0
	switch align {
	case AlignRight:
		left = gap
	case AlignCenter:
		left = gap / 2
	}
	if text != "" && !strings.Contains(text, "\x1b") {
		text = st.Render(text)
	}
	return strings.Repeat(" ", left) + text + strings.Repeat(" ", gap-left)
}

// Box draws lines in a box of width cells with - lines at the top and the
// bottom.
func Box(s Styles, width int, lines []string) string { return box(s, width, '-', lines) }

// DoubleBox draws lines in a box of width cells with = lines at the top and
// the bottom.
func DoubleBox(s Styles, width int, lines []string) string { return box(s, width, '=', lines) }

func box(s Styles, width int, ch rune, lines []string) string {
	bar := s.Muted.Render("|")
	out := make([]string, 0, len(lines)+2)
	out = append(out, Rule(s, width, ch))
	for _, l := range lines {
		out = append(out, bar+" "+cell(s.Text, l, max(width-4, 0), AlignLeft)+" "+bar)
	}
	return strings.Join(append(out, Rule(s, width, ch)), "\n")
}

// Rule returns a line of width cells: ch between two + corners.
func Rule(s Styles, width int, ch rune) string {
	return s.Muted.Render("+" + strings.Repeat(string(ch), max(width-2, 0)) + "+")
}

// Thousands formats n with the thousands mark of the current language,
// for example 4.182 in Spanish and 4,182 in English.
func Thousands(n int) string {
	digits := strconv.Itoa(max(n, -n))
	var b strings.Builder
	if n < 0 {
		b.WriteByte('-')
	}
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteString(i18n.T("bbs.thousands"))
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Num returns the row number i for a list of total rows: two digits or
// more, and as many digits as total has.
func Num(i, total int) string {
	return fmt.Sprintf("%0*d", max(2, len(strconv.Itoa(total))), i)
}
