package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/forum"
	"github.com/albertgmz/sermo-cli/internal/tui/social"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// updateNav handles the stack messages of the screens.
func (m *model) updateNav(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case ui.PushMsg:
		return m.push(msg.Screen), true
	case ui.PopMsg:
		return m.pop(), true
	case ui.ReplaceMsg:
		if len(m.stack) == 1 {
			m.stack[0] = msg.Screen
			return m.started(msg.Screen), true
		}
		m.stack = m.stack[:len(m.stack)-1]
		return m.push(msg.Screen), true
	case ui.LoginMsg:
		return m.showLogin(i18n.T("core.login.membersOnly")), true
	}
	return nil, false
}

// updateIntent turns the intent messages of the screens into screens.
func (m *model) updateIntent(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case ui.OpenNodeMsg:
		return m.push(forum.NewNode(m.d, msg.NodeID)), true
	case ui.OpenThreadMsg:
		return m.push(forum.NewThread(m.d, msg.ThreadID, msg.PostID)), true
	case ui.OpenProfileMsg:
		return m.push(social.NewProfile(m.d, msg.UserID)), true
	case ui.OpenConversationMsg:
		return m.members(func() ui.Screen { return social.NewConversation(m.d, msg.ID) }, false), true
	case ui.ComposeMsg:
		return m.members(func() ui.Screen { return forum.NewComposer(m.d, msg) }, false), true
	}
	return nil, false
}

// hotkey runs a global hotkey. The sections replace all screens above home.
func (m *model) hotkey(k string) tea.Cmd {
	switch k {
	case "f":
		m.stack = m.stack[:1]
	case "m":
		return m.members(func() ui.Screen { return social.NewConversations(m.d) }, true)
	case "n":
		return m.members(func() ui.Screen { return social.NewNotifications(m.d) }, true)
	case "p":
		return m.members(func() ui.Screen { return social.NewProfile(m.d, m.d.Me.Id) }, true)
	case "b":
		m.stack = m.stack[:1]
		return m.push(social.NewSearch(m.d))
	case "?":
		if _, ok := m.top().(*helpScreen); !ok {
			return m.push(newHelp(m.d, m.top().Keys()))
		}
	case "q":
		if len(m.stack) == 1 {
			return tea.Quit
		}
		m.confirmQuit = true
	}
	return nil
}

// members shows the screen of mk for a member. A guest gets the sign-in
// screen. With section true the screen replaces all screens above home.
func (m *model) members(mk func() ui.Screen, section bool) tea.Cmd {
	if m.d.Me == nil {
		return m.showLogin(i18n.T("core.login.membersOnly"))
	}
	if section {
		m.stack = m.stack[:1]
	}
	return m.push(mk())
}

func (m *model) showLogin(reason string) tea.Cmd {
	if _, ok := m.top().(*login); ok {
		return nil
	}
	return m.push(newLogin(m.ctx, m.d, reason, m.opts.Sleep))
}

func (m *model) push(s ui.Screen) tea.Cmd {
	m.stack = append(m.stack, s)
	return m.started(s)
}

// started gives the new top screen s the content size and runs its Init.
func (m *model) started(s ui.Screen) tea.Cmd {
	cw, ch := m.contentSize()
	s, cmd := s.Update(tea.WindowSizeMsg{Width: cw, Height: ch})
	m.stack[len(m.stack)-1] = s
	return tea.Batch(cmd, s.Init())
}

func (m *model) pop() tea.Cmd {
	if len(m.stack) > 1 {
		m.stack = m.stack[:len(m.stack)-1]
	}
	return nil
}
