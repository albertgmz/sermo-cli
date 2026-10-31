package tui

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// toastTime is how long a toast shows.
const toastTime = 4 * time.Second

// model is the root Bubble Tea model. It owns the screen stack, the chrome
// and the session state.
type model struct {
	ctx   context.Context
	d     *ui.Deps
	opts  Options
	start time.Time

	stack  []ui.Screen // stack[0] is the home screen
	splash *splash     // not nil while the splash shows
	w, h   int

	site         string
	counters     api.Counters
	haveCounters bool
	pollGen      int

	toast       *ui.ToastMsg
	toastID     int
	confirmQuit bool
	ticking     bool
}

type hideToastMsg struct{ id int }

func (m *model) Init() tea.Cmd {
	cmds := []tea.Cmd{tea.RequestBackgroundColor, m.loadSite(), m.stack[0].Init(), m.nextClock()}
	if m.d.Cred.Token != "" {
		cmds = append(cmds, m.loadMe(false), m.loadCounters(false), m.nextPoll())
	}
	return tea.Batch(append(cmds, m.ensureTick())...)
}

// Update runs the handlers in order. The first handler that knows the
// message handles it. Other messages go to the top screen.
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	handled := false
	for _, h := range []func(tea.Msg) (tea.Cmd, bool){m.updateFrame, m.updateNav, m.updateSession, m.updateIntent} {
		if cmd, handled = h(msg); handled {
			break
		}
	}
	if !handled {
		cmd = m.forward(msg)
	}
	return m, tea.Batch(cmd, m.ensureTick())
}

// updateFrame handles the window, the keys, the clock and the toasts.
func (m *model) updateFrame(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		return m.resizeAll(), true
	case tea.BackgroundColorMsg:
		m.d.Glamour = glamourStyle(m.d, msg.IsDark())
		return nil, true
	case tea.KeyPressMsg:
		return m.handleKey(msg), true
	case ui.AnimTickMsg:
		m.ticking = false
		m.d.AnimT = msg.Time.Sub(m.start).Seconds()
		return m.forward(msg), true
	case clockMsg:
		return m.nextClock(), true
	case ui.ToastMsg:
		m.toastID++
		m.toast = &msg
		id := m.toastID
		return tea.Tick(toastTime, func(time.Time) tea.Msg { return hideToastMsg{id} }), true
	case hideToastMsg:
		if msg.id == m.toastID {
			m.toast = nil
		}
		return nil, true
	}
	return nil, false
}

func (m *model) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	if msg.String() == "ctrl+c" {
		return tea.Quit
	}
	if m.confirmQuit {
		m.confirmQuit = false
		if k, _ := globalKey(msg); k == "q" {
			return tea.Quit
		}
		return nil
	}
	if m.splash != nil {
		m.splash = nil
		return nil
	}
	if m.top().CapturesInput() {
		return m.forward(msg)
	}
	if k, ok := globalKey(msg); ok {
		return m.hotkey(k)
	}
	if isBack(msg) {
		return m.pop()
	}
	return m.forward(msg)
}

func (m *model) top() ui.Screen { return m.stack[len(m.stack)-1] }

func (m *model) forward(msg tea.Msg) tea.Cmd {
	s, cmd := m.top().Update(msg)
	m.stack[len(m.stack)-1] = s
	return cmd
}

// ensureTick starts the animation clock when a view animates and no tick
// waits. With reduced motion it never starts.
func (m *model) ensureTick() tea.Cmd {
	if m.ticking || !m.animating() {
		return nil
	}
	m.ticking = true
	return tea.Tick(time.Second/maxFPS, func(t time.Time) tea.Msg { return ui.AnimTickMsg{Time: t} })
}

func (m *model) animating() bool {
	if !m.d.Animate() {
		return false
	}
	if m.splash != nil {
		return m.splash.Animating()
	}
	a, ok := m.top().(ui.Animator)
	return ok && a.Animating()
}

// contentSize is the size of the content area inside the frame.
func (m *model) contentSize() (int, int) {
	return max(m.w-4, 0), max(m.h-4, 0)
}

func (m *model) resizeAll() tea.Cmd {
	cw, ch := m.contentSize()
	cmds := make([]tea.Cmd, len(m.stack))
	for i, s := range m.stack {
		m.stack[i], cmds[i] = s.Update(tea.WindowSizeMsg{Width: cw, Height: ch})
	}
	return tea.Batch(cmds...)
}

func (m *model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = m.site
	return v
}

func (m *model) render() string {
	switch {
	case m.w == 0 || m.h == 0:
		return ""
	case m.w < minWidth || m.h < minHeight:
		return tooSmall(m.d.Styles, m.w, m.h)
	case m.splash != nil:
		return m.splash.View(m.site, m.w, m.h)
	}
	c := m.chrome()
	bottom := c.hotkeyBar(m.w)
	if m.confirmQuit {
		bottom = c.confirmBar(m.w)
	}
	cw, ch := m.contentSize()
	content := m.top().View(cw, ch)
	screen := c.statusBar(m.w) + "\n" +
		ui.Frame(m.d.Styles, m.d.Theme, m.breadcrumb(), content, m.w, m.h-2) + "\n" + bottom
	if m.toast != nil {
		box := toastBox(m.d.Styles, m.d.Theme, *m.toast, m.w/2)
		bw, bh := ansi.StringWidth(strings.Split(box, "\n")[0]), strings.Count(box, "\n")+1
		screen = overlay(screen, box, m.w-bw-3, m.h-2-bh)
	}
	return screen
}

func (m *model) chrome() chrome {
	c := chrome{s: m.d.Styles, t: m.d.Theme, site: m.site, clock: m.d.Now().Format("15:04")}
	if m.d.Me != nil {
		c.user = m.d.Me.Username
		c.alerts, c.messages = m.counters.Notifications, m.counters.Conversations
	}
	return c
}

func (m *model) breadcrumb() string {
	parts := make([]string, len(m.stack))
	for i, s := range m.stack {
		parts[i] = s.Title()
	}
	return strings.Join(parts, " › ")
}
