package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/auth"
	"github.com/albertgmz/sermo-cli/internal/buildinfo"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/forum"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// pollInterval is the time between two counter requests.
const pollInterval = 60 * time.Second

type siteMsg struct{ name string }

type meMsg struct {
	me      *api.Me
	err     error
	welcome bool
}

type countersMsg struct {
	c    *api.Counters
	err  error
	poll bool
}

type pollMsg struct{ gen int }

type clockMsg struct{}

// updateSession handles the site, the member, the counters and the sign-in
// state.
func (m *model) updateSession(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case siteMsg:
		m.site = msg.name
		return nil, true
	case meMsg:
		return m.gotMe(msg), true
	case countersMsg:
		return m.gotCounters(msg), true
	case pollMsg:
		if msg.gen != m.pollGen || m.d.Cred.Token == "" {
			return nil, true
		}
		return tea.Batch(m.loadCounters(true), m.nextPoll()), true
	case ui.CountersChangedMsg:
		return m.reloadCounters(), true
	case ui.UnauthorizedMsg:
		return m.unauthorized(), true
	case loggedInMsg:
		return m.loggedIn(msg), true
	}
	return nil, false
}

func (m *model) loadSite() tea.Cmd {
	c, ctx := m.d.API, m.ctx
	return func() tea.Msg {
		site, err := c.Site(ctx)
		if err != nil || site.Name == "" {
			return nil
		}
		return siteMsg{name: site.Name}
	}
}

func (m *model) loadMe(welcome bool) tea.Cmd {
	c, ctx := m.d.API, m.ctx
	return func() tea.Msg {
		me, err := c.Me(ctx)
		return meMsg{me: me, err: err, welcome: welcome}
	}
}

func (m *model) loadCounters(poll bool) tea.Cmd {
	c, ctx := m.d.API, m.ctx
	return func() tea.Msg {
		n, err := c.Counters(ctx)
		return countersMsg{c: n, err: err, poll: poll}
	}
}

// reloadCounters loads the counters as a new baseline. The change shows no
// toast.
func (m *model) reloadCounters() tea.Cmd {
	if m.d.Cred.Token == "" {
		return nil
	}
	return m.loadCounters(false)
}

func (m *model) nextPoll() tea.Cmd {
	gen := m.pollGen
	return tea.Tick(pollInterval, func(time.Time) tea.Msg { return pollMsg{gen: gen} })
}

// nextClock wakes the app at the next minute so that the clock changes.
func (m *model) nextClock() tea.Cmd {
	now := m.d.Now()
	wait := now.Truncate(time.Minute).Add(time.Minute).Sub(now)
	return tea.Tick(wait, func(time.Time) tea.Msg { return clockMsg{} })
}

func (m *model) gotMe(msg meMsg) tea.Cmd {
	if m.d.Cred.Token == "" {
		return nil
	}
	if api.IsUnauthorized(msg.err) || (msg.err == nil && msg.me.User == nil) {
		return m.unauthorized()
	}
	if msg.err != nil {
		return ui.ErrorCmd(msg.err)
	}
	m.d.Me = msg.me.User
	if msg.welcome {
		return ui.Toast(i18n.Tf("core.toast.welcome", map[string]any{"Name": m.d.Me.Username}), ui.ToastSuccess)
	}
	return nil
}

func (m *model) gotCounters(msg countersMsg) tea.Cmd {
	if m.d.Cred.Token == "" {
		return nil
	}
	if api.IsUnauthorized(msg.err) {
		return m.unauthorized()
	}
	if msg.err != nil {
		return nil
	}
	prev, known := m.counters.Notifications, m.haveCounters
	m.counters, m.haveCounters = *msg.c, true
	if msg.poll && known && msg.c.Notifications > prev {
		n := msg.c.Notifications - prev
		return ui.Toast(i18n.Plural("core.toast.newNotifications", n, nil), ui.ToastInfo)
	}
	return nil
}

// unauthorized drops the credential that the server refused. It deletes a
// stored device token, continues as a guest and shows the sign-in screen.
func (m *model) unauthorized() tea.Cmd {
	reason := i18n.T("core.login.membersOnly")
	if m.d.Cred.Token != "" {
		reason = i18n.T("core.login.sessionExpired")
		if m.d.Cred.Source == auth.SourceDevice {
			_ = m.d.Store.Delete(m.d.Server)
		}
		// A guest client needs no token, so New fails only for a bad server URL.
		_ = m.signIn(auth.Credential{})
	}
	return m.showLogin(reason)
}

func (m *model) loggedIn(msg loggedInMsg) tea.Cmd {
	err := m.signIn(auth.Credential{Token: msg.token, Source: auth.SourceDevice})
	var resumed tea.Cmd
	if _, ok := m.top().(*login); ok {
		resumed = m.pop()
	}
	if err != nil {
		return tea.Batch(resumed, ui.ErrorCmd(err))
	}
	cmds := []tea.Cmd{resumed, m.loadMe(msg.saveErr == nil), m.loadCounters(false), m.nextPoll()}
	if len(m.stack) == 1 {
		// Home loaded as a guest. Load it again for the unread markers.
		m.stack = m.stack[:0]
		cmds = append(cmds, m.push(forum.NewNodes(m.d)))
	}
	if msg.saveErr != nil {
		cmds = append(cmds, ui.Toast(i18n.T("core.toast.tokenSaveFailed"), ui.ToastError))
	}
	return tea.Batch(cmds...)
}

// signIn sets the credential and makes a new API client for it. The member
// and the counters reset until the next load.
func (m *model) signIn(c auth.Credential) error {
	client, err := api.New(api.Options{Server: m.d.Server, Token: c.Token, UserAgent: buildinfo.UserAgent()})
	if err != nil {
		return err
	}
	m.d.API, m.d.Cred, m.d.Me = client, c, nil
	m.counters, m.haveCounters = api.Counters{}, false
	m.pollGen++
	return nil
}
