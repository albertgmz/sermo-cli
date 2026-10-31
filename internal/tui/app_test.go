package tui

import (
	"bytes"
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/albertgmz/sermo-cli/internal/auth"
	"github.com/albertgmz/sermo-cli/internal/banner"
	"github.com/albertgmz/sermo-cli/internal/config"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

const wait = 300 * time.Millisecond

func TestSplashShowsBannerAndHandshake(t *testing.T) {
	f := newFakeSermo(t)
	m := newTestModel(t, testDeps(t, f, ""), StartSplash)
	drive(m, m.Init(), wait)
	out := screenText(m)
	logo := banner.Lines("Foros Cuba", 100-6)
	if len(logo) < 2 {
		t.Fatalf("expected a FIGlet banner, got %q", logo)
	}
	u, _ := url.Parse(f.URL)
	for _, want := range []string{strings.TrimSpace(logo[1]), "ATZ", "OK", "ATDT " + u.Host, "CONNECT 115200", "Bienvenido a Foros Cuba", "Pulsa cualquier tecla"} {
		if !strings.Contains(out, want) {
			t.Errorf("splash misses %q:\n%s", want, out)
		}
	}
}

func TestSplashKeyGoesHomeAndQQuits(t *testing.T) {
	f := newFakeSermo(t)
	m := newModel(context.Background(), testDeps(t, f, ""), Options{Start: StartSplash, Sleep: noSleep})
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(100, 30))
	waitText(t, tm, "Bienvenido")
	tm.Send(tea.KeyPressMsg{Code: 'x', Text: "x"})
	waitText(t, tm, "Esta pantalla llega pronto")
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func waitText(t *testing.T, tm *teatest.TestModel, text string) {
	t.Helper()
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains([]byte(ansi.Strip(string(b))), []byte(text))
	}, teatest.WithDuration(3*time.Second))
}

