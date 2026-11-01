package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone/v2"

	"github.com/albertgmz/sermo-cli/internal/theme"
	"github.com/albertgmz/sermo-cli/internal/tui/ui/zonetest"
)

func zoneDeps(t *testing.T) *Deps {
	t.Helper()
	th := theme.ByID("")
	z := zone.New()
	t.Cleanup(z.Close)
	return &Deps{Theme: th, Styles: NewStyles(th), Zones: z}
}

func TestMarkWithoutZonesKeepsText(t *testing.T) {
	var d *Deps
	if got := d.Mark("a", "text"); got != "text" {
		t.Fatalf("nil Deps: Mark = %q", got)
	}
	if got := (&Deps{}).Mark("a", "text"); got != "text" {
		t.Fatalf("no manager: Mark = %q", got)
	}
	if (&Deps{}).Clicked("a", zonetest.At(0, 0)) {
		t.Fatal("no manager must not click")
	}
}

func TestClickedNeedsTheLeftButtonInTheZone(t *testing.T) {
	d := zoneDeps(t)
	view := "ab " + d.Mark("x", "CD") + "\nnext"
	z := zonetest.Zone(t, d.Zones, view, "x")
	if z.StartX != 3 || z.EndX != 4 || z.StartY != 0 {
		t.Fatalf("zone = %+v, want x 3..4 on line 0", *z)
	}
	if !d.Clicked("x", zonetest.At(4, 0)) {
		t.Fatal("left release in the zone must click")
	}
	if d.Clicked("x", zonetest.At(5, 0)) || d.Clicked("x", zonetest.At(3, 1)) {
		t.Fatal("a release out of the zone must not click")
	}
	if d.Clicked("x", tea.MouseReleaseMsg{X: 3, Y: 0, Button: tea.MouseRight}) {
		t.Fatal("the right button must not click")
	}
	if d.Clicked("x", tea.MouseClickMsg{X: 3, Y: 0, Button: tea.MouseLeft}) {
		t.Fatal("a press must not click")
	}
}

func TestKeepZonesPutsMarkersInTheSameCells(t *testing.T) {
	d := zoneDeps(t)
	src := "ñu " + d.Mark("x", "\x1b[31mCD\x1b[m") + "ef\nab" + d.Mark("y", "Z")
	dst := "\x1b[1mñ\x1b[2mu CDef\x1b[m\n\x1b[7mabZ\x1b[m"
	got := KeepZones(src, dst)
	if ansi.Strip(got) != ansi.Strip(dst) {
		t.Fatalf("text changed: %q", ansi.Strip(got))
	}
	z := zonetest.Zone(t, d.Zones, got, "x")
	if z.StartX != 3 || z.EndX != 4 {
		t.Fatalf("zone x = %+v", *z)
	}
	if y := d.Zones.Get("y"); y == nil || y.StartX != 2 || y.EndX != 2 || y.StartY != 1 {
		t.Fatalf("zone y = %+v", y)
	}
}

func TestHighlightKeepsZones(t *testing.T) {
	d := zoneDeps(t)
	out := Highlight(d.Theme, "ab "+d.Mark("x", "CD"), 20, 0)
	if w := ansi.StringWidth(out); w != 20 {
		t.Fatalf("width = %d, want 20", w)
	}
	if z := zonetest.Zone(t, d.Zones, out, "x"); z.StartX != 3 || z.EndX != 4 {
		t.Fatalf("zone = %+v", *z)
	}
}

func TestListClickSelectsThenChooses(t *testing.T) {
	d := zoneDeps(t)
	var l List
	l.SetZones(d, "list")
	l.SetLen(5)
	l.SetHeight(5)
	view := func() string {
		return l.View(d.Styles, d.Theme, 30, 0, func(i, w int) string { return fmt.Sprintf("row %d", i) })
	}
	for _, line := range strings.Split(view(), "\n") {
		if w := ansi.StringWidth(line); w != 30 {
			t.Fatalf("row width %d, want 30", w)
		}
	}
	click := zonetest.Click(t, d.Zones, view(), l.RowZone(3))
	click.X += 20
	if l.Update(click) {
		t.Fatal("the first click must only select")
	}
	if l.Cursor() != 3 {
		t.Fatalf("cursor = %d, want 3", l.Cursor())
	}
	if !l.Update(zonetest.Click(t, d.Zones, view(), l.RowZone(3))) {
		t.Fatal("a click on the selected row must choose it")
	}
	l.Reset()
	if l.RowZone(1) != "list.1" {
		t.Fatalf("Reset dropped the zone: %q", l.RowZone(1))
	}
}

