package ui

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/auth"
	"github.com/albertgmz/sermo-cli/internal/config"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/theme"
)

func TestErrorText(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"401", &api.Error{Status: 401, Message: "x"}, i18n.T("errors.unauthorized")},
		{"403", &api.Error{Status: 403, Message: "x"}, i18n.T("errors.forbidden")},
		{"404", &api.Error{Status: 404, Message: "x"}, i18n.T("errors.notFound")},
		{"429", &api.Error{Status: 429, RetryAfter: 1500 * time.Millisecond}, i18n.Tf("errors.rateLimited", map[string]any{"Seconds": 2})},
		{"429 no wait", &api.Error{Status: 429}, i18n.Tf("errors.rateLimited", map[string]any{"Seconds": 1})},
		{"422 server text", &api.Error{Status: 422, Message: "El título es muy corto."}, "El título es muy corto."},
		{"400 issue text", &api.Error{Status: 400, Message: "Invalid input", Issues: []api.Issue{{Path: "title", Message: "Too short"}}}, "Too short"},
		{"409 server text", &api.Error{Status: 409, Message: "Ya existe."}, "Ya existe."},
		{"500", &api.Error{Status: 500, Message: "boom"}, i18n.T("errors.server")},
		{"wrapped 404", fmt.Errorf("load: %w", &api.Error{Status: 404}), i18n.T("errors.notFound")},
		{"device unsupported", auth.ErrUnsupported, i18n.T("errors.deviceUnsupported")},
		{"network", &url.Error{Op: "Get", URL: "https://x", Err: &net.OpError{Op: "dial", Err: errors.New("refused")}}, i18n.T("errors.network")},
		{"canceled", context.Canceled, ""},
		{"nil", nil, ""},
	}
	for _, c := range cases {
		if got := ErrorText(c.err); got != c.want {
			t.Errorf("%s: ErrorText = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestErrorCmd(t *testing.T) {
	if _, ok := ErrorCmd(&api.Error{Status: 401})().(UnauthorizedMsg); !ok {
		t.Fatal("401 must give UnauthorizedMsg")
	}
	msg, ok := ErrorCmd(&api.Error{Status: 404})().(ToastMsg)
	if !ok || msg.Kind != ToastError || msg.Text != i18n.T("errors.notFound") {
		t.Fatalf("404 gave %#v", msg)
	}
	if ErrorCmd(nil) != nil {
		t.Fatal("nil error must give nil Cmd")
	}
}

func TestNavCommands(t *testing.T) {
	s := &fakeScreen{}
	if m, ok := Push(s)().(PushMsg); !ok || m.Screen != s {
		t.Fatal("Push")
	}
	if _, ok := Pop()().(PopMsg); !ok {
		t.Fatal("Pop")
	}
	if m, ok := Replace(s)().(ReplaceMsg); !ok || m.Screen != s {
		t.Fatal("Replace")
	}
	if m, ok := Toast("hola", ToastSuccess)().(ToastMsg); !ok || m.Text != "hola" || m.Kind != ToastSuccess {
		t.Fatal("Toast")
	}
}

func press(s string) tea.KeyPressMsg {
	switch s {
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "pgdown":
		return tea.KeyPressMsg{Code: tea.KeyPgDown}
	case "pgup":
		return tea.KeyPressMsg{Code: tea.KeyPgUp}
	case "home":
		return tea.KeyPressMsg{Code: tea.KeyHome}
	case "end":
		return tea.KeyPressMsg{Code: tea.KeyEnd}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	}
	r := []rune(s)[0]
	return tea.KeyPressMsg{Code: r, Text: s}
}

func TestListNavigation(t *testing.T) {
	var l List
	l.SetLen(50)
	l.SetHeight(10)
	steps := []struct {
		key  string
		want int
	}{
		{"down", 1}, {"j", 2}, {"k", 1}, {"up", 0}, {"up", 0},
		{"pgdown", 10}, {"pgdown", 20}, {"pgup", 10},
		{"G", 49}, {"down", 49}, {"g", 0}, {"end", 49}, {"home", 0},
	}
	for _, s := range steps {
		if l.Update(press(s.key)) {
			t.Fatalf("%s must not choose", s.key)
		}
		if l.Cursor() != s.want {
			t.Fatalf("after %s cursor = %d, want %d", s.key, l.Cursor(), s.want)
		}
	}
	if !l.Update(press("enter")) {
		t.Fatal("enter must choose")
	}
}

func TestListEmptyAndShrink(t *testing.T) {
	var l List
	if l.Update(press("enter")) {
		t.Fatal("enter on an empty list must not choose")
	}
	l.Update(press("down"))
	if l.Cursor() != 0 {
		t.Fatalf("cursor = %d on an empty list", l.Cursor())
	}
	l.SetLen(10)
	l.Update(press("G"))
	l.SetLen(3)
	if l.Cursor() != 2 {
		t.Fatalf("cursor = %d after shrink, want 2", l.Cursor())
	}
}

func TestListViewScrollsAndMarksSelection(t *testing.T) {
	var l List
	l.SetLen(30)
	l.SetHeight(5)
	for range 7 {
		l.Update(press("down"))
	}
	th := theme.ByID("")
	out := l.View(NewStyles(th), th, 40, 0, func(i, w int) string { return fmt.Sprintf("row %02d", i) })
	lines := strings.Split(ansi.Strip(out), "\n")
	if len(lines) != 5 {
		t.Fatalf("got %d lines, want 5", len(lines))
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "row 07") || strings.Contains(joined, "row 00") {
		t.Fatalf("selected row not in view:\n%s", joined)
	}
	for _, ln := range lines {
		if ansi.StringWidth(ln) > 40 {
			t.Fatalf("line wider than 40: %q", ln)
		}
		if strings.Contains(ln, "row 07") && !strings.HasPrefix(ln, "▶") {
			t.Fatalf("selected row has no marker: %q", ln)
		}
	}
}

func TestHighlightIsStaticWithSamePhase(t *testing.T) {
	th := theme.ByID("")
	a := Highlight(th, "Foros generales", 30, 0)
	b := Highlight(th, "Foros generales", 30, 0)
	c := Highlight(th, "Foros generales", 30, 0.4)
	if a != b {
		t.Fatal("same phase must give the same output")
	}
	if a == c {
		t.Fatal("other phase must change the colors")
	}
	if got := ansi.StringWidth(a); got != 30 {
		t.Fatalf("width = %d, want 30", got)
	}
}

func TestFrameSizeAndTitle(t *testing.T) {
	th := theme.ByID("")
	out := Frame(NewStyles(th), th, "Foros › General", "hola\nmundo", 40, 8)
	lines := strings.Split(ansi.Strip(out), "\n")
	if len(lines) != 8 {
		t.Fatalf("got %d lines, want 8", len(lines))
	}
	for i, ln := range lines {
		if w := ansi.StringWidth(ln); w != 40 {
			t.Fatalf("line %d width %d, want 40: %q", i, w, ln)
		}
	}
	if !strings.Contains(lines[0], "Foros › General") {
		t.Fatalf("title missing: %q", lines[0])
	}
	if !strings.Contains(lines[1], "hola") || !strings.Contains(lines[2], "mundo") {
		t.Fatalf("content missing:\n%s", strings.Join(lines, "\n"))
	}
}

func TestFrameCutsLongContent(t *testing.T) {
	th := theme.ByID("")
	long := strings.Repeat("x", 100) + "\n" + strings.Repeat("y\n", 20)
	out := Frame(NewStyles(th), th, strings.Repeat("t", 80), long, 30, 6)
	lines := strings.Split(ansi.Strip(out), "\n")
	if len(lines) != 6 {
		t.Fatalf("got %d lines, want 6", len(lines))
	}
	for _, ln := range lines {
		if w := ansi.StringWidth(ln); w != 30 {
			t.Fatalf("width %d, want 30: %q", w, ln)
		}
	}
}

func TestHeaderWidth(t *testing.T) {
	th := theme.ByID("")
	for _, w := range []int{10, 40, 120} {
		out := Header(NewStyles(th), th, "Notificaciones", w)
		if got := ansi.StringWidth(out); got != w {
			t.Fatalf("width = %d, want %d", got, w)
		}
		if strings.Contains(out, "\n") {
			t.Fatal("header must be one line")
		}
	}
	if !strings.Contains(ansi.Strip(Header(NewStyles(th), th, "Notificaciones", 40)), "Notificaciones") {
		t.Fatal("header text missing")
	}
}

func TestEmptyAndLoading(t *testing.T) {
	s := NewStyles(theme.ByID(""))
	out := ansi.Strip(Empty(s, "Nada por aquí", 40, 5))
	if !strings.Contains(out, "Nada por aquí") || len(strings.Split(out, "\n")) != 5 {
		t.Fatalf("Empty = %q", out)
	}
	out = ansi.Strip(Loading(s, NewSpinner(s), "", 40, 5))
	if !strings.Contains(out, i18n.T("core.loading")) {
		t.Fatalf("Loading = %q", out)
	}
}

func TestDepsAnimate(t *testing.T) {
	d := &Deps{Config: &config.Config{}}
	if !d.Animate() {
		t.Fatal("default must animate")
	}
	d.NoAnim = true
	if d.Animate() {
		t.Fatal("NoAnim must stop animation")
	}
	d.NoAnim = false
	d.Config.ReducedMotion = true
	if d.Animate() || d.Phase() != 0 {
		t.Fatal("reduced motion must stop animation")
	}
}

func TestReadingWidth(t *testing.T) {
	if ReadingWidth(76) != 76 || ReadingWidth(200) != MaxReadingWidth {
		t.Fatal("ReadingWidth")
	}
}

type fakeScreen struct{}

func (*fakeScreen) Init() tea.Cmd                      { return nil }
func (f *fakeScreen) Update(tea.Msg) (Screen, tea.Cmd) { return f, nil }
func (*fakeScreen) View(int, int) string               { return "" }
func (*fakeScreen) Title() string                      { return "" }
func (*fakeScreen) Keys() []key.Binding                { return nil }
func (*fakeScreen) CapturesInput() bool                { return false }

func TestListAnimatesShortlyAfterMoves(t *testing.T) {
	now := time.Date(2026, 10, 31, 14, 5, 0, 0, time.UTC)
	var l List
	l.SetClock(func() time.Time { return now })
	l.SetHeight(5)
	if l.Animating(now) {
		t.Fatal("an empty list must not animate")
	}
	l.SetLen(10)
	if !l.Animating(now.Add(1999 * time.Millisecond)) {
		t.Fatal("a list must animate after it first gets items")
	}
	if l.Animating(now.Add(2 * time.Second)) {
		t.Fatal("an idle list must stop after 2 s")
	}
	now = now.Add(time.Minute)
	l.SetLen(12)
	l.Update(press("up"))
	if l.Animating(now) {
		t.Fatal("more items or a key that does not move the cursor must not animate")
	}
	l.Update(press("down"))
	if !l.Animating(now.Add(time.Second)) {
		t.Fatal("a cursor move must animate")
	}
	now = now.Add(time.Minute)
	l.Reset()
	l.SetLen(3)
	if !l.Animating(now) || l.Len() != 3 || l.Cursor() != 0 {
		t.Fatal("Reset must empty the list and keep the clock")
	}
}

func TestListZeroValueUsesSystemClock(t *testing.T) {
	var l List
	l.SetLen(3)
	if !l.Animating(time.Now()) {
		t.Fatal("a new list with items must animate")
	}
	if l.Animating(time.Now().Add(3 * time.Second)) {
		t.Fatal("the list must stop after 2 s")
	}
}
