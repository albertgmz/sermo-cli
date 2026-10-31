package social

import (
	"strconv"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/api/gen"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// placeIDs maps the kind of a place to the i18n ID of its text.
var placeIDs = map[gen.ActivityPlaceKind]string{
	gen.ActivityPlaceKindForumList:     "bbs.social.place.forumList",
	gen.ActivityPlaceKindLatest:        "bbs.social.place.latest",
	gen.ActivityPlaceKindForum:         "bbs.social.place.forum",
	gen.ActivityPlaceKindThread:        "bbs.social.place.thread",
	gen.ActivityPlaceKindMember:        "bbs.social.place.member",
	gen.ActivityPlaceKindMembers:       "bbs.social.place.members",
	gen.ActivityPlaceKindOnline:        "bbs.social.place.online",
	gen.ActivityPlaceKindConversations: "bbs.social.place.conversations",
	gen.ActivityPlaceKindSearch:        "bbs.social.place.search",
	gen.ActivityPlaceKindPage:          "bbs.social.place.page",
}

type onlineMsg struct {
	page  *api.OnlinePage
	stats *api.SiteStats
	err   error
}

// online lists the current visitors of the site. A number opens the
// profile of a member.
type online struct {
	d     *ui.Deps
	load  loader
	page  *api.OnlinePage
	stats *api.SiteStats
	list  ui.List
	pg    pager
	w, h  int
}

// NewOnline returns the list of the current visitors.
func NewOnline(d *ui.Deps) ui.Screen {
	o := &online{d: d, load: newLoader(d)}
	o.list.SetClock(d.Now)
	return o
}

func (o *online) Title() string       { return i18n.T("bbs.social.online.title") }
func (o *online) CapturesInput() bool { return false }
func (o *online) Keys() []key.Binding { return ui.ListKeys() }
func (o *online) Prompt() string      { return i18n.T("bbs.social.online.prompt") }

func (o *online) Init() tea.Cmd {
	client := o.d.API
	return tea.Batch(o.load.begin(), func() tea.Msg {
		page, err := client.OnlineList(bg)
		if err != nil {
			return onlineMsg{err: err}
		}
		stats, _ := client.SiteStats(bg)
		return onlineMsg{page: page, stats: stats}
	})
}

func (o *online) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		o.w, o.h = msg.Width, msg.Height
		o.list.SetHeight(max(o.h-2, 1))
		o.pg.per = bbsRows(o.h, 9)
	case spinner.TickMsg:
		return o, o.load.spin(msg)
	case onlineMsg:
		if cmd := o.load.done(msg.err); msg.err != nil {
			return o, cmd
		}
		o.page, o.stats = msg.page, msg.stats
		o.list.SetLen(len(o.page.Items))
	case tea.KeyPressMsg:
		return o, o.key(msg)
	}
	return o, nil
}

func (o *online) key(msg tea.KeyPressMsg) tea.Cmd {
	if o.page == nil {
		return nil
	}
	if o.d.BBS() {
		o.pg.move(msg.String(), len(o.page.Items))
		return nil
	}
	if o.list.Update(msg) {
		return o.open(o.list.Cursor())
	}
	return nil
}

// open opens the profile of the visitor i when it is a member.
func (o *online) open(i int) tea.Cmd {
	v := o.page.Items[i]
	if v.Card == nil {
		return ui.Toast(i18n.T("bbs.social.online.noProfile"), ui.ToastError)
	}
	id := v.Card.Id
	return func() tea.Msg { return ui.OpenProfileMsg{UserID: id} }
}

func (o *online) Commands() []ui.Command {
	n := 0
	if o.page != nil {
		n = len(o.page.Items)
	}
	pick := func(arg string) tea.Cmd {
		i, ok := o.pg.pick(arg, n)
		if !ok {
			return noRow()
		}
		return o.open(i)
	}
	cmds := []ui.Command{
		{Name: "#", Label: i18n.T("bbs.social.online.open"), Run: pick},
		{Name: "", Run: func(string) tea.Cmd { return pick(strconv.Itoa(o.pg.sel + 1)) }},
	}
	cmds = append(cmds, pageCommands(
		func() tea.Cmd { o.pg.prev(); return nil },
		func() tea.Cmd { o.pg.next(n, false); return nil },
	)...)
	return append(cmds, back())
}

// who returns the name of a visitor.
func who(v gen.OnlineVisitor) string {
	switch {
	case v.Card != nil:
		return v.Card.Username
	case v.Robot != nil && v.Robot.Name != nil:
		return *v.Robot.Name
	case v.Type == gen.OnlineVisitorTypeRobot:
		return i18n.T("bbs.social.online.robot")
	}
	return i18n.T("bbs.social.online.guest")
}

// where returns the text of the place of a visitor.
func where(v gen.OnlineVisitor) string {
	p := v.Activity.Place
	if p == nil {
		return "--"
	}
	id, ok := placeIDs[p.Kind]
	if !ok {
		return "--"
	}
	if p.Target != nil {
		return i18n.Tf(id+"In", map[string]any{"Title": p.Target.Title})
	}
	return i18n.T(id)
}

func (o *online) View(width, height int) string {
	if o.d.BBS() {
		return o.bbsView(width, height)
	}
	if v, ok := o.load.view(width, height); ok {
		return v
	}
	s := o.d.Styles
	head := ui.Header(s, o.d.Theme, o.Title(), width)
	if len(o.page.Items) == 0 {
		return head + "\n" + ui.Empty(s, i18n.T("bbs.social.online.empty"), width, height-1)
	}
	rows := o.list.View(s, o.d.Theme, width, o.d.Phase(), func(i, _ int) string {
		v := o.page.Items[i]
		return s.Text.Render(who(v)) + s.Muted.Render("  ·  "+relTime(o.d, v.Activity.At)+"  ·  "+where(v))
	})
	return head + "\n\n" + rows
}

func (o *online) bbsView(width, height int) string {
	if v, ok := o.load.bbsView(width, height); ok {
		return v
	}
	s := o.d.Styles
	items := o.page.Items
	o.pg.clamp(len(items))
	t := ui.NewTable(width,
		ui.Column{Title: i18n.T("bbs.help.number"), Width: 2, Align: ui.AlignRight},
		ui.Column{Title: i18n.T("bbs.social.online.user"), Width: 20},
		ui.Column{Title: i18n.T("bbs.social.online.seen"), Width: 10},
		ui.Column{Title: i18n.T("bbs.social.online.where")},
	)
	title := i18n.Tf("bbs.social.online.heading", map[string]any{"Count": ui.Thousands(o.page.Total)})
	t.Full(s.Subtitle.Render(title)).Header()
	lo, hi := o.pg.window(len(items))
	for i := lo; i < hi; i++ {
		v := items[i]
		t.Row(ui.Num(i-lo+1, hi-lo), who(v), bbsAgo(o.d, v.Activity.At), where(v))
		if i-lo == o.pg.sel {
			t.Select()
		}
	}
	if len(items) == 0 {
		t.Full(i18n.T("bbs.social.online.empty"))
	}
	if o.stats != nil {
		t.Full(i18n.Tf("bbs.social.online.stats", map[string]any{
			"Members": ui.Thousands(o.stats.Members), "Threads": ui.Thousands(o.stats.Threads),
			"Posts": ui.Thousands(o.stats.Posts),
		}))
	}
	return t.Render(s)
}