func TestChromeForGuest(t *testing.T) {
	f := newFakeSermo(t)
	m := newTestModel(t, testDeps(t, f, ""), StartHome)
	drive(m, m.Init(), wait)
	out := screenText(m)
	for _, want := range []string{"Foros Cuba", "Invitado", "14:05", "[F]oros", "[M]ensajes", "[N]otificaciones", "[B]uscar", "[P]erfil", "[?]Ayuda", "[Q]Salir", "Foros"} {
		if !strings.Contains(out, want) {
			t.Errorf("screen misses %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Alertas") {
		t.Error("a guest must not see badges")
	}
}

func TestChromeForMemberShowsBadges(t *testing.T) {
	f := newFakeSermo(t)
	f.notifications.Store(3)
	f.conversations.Store(2)
	m := newTestModel(t, testDeps(t, f, testToken), StartHome)
	drive(m, m.Init(), wait)
	out := screenText(m)
	for _, want := range []string{"albert", "Alertas  3 ", "Mensajes  2 "} {
		if !strings.Contains(out, want) {
			t.Errorf("status bar misses %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Invitado") {
		t.Error("a member must not show as a guest")
	}
}

func TestLayoutSizes(t *testing.T) {
	f := newFakeSermo(t)
	m := newTestModel(t, testDeps(t, f, ""), StartHome)
	drive(m, m.Init(), wait)
	for _, size := range [][2]int{{80, 24}, {160, 50}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		lines := strings.Split(screenText(m), "\n")
		if len(lines) != size[1] {
			t.Fatalf("%v: %d lines", size, len(lines))
		}
		for i, ln := range lines {
			if w := ansi.StringWidth(ln); w > size[0] {
				t.Fatalf("%v: line %d is %d wide: %q", size, i, w, ln)
			}
		}
		if !strings.Contains(lines[len(lines)-1], "[Q]Salir") {
			t.Fatalf("%v: hotkey bar missing: %q", size, lines[len(lines)-1])
		}
	}
	m.Update(tea.WindowSizeMsg{Width: 79, Height: 24})
	if out := screenText(m); !strings.Contains(out, "La ventana es muy pequeña (79×24)") {
		t.Fatalf("no resize message:\n%s", out)
	}
}

func TestHotkeyBarEnglishUsesKeycaps(t *testing.T) {
	f := newFakeSermo(t)
	d := testDeps(t, f, "")
	d.Config.Locale = "en"
	m := newTestModel(t, d, StartHome)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	lines := strings.Split(screenText(m), "\n")
	bar := lines[len(lines)-1]
	for _, want := range []string{"F Forums", "M Messages", "N Notifications", "B Search", "P Profile", "? Help", "Q Quit"} {
		if !strings.Contains(bar, want) {
			t.Errorf("bar misses %q: %q", want, bar)
		}
	}
	if strings.Contains(bar, "[") {
		t.Errorf("English bar must not use brackets: %q", bar)
	}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	lines = strings.Split(screenText(m), "\n")
	bar = lines[len(lines)-1]
	if !strings.Contains(bar, "Q") || ansi.StringWidth(bar) > 80 {
		t.Errorf("narrow bar must keep keys: %q", bar)
	}
}

func TestEscGoesBackAndQAsksOffHome(t *testing.T) {
	f := newFakeSermo(t)
	m := newTestModel(t, testDeps(t, f, ""), StartHome)
	pressKey(m, "B")
	if !strings.Contains(m.breadcrumb(), "Buscar") {
		t.Fatalf("B must open search, breadcrumb %q", m.breadcrumb())
	}
	if cmd := pressKey(m, "q"); cmd != nil && isQuit(cmd) {
		t.Fatal("q off home must ask first")
	}
	if !strings.Contains(screenText(m), "¿Quieres salir?") {
		t.Fatal("no quit question")
	}
	pressKey(m, "x")
	if strings.Contains(screenText(m), "¿Quieres salir?") {
		t.Fatal("another key must cancel the question")
	}
	pressKey(m, "esc")
	if len(m.stack) != 1 {
		t.Fatalf("esc must go back, stack %d", len(m.stack))
	}
	if !isQuit(pressKey(m, "q")) {
		t.Fatal("q on home must quit")
	}
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestGuestMembersHotkeyShowsLogin(t *testing.T) {
	f := newFakeSermo(t)
	m := newTestModel(t, testDeps(t, f, ""), StartHome)
	for _, k := range []string{"m", "n", "p"} {
		pressKey(m, k)
		out := screenText(m)
		if !strings.Contains(out, "Esta sección es solo para miembros.") || !strings.Contains(out, "[Enter] Entrar") {
			t.Fatalf("%s: no sign-in offer:\n%s", k, out)
		}
		pressKey(m, "esc")
	}
}

func TestDeviceLoginInsideTheApp(t *testing.T) {
	f := newFakeSermo(t)
	d := testDeps(t, f, "")
	m := newTestModel(t, d, StartHome)
	pressKey(m, "m")
	msgs := drive(m, pressKey(m, "enter"), 2*time.Second)
	if !hasMsg[codeMsg](msgs) || !hasMsg[loggedInMsg](msgs) {
		t.Fatalf("login did not finish: %#v", msgs)
	}
	assertSignedIn(t, d, f.URL)
	out := screenText(m)
	if !strings.Contains(out, "¡Hola, albert! Ya entraste.") || !strings.Contains(out, "albert") {
		t.Fatalf("no welcome:\n%s", out)
	}
	if _, ok := m.top().(*login); ok {
		t.Fatal("login screen must close")
	}
}

func assertSignedIn(t *testing.T, d *ui.Deps, server string) {
	t.Helper()
	if d.Cred.Token != testToken || d.Me == nil || d.Me.Username != "albert" {
		t.Fatalf("not signed in: %+v %+v", d.Cred, d.Me)
	}
	if tok, err := d.Store.Load(server); err != nil || tok.AccessToken != testToken {
		t.Fatalf("token not stored: %v %v", tok, err)
	}
}

func TestLoginWaitingViewShowsCodeAndAddress(t *testing.T) {
	f := newFakeSermo(t)
	d := testDeps(t, f, "")
	newModel(context.Background(), d, Options{})
	l := newLogin(context.Background(), d, "", noSleep)
	l.start()
	l.Update(codeMsg{from: l, code: auth.DeviceCode{UserCode: "WXYZ-1234", VerificationURI: "https://x/device", VerificationURIComplete: "https://x/device?c=1"}})
	l.stop()
	out := ansi.Strip(l.View(76, 20))
	for _, want := range []string{"https://x/device?c=1", "WXYZ-1234", "Esperando tu confirmación", "[O] Abrir el navegador"} {
		if !strings.Contains(out, want) {
			t.Errorf("waiting view misses %q:\n%s", want, out)
		}
	}
	if !l.CapturesInput() {
		t.Error("the waiting login must capture input")
	}
	l.Update(loginFailedMsg{from: l, err: auth.ErrExpired})
	if out := ansi.Strip(l.View(76, 20)); !strings.Contains(out, "El código venció") {
		t.Errorf("no expiry text:\n%s", out)
	}
	l.Update(loginFailedMsg{from: l, err: errors.New("x")})
	l.Update(loginFailedMsg{from: &login{}, err: auth.ErrDenied})
	if out := ansi.Strip(l.View(76, 20)); strings.Contains(out, "Rechazaste") {
		t.Error("a message from another login screen must not change this one")
	}
}

func TestUnauthorizedDeletesDeviceTokenAndOffersLogin(t *testing.T) {
	f := newFakeSermo(t)
	f.meStatus.Store(401)
	d := testDeps(t, f, testToken)
	if err := d.Store.Save(f.URL, auth.Token{AccessToken: testToken}); err != nil {
		t.Fatal(err)
	}
	m := newTestModel(t, d, StartHome)
	drive(m, m.Init(), wait)
	if _, err := d.Store.Load(f.URL); !errors.Is(err, auth.ErrNoToken) {
		t.Fatalf("token still stored: %v", err)
	}
	if d.Cred.Token != "" || d.Me != nil {
		t.Fatalf("still signed in: %+v", d.Cred)
	}
	out := screenText(m)
	for _, want := range []string{"Tu sesión venció o ya no es válida.", "[Enter] Entrar", "Invitado"} {
		if !strings.Contains(out, want) {
			t.Errorf("screen misses %q:\n%s", want, out)
		}
	}
}

func TestUnauthorizedKeepsAPIKey(t *testing.T) {
	f := newFakeSermo(t)
	f.meStatus.Store(401)
	d := testDeps(t, f, testToken)
	d.Cred.Source = auth.SourceAPIKey
	if err := d.Store.Save(f.URL, auth.Token{AccessToken: "other"}); err != nil {
		t.Fatal(err)
	}
	m := newTestModel(t, d, StartHome)
	drive(m, m.Init(), wait)
	if _, err := d.Store.Load(f.URL); err != nil {
		t.Fatalf("a stored device token must stay for an API key error: %v", err)
	}
}

func TestReducedMotionSchedulesNoAnimTick(t *testing.T) {
	f := newFakeSermo(t)
	d := testDeps(t, f, "")
	d.NoAnim = false
	m := newModel(context.Background(), d, Options{Start: StartSplash})
	if !hasMsg[ui.AnimTickMsg](firstLevel(m.Init())) {
		t.Fatal("with motion the splash must tick")
	}

	d2 := testDeps(t, f, "")
	d2.NoAnim = false
	d2.Config.ReducedMotion = true
	m2 := newTestModel(t, d2, StartSplash)
	if hasMsg[ui.AnimTickMsg](firstLevel(m2.Init())) {
		t.Fatal("reduced motion: Init scheduled an AnimTickMsg")
	}
	_, cmd := m2.Update(ui.AnimTickMsg{Time: time.Now()})
	if hasMsg[ui.AnimTickMsg](firstLevel(cmd)) {
		t.Fatal("reduced motion: Update scheduled an AnimTickMsg")
	}
	if hasMsg[ui.AnimTickMsg](firstLevel(pressKey(m2, "x"))) {
		t.Fatal("reduced motion: a key scheduled an AnimTickMsg")
	}
}

// firstLevel runs cmd and the commands of a batch once, without Update.
func firstLevel(cmd tea.Cmd) []tea.Msg {
	var out []tea.Msg
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		msg, ok := run(c, wait)
		if !ok {
			continue
		}
		if b, isBatch := msg.(tea.BatchMsg); isBatch {
			queue = append(queue, b...)
			continue
		}
		out = append(out, msg)
	}
	return out
}

func TestPollerToastsNewNotifications(t *testing.T) {
	f := newFakeSermo(t)
	f.notifications.Store(1)
	m := newTestModel(t, testDeps(t, f, testToken), StartHome)
	drive(m, m.Init(), wait)
	if strings.Contains(screenText(m), "notificaci") {
		t.Fatal("the first load must not toast")
	}
	poll := func() { drive(m, func() tea.Msg { return pollMsg{gen: m.pollGen} }, wait) }
	poll()
	if strings.Contains(screenText(m), "Tienes") {
		t.Fatal("no change must not toast")
	}
	f.notifications.Store(4)
	poll()
	out := screenText(m)
	if !strings.Contains(out, "Tienes 3 notificaciones nuevas") || !strings.Contains(out, "Alertas  4 ") {
		t.Fatalf("no toast or badge:\n%s", out)
	}
	m.toast = nil
	f.notifications.Store(0)
	poll()
	if strings.Contains(screenText(m), "Tienes") {
		t.Fatal("fewer notifications must not toast")
	}
	stale := drive(m, func() tea.Msg { return pollMsg{gen: m.pollGen - 1} }, wait)
	if hasMsg[countersMsg](stale) {
		t.Fatal("a stale poll must not load counters")
	}
}

func TestToastHides(t *testing.T) {
	f := newFakeSermo(t)
	m := newTestModel(t, testDeps(t, f, ""), StartHome)
	m.Update(ui.ToastMsg{Text: "Guardado", Kind: ui.ToastSuccess})
	if !strings.Contains(screenText(m), "Guardado") {
		t.Fatal("toast not shown")
	}
	m.Update(hideToastMsg{id: m.toastID - 1})
	if !strings.Contains(screenText(m), "Guardado") {
		t.Fatal("an old hide message must not hide a new toast")
	}
	m.Update(hideToastMsg{id: m.toastID})
	if strings.Contains(screenText(m), "Guardado") {
		t.Fatal("toast not hidden")
	}
}

func TestSettingsChangeThemeSavesAndRedraws(t *testing.T) {
	f := newFakeSermo(t)
	d := testDeps(t, f, "")
	m := newTestModel(t, d, StartHome)
	pressKey(m, "?")
	if _, ok := m.top().(*helpScreen); !ok {
		t.Fatal("? must open help")
	}
	if out := screenText(m); !strings.Contains(out, "Teclas generales") || !strings.Contains(out, "Ajustes") {
		t.Fatalf("help misses keys:\n%s", out)
	}
	drive(m, pressKey(m, "s"), wait)
	if _, ok := m.top().(*settings); !ok {
		t.Fatal("s must open settings")
	}
	before := m.View().Content
	drive(m, pressKey(m, "right"), wait)
	c, err := config.Load()
	if err != nil || c.Theme != "ambar-crt" {
		t.Fatalf("config theme = %q, %v", c.Theme, err)
	}
	if d.Theme.ID != "ambar-crt" || m.View().Content == before {
		t.Fatal("the theme change must redraw")
	}
	if !strings.Contains(screenText(m), "Ámbar CRT") {
		t.Fatal("settings must show the theme name")
	}
}

func TestSettingsChangeLanguage(t *testing.T) {
	f := newFakeSermo(t)
	d := testDeps(t, f, "")
	m := newTestModel(t, d, StartHome)
	m.push(newSettings(d))
	for range 3 {
		pressKey(m, "down")
	}
	pressKey(m, "right")
	if i18n.Locale() != "en" || d.Config.Locale != "en" {
		t.Fatalf("locale = %q", i18n.Locale())
	}
	if out := screenText(m); !strings.Contains(out, "Settings") || !strings.Contains(out, "Q Quit") {
		t.Fatalf("English not applied:\n%s", out)
	}
	if c, _ := config.Load(); c.Locale != "en" {
		t.Fatalf("saved locale %q", c.Locale)
	}
}
