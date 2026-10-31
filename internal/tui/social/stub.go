// Package social holds the screens for notifications, conversations,
// profiles and search.
package social

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// NewNotifications returns the notification list of the viewer.
func NewNotifications(d *ui.Deps) ui.Screen {
	return &placeholder{d: d, title: i18n.T("core.hotkey.notifications")}
}

// NewConversations returns the conversation list of the viewer.
func NewConversations(d *ui.Deps) ui.Screen {
	return &placeholder{d: d, title: i18n.T("core.hotkey.messages")}
}

// NewConversation returns the conversation id.
func NewConversation(d *ui.Deps, id int) ui.Screen {
	return &placeholder{d: d, title: i18n.Tf("core.stub.conversation", map[string]any{"ID": id})}
}

// NewProfile returns the profile of the user userID.
func NewProfile(d *ui.Deps, userID int) ui.Screen {
	return &placeholder{d: d, title: i18n.Tf("core.stub.profile", map[string]any{"ID": userID})}
}

// NewSearch returns the search screen.
func NewSearch(d *ui.Deps) ui.Screen {
	return &placeholder{d: d, title: i18n.T("core.hotkey.search")}
}

type placeholder struct {
	d     *ui.Deps
	title string
}

func (p *placeholder) Init() tea.Cmd                       { return nil }
func (p *placeholder) Update(tea.Msg) (ui.Screen, tea.Cmd) { return p, nil }
func (p *placeholder) Title() string                       { return p.title }
func (p *placeholder) Keys() []key.Binding                 { return nil }
func (p *placeholder) CapturesInput() bool                 { return false }
func (p *placeholder) View(width, height int) string {
	return ui.Empty(p.d.Styles, i18n.T("core.stub.text"), width, height)
}
