package tui

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/albertgmz/sermo-cli/internal/auth"
	"github.com/albertgmz/sermo-cli/internal/banner"
	"github.com/albertgmz/sermo-cli/internal/buildinfo"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

type loginState int

const (
	loginOffer loginState = iota
	loginRequesting
	loginWaiting
	loginFailed
)

// login runs the device login inside the terminal. It shows the user code
// and the address, and waits until the user approves the code in the
// browser.
type login struct {
	ctx    context.Context
	run    context.Context // the context of the running login
	cancel context.CancelFunc
	d      *ui.Deps
	sleep  func(context.Context, time.Duration) error
	reason string
	state  loginState
	code   auth.DeviceCode
	err    string
	sp     spinner.Model
}

// codeMsg is the reply to the device code request of one login screen.
type codeMsg struct {
	from *login
	code auth.DeviceCode
	err  error
}

// loginFailedMsg tells a login screen that the poll for the token failed.
type loginFailedMsg struct {
	from *login
	err  error
}

// loggedInMsg tells the app that the user approved the device login.
type loggedInMsg struct {
	token   string
	saveErr error
}

func newLogin(ctx context.Context, d *ui.Deps, reason string, sleep func(context.Context, time.Duration) error) *login {
	return &login{ctx: ctx, d: d, reason: reason, sleep: sleep, sp: ui.NewSpinner(d.Styles)}
}

func (l *login) Init() tea.Cmd { return nil }

func (l *login) Title() string { return i18n.T("core.login.title") }

// CapturesInput is true while the login runs, so that Esc cancels it.
func (l *login) CapturesInput() bool {
	return l.state == loginRequesting || l.state == loginWaiting
}

func (l *login) Keys() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", i18n.T("core.login.keyStart"))),
		key.NewBinding(key.WithKeys("o"), key.WithHelp("O", i18n.T("core.login.keyOpen"))),
		key.NewBinding(key.WithKeys("c"), key.WithHelp("C", i18n.T("core.login.keyCopy"))),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("Esc", i18n.T("core.login.keyCancel"))),
	}
}

func (l *login) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return l, l.key(msg)
	case codeMsg:
		if msg.from == l {
			return l, l.gotCode(msg)
		}
	case loginFailedMsg:
		if msg.from == l {
			l.fail(msg.err)
		}
	case spinner.TickMsg:
		if l.state == loginRequesting || l.state == loginWaiting {
			var cmd tea.Cmd
			l.sp, cmd = l.sp.Update(msg)
			return l, cmd
		}
	}
	return l, nil
}

func (l *login) key(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		if l.state == loginOffer || l.state == loginFailed {
			return l.start()
		}
	case "esc":
		l.stop()
		return ui.Pop()
	case "o", "O":
		if l.state == loginWaiting {
			return ui.Open(l.address())
		}
	case "c", "C":
		if l.state == loginWaiting {
			return tea.Batch(tea.SetClipboard(l.code.UserCode), ui.Toast(i18n.T("core.toast.copied"), ui.ToastSuccess))
		}
	}
	return nil
}

func (l *login) start() tea.Cmd {
	l.stop()
	ctx, cancel := context.WithCancel(l.ctx)
	l.run, l.cancel = ctx, cancel
	l.state, l.err = loginRequesting, ""
	dev := l.device()
	cmds := []tea.Cmd{func() tea.Msg {
		dc, err := dev.RequestCode(ctx)
		return codeMsg{from: l, code: dc, err: err}
	}}
	if l.d.Animate() {
		cmds = append(cmds, l.sp.Tick)
	}
	return tea.Batch(cmds...)
}

func (l *login) gotCode(msg codeMsg) tea.Cmd {
	if msg.err != nil {
		l.fail(msg.err)
		return nil
	}
	l.state, l.code = loginWaiting, msg.code
	ctx, dev, store, server := l.run, l.device(), l.d.Store, l.d.Server
	return func() tea.Msg {
		tok, err := dev.PollToken(ctx, msg.code)
		if err != nil {
			return loginFailedMsg{from: l, err: err}
		}
		return loggedInMsg{token: tok.AccessToken, saveErr: store.Save(server, tok)}
	}
}

func (l *login) fail(err error) {
	l.stop()
	if errors.Is(err, context.Canceled) {
		l.state = loginOffer
		return
	}
	l.state, l.err = loginFailed, loginErrorText(err)
}

func loginErrorText(err error) string {
	switch {
	case errors.Is(err, auth.ErrExpired):
		return i18n.T("core.login.expired")
	case errors.Is(err, auth.ErrDenied):
		return i18n.T("core.login.denied")
	case errors.Is(err, auth.ErrInvalidGrant):
		return i18n.T("core.login.invalidGrant")
	case errors.Is(err, auth.ErrBanned):
		return i18n.T("core.login.banned")
	}
	var rl *auth.RateLimitError
	if errors.As(err, &rl) {
		secs := max(1, int(math.Ceil(rl.Wait.Seconds())))
		return i18n.Tf("errors.rateLimited", map[string]any{"Seconds": secs})
	}
	return ui.ErrorText(err)
}

func (l *login) stop() {
	if l.cancel != nil {
		l.cancel()
		l.cancel = nil
	}
}

func (l *login) device() *auth.Device {
	return &auth.Device{Server: l.d.Server, UserAgent: buildinfo.UserAgent(), Sleep: l.sleep}
}

func (l *login) address() string {
	if l.code.VerificationURIComplete != "" {
		return l.code.VerificationURIComplete
	}
	return l.code.VerificationURI
}

