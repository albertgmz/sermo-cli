package social

import (
	"regexp"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/render"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// rowZone is the zone ID of the shown row k of a BBS table.
func rowZone(id string, k int) string { return id + "." + strconv.Itoa(k) }

// click finds the shown row that msg clicks. The zone of the shown row k
// is rowZone(id, k). A click on another row selects it and returns false.
// A click on the selected row returns its list index and true.
func (p *pager) click(d *ui.Deps, id string, msg tea.Msg, n int) (int, bool) {
	lo, hi := p.window(n)
	for k := range hi - lo {
		if !d.Clicked(rowZone(id, k), msg) {
			continue
		}
		if k == p.sel {
			return lo + k, true
		}
		p.sel = k
		return 0, false
	}
	return 0, false
}

// linkURL returns the URL of the link with the number n.
func linkURL(links []render.Link, n int) (string, bool) {
	for _, l := range links {
		if l.N == n {
			return l.URL, true
		}
	}
	return "", false
}

// clickLink opens the link of links that msg clicks. The link labels are
// in the zones of d.MarkLinks with prefix.
func clickLink(d *ui.Deps, prefix string, links []render.Link, msg tea.Msg) (tea.Cmd, bool) {
	k, ok := d.ClickedLink(prefix, len(links), msg)
	if !ok {
		return nil, false
	}
	url, ok := linkURL(links, k)
	if !ok {
		return nil, false
	}
	return ui.Open(url), true
}

// zonesOn reports whether the views of d mark their zones.
func zonesOn(d *ui.Deps) bool { return d.Zones != nil && d.Zones.Enabled() }

// pageTurn returns the page next to page for a click on the page counter
// in the zone id: the left half gives the page before and the right half
// the page after.
func pageTurn(d *ui.Deps, id string, msg tea.Msg, page int) (int, bool) {
	side, ok := d.ClickedSide(id, msg)
	return page + side, ok
}

// actionRe matches one action like "[C]itar" or "[C] Quote".
var actionRe = regexp.MustCompile(`\[([^\]]+)\][^\[]*`)

// actionParts splits a text of actions like "[C]itar  [R]esponder" into
// its actions. Each action starts with a key in brackets.
func actionParts(text string) []hintAction {
	var out []hintAction
	for _, m := range actionRe.FindAllStringSubmatch(text, -1) {
		out = append(out, hintAction{key: m[1], text: strings.TrimSpace(m[0])})
	}
	return out
}

// hintAction is one action of a text of actions.
type hintAction struct {
	key, text string
}
