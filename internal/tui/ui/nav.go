package ui

import (
	"io"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/cli/browser"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/i18n"
)

// PushMsg tells the app to show Screen on top of the current screen.
type PushMsg struct{ Screen Screen }

// PopMsg tells the app to go back to the previous screen.
type PopMsg struct{}

// ReplaceMsg tells the app to put Screen in the place of the current screen.
type ReplaceMsg struct{ Screen Screen }

// ToastKind sets the color of a toast.
type ToastKind int

// The kinds of toast.
const (
	ToastInfo ToastKind = iota
	ToastSuccess
	ToastError
)

// ToastMsg tells the app to show a short message. The message goes away
// after some seconds.
type ToastMsg struct {
	Text string
	Kind ToastKind
}

// AnimTickMsg is one frame of the animation clock. Deps.AnimT has the time
// of the frame.
type AnimTickMsg struct{ Time time.Time }

// UnauthorizedMsg tells the app that the server did not accept the
// credential. The app removes a stored device token, continues as a guest
// and shows the sign-in screen.
type UnauthorizedMsg struct{}

// LoginMsg tells the app to show the sign-in screen.
type LoginMsg struct{}

// OpenNodeMsg tells the app to show a forum node.
type OpenNodeMsg struct{ NodeID int }

// OpenThreadMsg tells the app to show a thread. A PostID of 0 opens the
// first unread post.
type OpenThreadMsg struct{ ThreadID, PostID int }

// OpenProfileMsg tells the app to show the profile of a user.
type OpenProfileMsg struct{ UserID int }

// OpenConversationMsg tells the app to show a conversation.
type OpenConversationMsg struct{ ID int }

// ComposeMsg tells the app to show the composer. Set one target:
// NodeID for a new thread, ThreadID for a reply, ConversationID for a reply
// in a conversation, or UserID for a new conversation with that user.
// Body is the start text in BB code, for example a quote.
type ComposeMsg struct {
	NodeID, ThreadID, ConversationID, UserID int
	Body                                     string
}

// Push returns a Cmd that shows s on top of the current screen.
func Push(s Screen) tea.Cmd { return func() tea.Msg { return PushMsg{Screen: s} } }

// Pop returns a Cmd that goes back to the previous screen.
func Pop() tea.Cmd { return func() tea.Msg { return PopMsg{} } }

// Replace returns a Cmd that puts s in the place of the current screen.
func Replace(s Screen) tea.Cmd { return func() tea.Msg { return ReplaceMsg{Screen: s} } }

// Toast returns a Cmd that shows a short message.
func Toast(text string, kind ToastKind) tea.Cmd {
	return func() tea.Msg { return ToastMsg{Text: text, Kind: kind} }
}

// Open returns a Cmd that opens url in the system browser. When the browser
// does not open, the Cmd gives an error toast.
func Open(url string) tea.Cmd {
	return func() tea.Msg {
		browser.Stdout, browser.Stderr = io.Discard, io.Discard
		if err := browser.OpenURL(url); err != nil {
			return ToastMsg{Text: i18n.T("core.toast.openFailed"), Kind: ToastError}
		}
		return nil
	}
}

// ErrorCmd returns a Cmd for an API error. A 401 gives UnauthorizedMsg.
// Other errors give an error toast with ErrorText. A nil err gives nil.
func ErrorCmd(err error) tea.Cmd {
	if err == nil {
		return nil
	}
	if api.IsUnauthorized(err) {
		return func() tea.Msg { return UnauthorizedMsg{} }
	}
	text := ErrorText(err)
	if text == "" {
		return nil
	}
	return Toast(text, ToastError)
}
