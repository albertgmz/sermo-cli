package forum

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// picker is a small list to choose one item, for example a reaction or a
// link. The digits 1 to 9 choose an item at once.
type picker struct {
	title string
	items []string
	list  ui.List
}

func newPicker(title string, items []string, cursor int) *picker {
	p := &picker{title: title, items: items}
	p.list.SetLen(len(items))
	p.list.SetHeight(len(items))
	p.list.Select(cursor)
	return p
}

// update handles msg. It returns the chosen index, or -1 when the user
// closes the picker. done is false while the picker stays open.
func (p *picker) update(msg tea.Msg) (chosen int, done bool) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		s := k.String()
		if s == "esc" {
			return -1, true
		}
		if n, err := strconv.Atoi(s); err == nil && n >= 1 && n <= len(p.items) {
			return n - 1, true
		}
	}
	if p.list.Update(msg) {
		return p.list.Cursor(), true
	}
	return -1, false
}

func (p *picker) view(d *ui.Deps, width, height int) string {
	p.list.SetHeight(max(height-4, 1))
	rows := p.list.View(d.Styles, d.Theme, width, d.Phase(), func(i, w int) string {
		return d.Styles.Muted.Render(strconv.Itoa(i+1)+". ") + p.items[i]
	})
	return strings.Join([]string{
		ui.Header(d.Styles, d.Theme, p.title, width), "", rows, "",
		d.Styles.Muted.Render(i18n.T("forum.pick.hint")),
	}, "\n")
}
