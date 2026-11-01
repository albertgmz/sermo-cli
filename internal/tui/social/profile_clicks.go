package social

import (
	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// click selects a clicked post, or opens it when it is the selected post.
// In an open post a click on a link label opens the link.
func (t *postsTab) click(msg tea.MouseReleaseMsg) tea.Cmd {
	if t.picker.open {
		return t.picker.update(msg)
	}
	if t.load.loading || t.load.err != nil {
		return nil
	}
	if cmd, ok := clickLink(t.d, t.zid+"link.", t.links, msg); ok && t.open >= 0 {
		return cmd
	}
	if t.d.BBS() {
		if i, ok := t.pg.click(t.d, t.zid+"row", msg, len(t.items)); ok && t.open < 0 {
			return t.toggle(i)
		}
		return nil
	}
	if t.list.Update(msg) {
		return t.toggle(t.list.Cursor())
	}
	return nil
}

// click turns the page for a click on the page counter. A click on a row
// selects it, or opens it when it is the selected row.
func (t *bookmarksTab) click(msg tea.MouseReleaseMsg) tea.Cmd {
	if t.load.loading || t.load.err != nil {
		return nil
	}
	if page, ok := pageTurn(t.d, t.zid+"page", msg, t.page); ok {
		if page < 1 || page > t.pages {
			return nil
		}
		return t.fetch(page)
	}
	if t.d.BBS() {
		if i, ok := t.pg.click(t.d, t.zid+"row", msg, len(t.items)); ok {
			return openBookmark(t.items[i])
		}
		return nil
	}
	if t.list.Update(msg) {
		return openBookmark(t.items[t.list.Cursor()])
	}
	return nil
}

// click selects a clicked row, or opens its thread when it is the
// selected row.
func (t *watchedTab) click(msg tea.MouseReleaseMsg) tea.Cmd {
	if t.load.loading || t.load.err != nil {
		return nil
	}
	var i int
	var ok bool
	if t.d.BBS() {
		i, ok = t.pg.click(t.d, t.zid+"row", msg, len(t.items))
	} else {
		ok = t.list.Update(msg)
		i = t.list.Cursor()
	}
	if !ok {
		return nil
	}
	id := t.items[i].ThreadId
	return func() tea.Msg { return ui.OpenThreadMsg{ThreadID: id} }
}
