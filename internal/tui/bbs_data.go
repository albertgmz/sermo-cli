package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/api"
)

// onlineInterval is the shortest time between two loads of the visitor
// count.
const onlineInterval = 60 * time.Second

type onlineMsg struct{ total int }

type statsMsg struct{ stats *api.SiteStats }

type groupMsg struct {
	userID int
	title  string
}

// updateBBS handles the data of the BBS chrome and splash.
func (m *model) updateBBS(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case onlineMsg:
		m.online, m.haveOnline = msg.total, true
		return nil, true
	case statsMsg:
		m.stats = msg.stats
		return nil, true
	case groupMsg:
		if m.d.Me != nil && m.d.Me.Id == msg.userID {
			m.group = msg.title
		}
		return nil, true
	case tea.PasteMsg:
		return nil, m.paste(msg)
	}
	return nil, false
}

// bbsData loads the BBS data that is missing: the site totals once, the
// visitor count when it is old, and the group of the member.
func (m *model) bbsData() tea.Cmd {
	if !m.d.BBS() {
		return nil
	}
	cmds := []tea.Cmd{m.loadOnline(), m.loadGroup()}
	if !m.statsAsked {
		m.statsAsked = true
		c, ctx := m.d.API, m.ctx
		cmds = append(cmds, func() tea.Msg {
			st, err := c.SiteStats(ctx)
			if err != nil {
				return nil
			}
			return statsMsg{stats: st}
		})
	}
	return tea.Batch(cmds...)
}

// loadOnline loads the visitor count when the last load is older than
// onlineInterval.
func (m *model) loadOnline() tea.Cmd {
	now := m.d.Now()
	if !m.d.BBS() || (!m.onlineAt.IsZero() && now.Sub(m.onlineAt) < onlineInterval) {
		return nil
	}
	m.onlineAt = now
	c, ctx := m.d.API, m.ctx
	return func() tea.Msg {
		page, err := c.OnlineList(ctx)
		if err != nil {
			return nil
		}
		return onlineMsg{total: page.Total}
	}
}

// loadGroup loads the group title of the member for the status box.
func (m *model) loadGroup() tea.Cmd {
	if !m.d.BBS() || m.d.Me == nil || m.group != "" {
		return nil
	}
	c, ctx, id := m.d.API, m.ctx, m.d.Me.Id
	return func() tea.Msg {
		card, err := c.AuthorCard(ctx, id)
		if err != nil || card == nil {
			return nil
		}
		return groupMsg{userID: id, title: strings.ToUpper(card.GroupTitle)}
	}
}
