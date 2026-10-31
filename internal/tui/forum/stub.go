// Package forum holds the screens for forum nodes, thread lists, threads and
// the composer.
package forum

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// NewNodes returns the screen with the forum tree. It is the home screen.
func NewNodes(d *ui.Deps) ui.Screen {
	return &placeholder{d: d, title: i18n.T("core.hotkey.forums")}
}

// NewNode returns the thread list of the node nodeID.
func NewNode(d *ui.Deps, nodeID int) ui.Screen {
	return &placeholder{d: d, title: i18n.Tf("core.stub.node", map[string]any{"ID": nodeID})}
}

// NewThread returns the reader for the thread threadID. A postID of 0 opens
// the first unread post.
func NewThread(d *ui.Deps, threadID, postID int) ui.Screen {
	return &placeholder{d: d, title: i18n.Tf("core.stub.thread", map[string]any{"ID": threadID})}
}

// NewComposer returns the composer for the target in m.
func NewComposer(d *ui.Deps, m ui.ComposeMsg) ui.Screen {
	return &placeholder{d: d, title: i18n.T("core.stub.composer")}
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
