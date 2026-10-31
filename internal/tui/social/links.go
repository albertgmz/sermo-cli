package social

import (
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/render"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// linkPicker is a small list of the numbered links of one text. A digit
// selects the link with that number and Enter opens it.
type linkPicker struct {
	links []render.Link
	list  ui.List
	open  bool
}

// start opens the picker for links. One link opens at once.
func (p *linkPicker) start(links []render.Link) tea.Cmd {
	switch len(links) {
	case 0:
		return ui.Toast(i18n.T("social.links.none"), ui.ToastInfo)
	case 1:
		return ui.Open(links[0].URL)
	}
	p.links, p.open = links, true
	p.list = ui.List{}
	p.list.SetLen(len(links))
	p.list.SetHeight(min(len(links), 8))
	return nil
}

func (p *linkPicker) update(msg tea.Msg) tea.Cmd {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		p.list.Update(msg)
		return nil
	}
	if k.String() == "esc" {
		p.open = false
		return nil
	}
	if n, err := strconv.Atoi(k.String()); err == nil {
		p.list.Select(n - 1)
		return nil
	}
	if p.list.Update(k) {
		p.open = false
		return ui.Open(p.links[p.list.Cursor()].URL)
	}
	return nil
}

func (p *linkPicker) view(d *ui.Deps, width int) string {
	rows := p.list.View(d.Styles, d.Theme, width, d.Phase(), func(i, _ int) string {
		l := p.links[i]
		label := l.Label
		if label == "" || label == l.URL {
			return "[" + strconv.Itoa(l.N) + "] " + l.URL
		}
		return "[" + strconv.Itoa(l.N) + "] " + label + " · " + l.URL
	})
	return ui.Header(d.Styles, d.Theme, i18n.T("social.links.title"), width) + "\n" + rows
}
