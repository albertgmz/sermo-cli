package social

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// profileFrame is the number of lines of the BBS profile around the rows
// of the table: the card and the frame of the table.
const profileFrame = 13

// bbsTab is a profile tab in the BBS interface.
type bbsTab interface {
	bbsCommands() []ui.Command
	bbsView(width, height int) string
	bbsKey(k string)
}

// rowLimit is the number of items in one request: the rows of the pager
// in the BBS interface, else pageSize.
func rowLimit(d *ui.Deps, p pager) int {
	if d.BBS() && p.per > 0 {
		return p.per
	}
	return pageSize
}

// setRows sets the rows of the BBS tables of the lists for a content
// area of height lines. The Cmd loads the bookmarks again when their page
// size changed.
func (pr *profile) setRows(height int) tea.Cmd {
	var cmd tea.Cmd
	for _, t := range pr.tabs {
		switch t := t.(type) {
		case *bookmarksTab:
			t.pg.per = bbsRows(height, profileFrame)
			cmd = t.refit()
		case *watchedTab:
			t.pg.per = bbsRows(height, profileFrame)
		}
	}
	return cmd
}

// refit loads the page that shows the first shown bookmark when the page
// size changed, for example after a resize.
func (t *bookmarksTab) refit() tea.Cmd {
	limit := t.limit()
	if !t.d.BBS() || t.load.loading || t.shown == 0 || t.shown == limit {
		return nil
	}
	return t.fetch(refitPage(t.page, t.shown, limit))
}

// own reports whether the profile is the profile of the viewer.
func (pr *profile) own() bool { return len(pr.tabs) > 1 }

// Prompt is the label of the BBS prompt: the username.
func (pr *profile) Prompt() string {
	if pr.posts.led != nil {
		return i18n.T("bbs.social.editor.prompt")
	}
	return pr.Title()
}

// Commands are the BBS commands of the profile and of its open tab.
func (pr *profile) Commands() []ui.Command {
	if pr.posts.led != nil {
		return editorHints(&pr.posts.led)
	}
	if pr.p == nil {
		return []ui.Command{back()}
	}
	var cmds, pages []ui.Command
	hasBack := false
	for _, c := range pr.tabs[pr.tab].(bbsTab).bbsCommands() {
		hasBack = hasBack || c.Name == "V"
		if c.Name == "<" || c.Name == ">" {
			pages = append(pages, c)
			continue
		}
		cmds = append(cmds, c)
	}
	if pr.own() {
		cmds = append(cmds,
			ui.Command{Name: "F", Label: i18n.T("bbs.social.profile.bookmarks"), Run: func(string) tea.Cmd { return pr.toggle(1) }},
			ui.Command{Name: "S", Label: i18n.T("bbs.social.profile.watched"), Run: func(string) tea.Cmd { return pr.toggle(2) }},
		)
	}
	cmds = append(cmds, pages...)
	if hasBack {
		return cmds
	}
	return append(cmds, back())
}

// toggle opens the tab i, or the wall when the tab i is open.
func (pr *profile) toggle(i int) tea.Cmd {
	if pr.tab == i {
		return pr.open(0)
	}
	return pr.open(i)
}

func (pr *profile) bbsKey(k string) tea.Cmd {
	pr.tabs[pr.tab].(bbsTab).bbsKey(k)
	return nil
}

// groupTitle is the title of the display group, else the group title.
func groupTitle(p *api.Profile) string {
	if g := p.DisplayGroup; g != nil {
		if g.UserTitle != "" {
			return g.UserTitle
		}
		if g.Title != "" {
			return g.Title
		}
	}
	return p.GroupTitle
}

// card returns the facts of the member in a box of = lines.
func (pr *profile) card(width int) string {
	s, p := pr.d.Styles, pr.p
	title := p.Card.Title
	if p.CustomTitle != nil && *p.CustomTitle != "" {
		title = *p.CustomTitle
	}
	lines := []string{
		userName(pr.d, p.Username, p.Card.NameColor) + s.Muted.Render("  [") + s.Accent.Render(title) + s.Muted.Render("]"),
		i18n.Tf("bbs.social.profile.group", map[string]any{"Group": groupTitle(p)}),
		i18n.Tf("bbs.social.profile.facts", map[string]any{
			"Date": bbsDay(pr.d, p.CreatedAt), "Posts": ui.Thousands(p.PostCount), "Score": ui.Thousands(p.ReactionScore),
		}),
	}
	if p.Location != "" {
		lines = append(lines, i18n.Tf("bbs.social.profile.location", map[string]any{"Place": p.Location}))
	}
	return ui.DoubleBox(s, width, lines)
}

func (pr *profile) bbsView(width, height int) string {
	if v, ok := pr.load.bbsView(width, height); ok {
		return v
	}
	card := pr.card(width)
	used := strings.Count(card, "\n") + 1
	return card + "\n" + pr.tabs[pr.tab].(bbsTab).bbsView(width, max(height-used, 1))
}

// numberCommand returns the command "#" with the label id that runs
// open with the list index of the row number.
func numberCommand(id string, p *pager, n int, open func(i int) tea.Cmd) ui.Command {
	return ui.Command{Name: "#", Label: i18n.T(id), Run: func(arg string) tea.Cmd {
		i, ok := p.pick(arg, n)
		if !ok {
			return noRow()
		}
		return open(i)
	}}
}