func (l *login) View(width, height int) string {
	if l.d.BBS() {
		return l.bbsView(width)
	}
	s := l.d.Styles
	w := min(width, 72)
	var body []string
	switch l.state {
	case loginOffer:
		body = l.offerView(w)
	case loginRequesting:
		body = []string{l.sp.View() + " " + s.Muted.Render(i18n.T("core.login.requesting"))}
	case loginWaiting:
		body = l.waitView(w)
	case loginFailed:
		body = []string{s.Error.Render(lipgloss.Wrap(l.err, w, " ")), "",
			hints(s, [][2]string{{"Enter", "core.login.keyRetry"}, {"Esc", "core.key.back"}})}
	}
	head := ui.Header(s, l.d.Theme, i18n.T("core.login.title"), width)
	box := lipgloss.PlaceHorizontal(width, lipgloss.Center, lipgloss.NewStyle().Width(w).Render(strings.Join(body, "\n")))
	return head + "\n\n" + box
}

func (l *login) offerView(w int) []string {
	s := l.d.Styles
	var out []string
	if l.reason != "" {
		out = append(out, s.Warning.Bold(true).Render(lipgloss.Wrap(l.reason, w, " ")), "")
	}
	return append(out, s.Text.Render(lipgloss.Wrap(i18n.T("core.login.offer"), w, " ")), "",
		hints(s, [][2]string{{"Enter", "core.login.keyStart"}, {"Esc", "core.login.keyGuest"}}))
}

func (l *login) waitView(w int) []string {
	s := l.d.Styles
	out := []string{
		s.Text.Render(i18n.T("core.login.step1")),
		"   " + s.Accent.Underline(true).Render(l.address()),
		"",
		s.Text.Render(i18n.T("core.login.step2")),
	}
	for _, line := range banner.Lines(l.code.UserCode, w-3) {
		out = append(out, "   "+l.d.Theme.Title.Text(line, 0))
	}
	out = append(out, "   "+s.Badge.Render(l.code.UserCode), "",
		l.sp.View()+" "+s.Muted.Render(i18n.T("core.login.waiting")), "",
		hints(s, [][2]string{{"O", "core.login.keyOpen"}, {"C", "core.login.keyCopy"}, {"Esc", "core.login.keyCancel"}}))
	return out
}

// hints draws key hints like "[Enter] Entrar". Each pair is a key and an
// i18n ID.
func hints(s ui.Styles, pairs [][2]string) string {
	parts := make([]string, len(pairs))
	for i, p := range pairs {
		parts[i] = s.Key.Render("["+p[0]+"]") + " " + s.Text.Render(i18n.T(p[1]))
	}
	return strings.Join(parts, "   ")
}

// Prompt is the label of the BBS prompt.
func (l *login) Prompt() string { return i18n.T("bbs.prompt.login") }

// Commands are the BBS hints of the login. The keys are the same as in the
// modern view. While the login runs, the screen takes the keys itself.
func (l *login) Commands() []ui.Command {
	switch l.state {
	case loginRequesting:
		return []ui.Command{{Label: i18n.T("bbs.login.cancel")}}
	case loginWaiting:
		return []ui.Command{
			{Name: "O", Label: i18n.T("bbs.login.open")},
			{Name: "C", Label: i18n.T("bbs.login.copy")},
			{Label: i18n.T("bbs.login.cancel")},
		}
	case loginFailed:
		return []ui.Command{{Name: "", Label: i18n.T("bbs.login.retry"), Run: func(string) tea.Cmd { return l.start() }},
			{Name: "V", Label: i18n.T("bbs.global.back")}}
	}
	return []ui.Command{{Name: "", Label: i18n.T("bbs.login.start"), Run: func(string) tea.Cmd { return l.start() }},
		{Name: "V", Label: i18n.T("bbs.login.guest")}}
}

// bbsView shows the device login as lines of a terminal session.
func (l *login) bbsView(width int) string {
	s := l.d.Styles
	mark := s.Accent.Render(">> ")
	ok := " " + s.Success.Render(i18n.T("bbs.ok"))
	out := []string{"", "  " + s.Subtitle.Render(strings.ToUpper(i18n.T("core.login.title"))), ""}
	if l.reason != "" && l.state == loginOffer {
		out = append(out, "  "+mark+s.Warning.Render(l.reason), "")
	}
	switch l.state {
	case loginOffer:
		for _, line := range strings.Split(lipgloss.Wrap(i18n.T("bbs.login.offer"), max(width-7, 10), " "), "\n") {
			out = append(out, "  "+mark+s.Text.Render(line))
		}
	case loginRequesting:
		out = append(out, "  "+mark+s.Text.Render(i18n.T("bbs.login.requesting")+" ..."))
	case loginWaiting:
		out = append(out, l.bbsWaiting(width, mark, ok)...)
	case loginFailed:
		out = append(out, "  "+mark+s.Error.Render(l.err))
	}
	return strings.Join(out, "\n")
}

func (l *login) bbsWaiting(width int, mark, ok string) []string {
	s := l.d.Styles
	out := []string{
		"  " + mark + s.Text.Render(i18n.T("bbs.login.requesting")+" ...") + ok,
		"  " + mark + s.Text.Render(i18n.T("bbs.login.address")),
		"     " + s.Accent.Render(l.address()),
		"  " + mark + s.Text.Render(i18n.T("bbs.login.code")),
	}
	for _, line := range banner.Lines(l.code.UserCode, width-6) {
		out = append(out, "     "+s.Accent.Render(line))
	}
	return append(out, "     "+s.Key.Render(l.code.UserCode), "",
		"  "+mark+s.Text.Render(i18n.T("bbs.login.waiting")+" ..."))
}