func TestListWithoutZonesKeepsItsRows(t *testing.T) {
	var l List
	l.SetLen(2)
	th := theme.ByID("")
	out := l.View(NewStyles(th), th, 30, 0, func(i, w int) string { return "row" })
	if zonetest.HasMarkers(out) {
		t.Fatal("a list without zones must not mark")
	}
	if l.Update(zonetest.At(3, 1)) {
		t.Fatal("a list without zones must ignore clicks")
	}
}

func TestTableZoneCoversRowAndSubAndKeepsWidth(t *testing.T) {
	d := zoneDeps(t)
	tb := NewTable(40, Column{Title: "##", Width: 2}, Column{Title: "Foro"})
	tb.Header().Row("01", "Anuncios").Zone(d, "r1").Sub("", "Noticias").
		Row("02", "Otro "+d.Mark("in", "[R]")).Select().Zone(d, "r2")
	out := tb.Render(d.Styles)
	for _, line := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(line); w != 40 {
			t.Fatalf("line width %d, want 40: %q", w, ansi.Strip(line))
		}
	}
	z := zonetest.Zone(t, d.Zones, out, "r1")
	if z.StartX != 0 || z.EndX != 39 || z.StartY != 3 || z.EndY != 4 {
		t.Fatalf("zone r1 = %+v, want 0..39 on lines 3..4", *z)
	}
	if in := d.Zones.Get("in"); in == nil || in.StartY != 5 || in.EndX-in.StartX != 2 {
		t.Fatalf("zone in = %+v", in)
	}
}

func TestTableCellWithMarkerKeepsRowStyle(t *testing.T) {
	d := zoneDeps(t)
	tb := NewTable(40, Column{Title: "##", Width: 2}, Column{Title: "Foro"})
	out := tb.Row("02", "Otro "+d.Mark("in", "[R]")).Select().Render(d.Styles)
	if !strings.Contains(markerRe.ReplaceAllString(out, ""), d.Styles.Key.Render("Otro [R]")) {
		t.Fatal("a cell with a marker must keep the style of its row")
	}
}

func TestMarkLinksFindsTheClickedLink(t *testing.T) {
	d := zoneDeps(t)
	text := "see this [1] and\n\x1b[1mthat [2]\x1b[m or [9] and [2] again"
	out := d.MarkLinks("p.", text, 2)
	if ansi.Strip(out) != ansi.Strip(text) {
		t.Fatalf("text changed: %q", ansi.Strip(out))
	}
	z := zonetest.Zone(t, d.Zones, out, "p.2")
	if z.StartX != 5 || z.EndX != 7 || z.StartY != 1 {
		t.Fatalf("zone p.2 = %+v, want the first [2]", *z)
	}
	if n, ok := d.ClickedLink("p.", 2, zonetest.At(6, 1)); !ok || n != 2 {
		t.Fatalf("ClickedLink = %d, %v, want 2", n, ok)
	}
	if _, ok := d.ClickedLink("p.", 2, zonetest.At(0, 0)); ok {
		t.Fatal("a click out of the labels must find no link")
	}
	if d.Zones.Get("p.9") != nil {
		t.Fatal("a label over n must not get a zone")
	}
}

func TestClickedSideGivesTheHalf(t *testing.T) {
	d := zoneDeps(t)
	zonetest.Zone(t, d.Zones, "  "+d.Mark("pg", "Página 2 / 5"), "pg")
	if dir, ok := d.ClickedSide("pg", zonetest.At(3, 0)); !ok || dir != -1 {
		t.Fatalf("left half = %d, %v, want -1", dir, ok)
	}
	if dir, ok := d.ClickedSide("pg", zonetest.At(13, 0)); !ok || dir != 1 {
		t.Fatalf("right half = %d, %v, want 1", dir, ok)
	}
	if _, ok := d.ClickedSide("pg", zonetest.At(1, 0)); ok {
		t.Fatal("a click out of the zone must give no side")
	}
}

func TestMarkTextFindsPlainTextInAStyledLine(t *testing.T) {
	d := zoneDeps(t)
	line := "| \x1b[2m[\x1b[1mC\x1b[2m]\x1b[mitar  \x1b[2m[\x1b[1mR\x1b[2m]\x1b[mesponder |"
	out := d.MarkText("r", d.MarkText("c", line, "[C]itar"), "[R]esponder")
	if out == line || ansi.Strip(out) != ansi.Strip(line) {
		t.Fatalf("text changed or no markers: %q", out)
	}
	if z := zonetest.Zone(t, d.Zones, out, "r"); z.StartX != 11 || z.EndX != 21 {
		t.Fatalf("zone r = %+v", *z)
	}
	if c := d.Zones.Get("c"); c == nil || c.StartX != 2 || c.EndX != 8 {
		t.Fatalf("zone c = %+v", c)
	}
	if got := d.MarkText("x", line, "nada"); got != line {
		t.Fatal("a missing text must keep the line")
	}
}
