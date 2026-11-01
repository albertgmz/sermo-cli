// Package zonetest helps the tests of the screens to click the zones of a
// view.
package zonetest

import (
	"regexp"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	zone "github.com/lrstanley/bubblezone/v2"
)

// wait is the maximum time to wait for the zones of a scan.
const wait = 2 * time.Second

var marker = regexp.MustCompile(`\x1b\[[0-9]+z`)

// HasMarkers reports whether view has zone markers.
func HasMarkers(view string) bool { return marker.MatchString(view) }

// Zone scans view with m and returns the zone id of that scan. The test
// fails when the view has no zone id.
func Zone(t testing.TB, m *zone.Manager, view, id string) *zone.ZoneInfo {
	t.Helper()
	before := m.Get(id)
	m.Scan(view)
	end := time.Now().Add(wait)
	for {
		if z := m.Get(id); z != nil && z != before {
			return z
		}
		if time.Now().After(end) {
			t.Fatalf("the view has no zone %q", id)
		}
		time.Sleep(time.Millisecond)
	}
}

// Click scans view with m and returns a left click on the top left cell
// of the zone id.
func Click(t testing.TB, m *zone.Manager, view, id string) tea.MouseReleaseMsg {
	t.Helper()
	z := Zone(t, m, view, id)
	return At(z.StartX, z.StartY)
}

// At returns a left click on the cell x, y.
func At(x, y int) tea.MouseReleaseMsg {
	return tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft}
}
