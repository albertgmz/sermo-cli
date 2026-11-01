package ui

import (
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// The zones are the clickable parts of a view. A screen marks a part with
// Deps.Mark and an ID. The app finds the position of each zone when it
// draws the screen. On a click, the screen asks Deps.Clicked for each ID.

// markerRe matches one zone marker.
var markerRe = regexp.MustCompile(`\x1b\[[0-9]+z`)

var zoneCount atomic.Int64

// ZonePrefix returns a new prefix for the zone IDs of one screen. Each call
// gives a different prefix.
func ZonePrefix() string {
	return "z" + strconv.FormatInt(zoneCount.Add(1), 10) + "."
}

// Mark puts s in the zone id. It returns s unchanged when there are no
// zones or the mouse is off.
func (d *Deps) Mark(id, s string) string {
	if !d.zonesOn() {
		return s
	}
	return d.Zones.Mark(id, s)
}

// Clicked reports whether msg is a release of the left button in the zone
// id of the last frame.
func (d *Deps) Clicked(id string, msg tea.Msg) bool {
	m, ok := msg.(tea.MouseReleaseMsg)
	if !ok || m.Button != tea.MouseLeft || !d.zonesOn() {
		return false
	}
	return d.Zones.Get(id).InBounds(m)
}

// ClickedSide returns -1 when msg clicks the left half of the zone id, and
// 1 when it clicks the right half. A page counter uses it for the previous
// page and the next page.
func (d *Deps) ClickedSide(id string, msg tea.Msg) (int, bool) {
	if !d.Clicked(id, msg) {
		return 0, false
	}
	z := d.Zones.Get(id)
	if x, _ := z.Pos(msg.(tea.MouseReleaseMsg)); x < (z.EndX-z.StartX+1)/2 {
		return -1, true
	}
	return 1, true
}

func (d *Deps) zonesOn() bool {
	return d != nil && d.Zones != nil && d.Zones.Enabled()
}

// IsClick reports whether msg is a release of the left button.
func IsClick(msg tea.Msg) bool {
	m, ok := msg.(tea.MouseReleaseMsg)
	return ok && m.Button == tea.MouseLeft
}

// zoneMark is one zone marker at a cell of a line.
type zoneMark struct {
	cell int
	seq  string
}

// KeepZones copies the zone markers of src into dst. Each marker goes to
// the same line and the same cell. Use it after a function draws src again
// and drops the markers, for example a canvas.
func KeepZones(src, dst string) string {
	if !markerRe.MatchString(src) {
		return dst
	}
	srcLines := strings.Split(src, "\n")
	dstLines := strings.Split(dst, "\n")
	for i := range min(len(srcLines), len(dstLines)) {
		if marks := findMarks(srcLines[i]); len(marks) > 0 {
			dstLines[i] = insertMarks(dstLines[i], marks)
		}
	}
	return strings.Join(dstLines, "\n")
}

func findMarks(line string) []zoneMark {
	var out []zoneMark
	for _, loc := range markerRe.FindAllStringIndex(line, -1) {
		out = append(out, zoneMark{cell: ansi.StringWidth(line[:loc[0]]), seq: line[loc[0]:loc[1]]})
	}
	return out
}

// insertMarks puts each mark before the text at its cell. The marks after
// the end of the text go at the end of the line.
func insertMarks(line string, marks []zoneMark) string {
	var b strings.Builder
	cell, state := 0, byte(0)
	for line != "" {
		seq, w, n, next := ansi.DecodeSequence(line, state, nil)
		for w > 0 && len(marks) > 0 && marks[0].cell <= cell {
			b.WriteString(marks[0].seq)
			marks = marks[1:]
		}
		b.WriteString(seq)
		cell += w
		line, state = line[n:], next
	}
	for _, m := range marks {
		b.WriteString(m.seq)
	}
	return b.String()
}

// hasColor reports whether text has escape codes other than zone markers.
func hasColor(text string) bool {
	return strings.Contains(markerRe.ReplaceAllString(text, ""), "\x1b")
}

// MarkText puts the first text in line in the zone id. line can have
// colors and zones. text is plain text. When line does not hold text,
// MarkText returns line unchanged.
func (d *Deps) MarkText(id, line, text string) string {
	plain := ansi.Strip(line)
	i := strings.Index(plain, text)
	if !d.zonesOn() || text == "" || i < 0 {
		return line
	}
	return KeepZones(plain[:i]+d.Mark(id, text)+plain[i+len(text):], line)
}
