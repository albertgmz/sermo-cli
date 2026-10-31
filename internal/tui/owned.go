package tui

import (
	"reflect"

	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// ownedMsg is the result of a Cmd of a screen. The app gives msg to owner,
// also when owner is not the top screen.
type ownedMsg struct {
	owner ui.Screen
	msg   tea.Msg
}

// runtimePkg is the package of the Bubble Tea messages that the program
// handles itself, for example tea.QuitMsg.
var runtimePkg = reflect.TypeFor[tea.QuitMsg]().PkgPath()

// wrap returns a Cmd that runs cmd and marks its result with owner. A nil
// cmd gives nil. A nil result stays nil.
func wrap(owner ui.Screen, cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		if msg == nil {
			return nil
		}
		return ownedMsg{owner: owner, msg: msg}
	}
}

// owned handles the result of a Cmd of a screen. The commands of a batch
// keep the owner. The app handlers get the message first. Bubble Tea gets
// its own messages back. Other messages go to the owner when it is in the
// stack. When the owner is not in the stack, the app drops the message.
// Bubble Tea does not export the message of tea.Sequence, so the results
// of a sequence have no owner and go to the top screen.
func (m *model) owned(o ownedMsg) tea.Cmd {
	if batch, ok := o.msg.(tea.BatchMsg); ok {
		cmds := make([]tea.Cmd, len(batch))
		for i, c := range batch {
			cmds[i] = wrap(o.owner, c)
		}
		return tea.Batch(cmds...)
	}
	if cmd, ok := m.handle(o.msg); ok {
		return cmd
	}
	if reflect.TypeOf(o.msg).PkgPath() == runtimePkg {
		msg := o.msg
		return func() tea.Msg { return msg }
	}
	for i, s := range m.stack {
		if s == o.owner {
			next, cmd := s.Update(o.msg)
			m.stack[i] = next
			return wrap(next, cmd)
		}
	}
	return nil
}
