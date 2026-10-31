package social

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// profileTab is one tab of the profile screen. Each tab loads its data
// the first time that it opens.
type profileTab interface {
	name() string
	start() tea.Cmd
	update(msg tea.Msg) tea.Cmd
	key(msg tea.KeyPressMsg) tea.Cmd
	view(width, height int) string
}

type profileMsg struct {
	p   *api.Profile
	err error
}

// profile shows the facts of a member and the tabs: the profile posts,
// and on the own profile also the bookmarks and the watched threads.
type profile struct {
	d       *ui.Deps
	userID  int
	load    loader
	p       *api.Profile
	tabs    []profileTab
	started []bool
	tab     int
	posts   *postsTab
	w, h    int
}

// NewProfile returns the profile of the user userID. A userID of 0 is the
// signed-in member.
func NewProfile(d *ui.Deps, userID int) ui.Screen {
	pr := &profile{d: d, userID: userID, load: newLoader(d)}
	if userID == 0 && d.Me != nil {
		pr.userID = d.Me.Id
	}
	pr.posts = newPostsTab(d, pr.userID)
	pr.tabs = []profileTab{pr.posts}
	if d.Me != nil && pr.userID == d.Me.Id {
		pr.tabs = append(pr.tabs, newBookmarksTab(d), newWatchedTab(d))
	}
	pr.started = make([]bool, len(pr.tabs))
	return pr
}

func (pr *profile) Title() string {
	if pr.p != nil {
		return pr.p.Username
	}
	return i18n.T("core.hotkey.profile")
}

func (pr *profile) CapturesInput() bool { return pr.posts.editing() }

func (pr *profile) Keys() []key.Binding {
	if pr.posts.ed != nil {
		return editorKeys()
	}
	return append(ui.ListKeys(),
		binding("Tab 1 2 3", "social.key.tabs", "tab", "shift+tab", "1", "2", "3"),
		binding("W", "social.key.writeProfile", "w"),
		binding("C", "social.key.comment", "c"),
		binding("O", "social.key.links", "o"),
		binding("h l", "social.key.pages", "h", "l"),
		binding("X", "social.key.unwatch", "x"),
	)
}

func (pr *profile) Init() tea.Cmd {
	if pr.userID == 0 {
		return login
	}
	client, id := pr.d.API, pr.userID
	return tea.Batch(pr.load.begin(), func() tea.Msg {
		p, err := client.Profile(bg, id)
		return profileMsg{p: p, err: err}
	}, pr.open(0))
}

// open makes the tab i active and starts it the first time.
func (pr *profile) open(i int) tea.Cmd {
	pr.tab = i
	if pr.started[i] {
		return nil
	}
	pr.started[i] = true
	return pr.tabs[i].start()
}

func (pr *profile) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	if pr.posts.editing() && forEditor(msg) {
		return pr, pr.posts.update(msg)
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		pr.w, pr.h = msg.Width, msg.Height
		pr.posts.resize(msg.Width, msg.Height)
		return pr, pr.setRows(msg.Height)
	case profileMsg:
		pr.p = msg.p
		pr.posts.canPost = msg.p != nil && msg.p.CanPostOnWall
		if msg.p != nil {
			pr.posts.owner = msg.p.Username
		}
		return pr, pr.load.done(msg.err)
	case spinner.TickMsg:
		return pr, tea.Batch(pr.load.spin(msg), pr.forward(msg))
	case tea.KeyPressMsg:
		return pr, pr.key(msg)
	}
	return pr, pr.forward(msg)
}

// forward gives a data message to all tabs. Each tab takes only its own.
func (pr *profile) forward(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(pr.tabs))
	for _, t := range pr.tabs {
		cmds = append(cmds, t.update(msg))
	}
	return tea.Batch(cmds...)
}

func (pr *profile) key(msg tea.KeyPressMsg) tea.Cmd {
	if cmd, stop := pr.load.blocked(msg, func() tea.Cmd {
		pr.started = make([]bool, len(pr.tabs))
		return pr.Init()
	}); stop {
		return cmd
	}
	if pr.d.BBS() {
		return pr.bbsKey(msg.String())
	}
	n := len(pr.tabs)
	switch k := msg.String(); k {
	case "tab":
		return pr.open((pr.tab + 1) % n)
	case "shift+tab":
		return pr.open((pr.tab + n - 1) % n)
	case "1", "2", "3":
		if i := int(k[0] - '1'); i < n {
			return pr.open(i)
		}
		return nil
	}
	return pr.tabs[pr.tab].key(msg)
}

func (pr *profile) View(width, height int) string {
	if pr.posts.ed != nil {
		return pr.posts.ed.view()
	}
	if pr.posts.led != nil {
		return pr.posts.led.view()
	}
	if pr.d.BBS() {
		return pr.bbsView(width, height)
	}
	if v, ok := pr.load.view(width, height); ok {
		return v
	}
	top := pr.facts(width)
	used := lipgloss.Height(top) + 2
	return top + "\n" + pr.tabBar() + "\n\n" + pr.tabs[pr.tab].view(width, max(height-used-1, 1))
}

// facts returns the header, the name, the facts and the about text.
func (pr *profile) facts(width int) string {
	s, p := pr.d.Styles, pr.p
	title := p.Card.Title
	if p.CustomTitle != nil && *p.CustomTitle != "" {
		title = *p.CustomTitle
	}
	name := userName(pr.d, p.Username, p.Card.NameColor) + "  " + s.Muted.Render(title)
	facts := []string{
		i18n.Tf("social.profile.joined", map[string]any{"Date": date(p.CreatedAt)}),
		i18n.Plural("social.profile.messages", p.PostCount, nil),
		i18n.Plural("social.profile.reactions", p.ReactionScore, nil),
	}
	if p.Location != "" {
		facts = append(facts, p.Location)
	}
	lines := []string{
		ui.Header(s, pr.d.Theme, i18n.T("core.hotkey.profile"), width),
		name,
		s.Muted.Render(lipgloss.Wrap(strings.Join(facts, " · "), width, " ")),
	}
	if about := strings.TrimSpace(p.About); about != "" {
		wrapped := strings.Split(lipgloss.Wrap(about, ui.ReadingWidth(width), " "), "\n")
		lines = append(lines, "", s.Text.Render(strings.Join(firstLines(wrapped, 4), "\n")))
	}
	return strings.Join(lines, "\n")
}

func (pr *profile) tabBar() string {
	s := pr.d.Styles
	parts := make([]string, len(pr.tabs))
	for i, t := range pr.tabs {
		label := string(rune('1'+i)) + " " + t.name()
		if i == pr.tab {
			parts[i] = s.Badge.Render(label)
			continue
		}
		parts[i] = s.Muted.Render(" " + label + " ")
	}
	return strings.Join(parts, " ")
}
