package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/albertgmz/sermo-cli/internal/banner"
	"github.com/albertgmz/sermo-cli/internal/config"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// fakeCommander is a screen with BBS commands. It records the commands
// that run and the keys that it gets.
type fakeCommander struct {
	names []string // the names of the commands, in order
	ran   []string // "name:arg" of each command that ran
	keys  []string
}

func newFakeCommander(names ...string) *fakeCommander { return &fakeCommander{names: names} }

func (f *fakeCommander) Init() tea.Cmd { return nil }
func (f *fakeCommander) Update(msg tea.Msg) (ui.Screen, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		f.keys = append(f.keys, k.String())
	}
	return f, nil
}
func (f *fakeCommander) View(width, height int) string { return "contenido de prueba" }
func (f *fakeCommander) Title() string                 { return "Prueba" }
func (f *fakeCommander) Keys() []key.Binding           { return nil }
func (f *fakeCommander) CapturesInput() bool           { return false }
func (f *fakeCommander) Prompt() string                { return "Leyendo" }
func (f *fakeCommander) Commands() []ui.Command {
	out := make([]ui.Command, len(f.names))
	for i, n := range f.names {
		label := "[" + n + "]prueba"
		out[i] = ui.Command{Name: n, Label: label, Run: func(arg string) tea.Cmd {
			f.ran = append(f.ran, n+":"+arg)
			return nil
		}}
	}
	return out
}

func bbsDeps(t *testing.T, f *fakeSermo, token string) *ui.Deps {
	t.Helper()
	d := testDeps(t, f, token)
	d.Config.Interface = config.InterfaceBBS
	return d
}

// bbsModel returns a BBS model of 80x24 with the data loaded and s on top
// of home. A nil s keeps home on top.
func bbsModel(t *testing.T, d *ui.Deps, s ui.Screen) *model {
	t.Helper()
	m := newTestModel(t, d, StartHome)
	drive(m, m.Init(), wait)
	drive(m, func() tea.Msg { return tea.WindowSizeMsg{Width: 80, Height: 24} }, wait)
	if s != nil {
		m.push(s)
	}
	return m
}

func typeText(m *model, text string) {
	for _, r := range text {
		m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

// submit types text, presses Enter and runs the result.
func submit(m *model, text string) []tea.Msg {
	typeText(m, text)
	return drive(m, pressKey(m, "enter"), wait)
}

func TestParseCommand(t *testing.T) {
	for _, c := range []struct{ in, name, arg string }{
		{"12", "#", "12"},
		{"  007 ", "#", "007"},
		{"", "", ""},
		{"   ", "", ""},
		{"r", "R", ""},
		{"r  hola mundo ", "R", "hola mundo"},
		{"Busca gatos", "BUSCA", "gatos"},
		{"12a", "12A", ""},
		{"?", "?", ""},
	} {
		if name, arg := parseCommand(c.in); name != c.name || arg != c.arg {
			t.Errorf("parseCommand(%q) = %q, %q, want %q, %q", c.in, name, arg, c.name, c.arg)
		}
	}
}

func TestPromptRunsScreenCommands(t *testing.T) {
	f := newFakeSermo(t)
	scr := newFakeCommander("#", "R", "F", "")
	m := bbsModel(t, bbsDeps(t, f, ""), scr)
	submit(m, "12")
	submit(m, "r hola")
	submit(m, "f")
	submit(m, "")
	want := []string{"#:12", "R:hola", "F:", ":"}
	if strings.Join(scr.ran, ",") != strings.Join(want, ",") {
		t.Errorf("ran %q, want %q", scr.ran, want)
	}
	if len(m.stack) != 2 || m.top() != scr {
		t.Errorf("the command F of the screen must win over the global F, stack %d", len(m.stack))
	}
	if m.input != "" {
		t.Errorf("prompt not cleared: %q", m.input)
	}
}

func TestPromptGlobalFallbackAndUnknown(t *testing.T) {
	f := newFakeSermo(t)
	scr := newFakeCommander("R")
	m := bbsModel(t, bbsDeps(t, f, ""), scr)
	msgs := submit(m, "xyz")
	if !hasToast(msgs, i18n.T("bbs.unknown")) {
		t.Errorf("unknown command gives no toast: %v", msgs)
	}
	if !strings.Contains(screenText(m), ">> "+i18n.T("bbs.unknown")) {
		t.Errorf("BBS toast line missing:\n%s", screenText(m))
	}
	submit(m, "")
	if len(scr.keys) != 1 || scr.keys[0] != "enter" {
		t.Errorf("a bare Enter without the command \"\" must go to the screen, keys %q", scr.keys)
	}
	submit(m, "S")
	if _, ok := m.top().(*settings); !ok {
		t.Fatalf("S must open the settings, top %T", m.top())
	}
	submit(m, "v")
	if m.top() != scr {
		t.Errorf("V must go back, top %T", m.top())
	}
}

func hasToast(msgs []tea.Msg, text string) bool {
	for _, msg := range msgs {
		if tm, ok := msg.(ui.ToastMsg); ok && tm.Text == text {
			return true
		}
	}
	return false
}

func TestPromptEscClearsThenGoesBack(t *testing.T) {
	f := newFakeSermo(t)
	scr := newFakeCommander("R")
	m := bbsModel(t, bbsDeps(t, f, ""), scr)
	typeText(m, "hola")
	pressKey(m, "esc")
	if m.input != "" || len(m.stack) != 2 {
		t.Fatalf("Esc must clear the prompt first: input %q, stack %d", m.input, len(m.stack))
	}
	drive(m, pressKey(m, "esc"), wait)
	if len(m.stack) != 1 {
		t.Errorf("Esc on an empty prompt must go back, stack %d", len(m.stack))
	}
}

func TestArrowsReachScreenWhilePromptHasText(t *testing.T) {
	f := newFakeSermo(t)
	scr := newFakeCommander("R")
	m := bbsModel(t, bbsDeps(t, f, ""), scr)
	typeText(m, "ab")
	pressKey(m, "down")
	m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	if strings.Join(scr.keys, ",") != "down,pgdown" || m.input != "ab" {
		t.Errorf("keys %q, input %q", scr.keys, m.input)
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m.input != "a" {
		t.Errorf("backspace must remove one character: %q", m.input)
	}
	m.Update(tea.PasteMsg{Content: "xy"})
	if m.input != "axy" {
		t.Errorf("paste must go to the prompt: %q", m.input)
	}
}

func TestBBSChromeAt80x24(t *testing.T) {
	f := newFakeSermo(t)
	f.conversations.Store(3)
	m := bbsModel(t, bbsDeps(t, f, testToken), newFakeCommander("R", "M"))
	lines := strings.Split(screenText(m), "\n")
	if len(lines) != 24 {
		t.Fatalf("got %d lines, want 24", len(lines))
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w != 80 {
			t.Errorf("line %d has width %d: %q", i, w, l)
		}
	}
	assertStatusBox(t, lines[:3])
	if !strings.Contains(lines[3], "contenido de prueba") {
		t.Errorf("content must start under the box: %q", lines[3])
	}
	if !strings.HasPrefix(lines[22], " [R]prueba  [M]prueba(3)") {
		t.Errorf("hint line: %q", lines[22])
	}
	if !strings.HasPrefix(lines[23], " Leyendo > _") {
		t.Errorf("prompt line: %q", lines[23])
	}
	typeText(m, "12")
	if !strings.HasPrefix(strings.Split(screenText(m), "\n")[23], " Leyendo > 12_") {
		t.Error("the prompt must show the typed text")
	}
}

func assertStatusBox(t *testing.T, lines []string) {
	t.Helper()
	border := "+" + strings.Repeat("-", 78) + "+"
	if lines[0] != border || lines[2] != border {
		t.Errorf("status box borders:\n%s\n%s", lines[0], lines[2])
	}
	for _, want := range []string{"| FOROS CUBA BBS", "En línea: 37", "albert [VETERANO]", "Sáb 31-Oct  14:05 |"} {
		if !strings.Contains(lines[1], want) {
			t.Errorf("status line misses %q: %q", want, lines[1])
		}
	}
}

func TestBBSFallbackShowsModernScreenInChrome(t *testing.T) {
	f := newFakeSermo(t)
	m := bbsModel(t, bbsDeps(t, f, ""), nil)
	out := screenText(m)
	lines := strings.Split(out, "\n")
	if len(lines) != 24 || !strings.Contains(lines[1], "Invitado") {
		t.Fatalf("no BBS chrome:\n%s", out)
	}
	if !strings.Contains(out, i18n.T("forum.nodes.empty")) {
		t.Errorf("home content missing:\n%s", out)
	}
	if !strings.HasPrefix(lines[22], " [F]oros  [M]ensajes  [N]otificaciones") || !strings.HasPrefix(lines[23], " "+i18n.T("bbs.prompt.command")+" > _") {
		t.Errorf("global hints or prompt missing:\n%s\n%s", lines[22], lines[23])
	}
	if strings.Contains(out, "╔") || strings.Contains(out, "║") {
		t.Errorf("the modern frame must not show:\n%s", out)
	}
}

func TestBBSSplashShowsRealData(t *testing.T) {
	f := newFakeSermo(t)
	f.notifications.Store(3)
	f.conversations.Store(2)
	m := newTestModel(t, bbsDeps(t, f, testToken), StartSplash)
	drive(m, m.Init(), wait)
	drive(m, func() tea.Msg { return tea.WindowSizeMsg{Width: 80, Height: 24} }, wait)
	out := screenText(m)
	lines := strings.Split(out, "\n")
	if len(lines) != 24 {
		t.Fatalf("splash has %d lines, want 24:\n%s", len(lines), out)
	}
	logo := banner.Lines("Foros Cuba", 70)
	for _, want := range []string{
		"+" + strings.Repeat("=", 76) + "+", strings.TrimSpace(logo[1]),
		"-=[  De Cuba para el mundo  ]=-", "Registrados: 4.182", "En línea: 37", "Terminal: 80x24", "Charset: UTF-8",
		">> Conectando con 127.0.0.1", "[ OK ]", ">> Sesión iniciada como albert.",
		">> Tienes 2 privados sin leer y 3 notificaciones nuevas.", "[ Presiona ENTER para continuar ]",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("splash misses %q:\n%s", want, out)
		}
	}
	for _, bad := range []string{"Clave", "Usuario", "Nodo"} {
		if strings.Contains(out, bad) {
			t.Errorf("splash must not show %q", bad)
		}
	}
	pressKey(m, "enter")
	if m.splash != nil || !strings.Contains(screenText(m), " "+i18n.T("bbs.prompt.command")+" > _") {
		t.Error("Enter must leave the splash for the BBS home")
	}
}

func TestBBSSplashForGuest(t *testing.T) {
	f := newFakeSermo(t)
	m := newTestModel(t, bbsDeps(t, f, ""), StartSplash)
	drive(m, m.Init(), wait)
	out := screenText(m)
	if !strings.Contains(out, ">> "+i18n.T("bbs.splash.guest")) || strings.Contains(out, "Tienes") {
		t.Errorf("guest splash:\n%s", out)
	}
}

func TestBBSNoAnimationTicks(t *testing.T) {
	f := newFakeSermo(t)
	d := bbsDeps(t, f, "")
	d.NoAnim = false
	m := newTestModel(t, d, StartHome)
	msgs := drive(m, m.Init(), wait)
	msgs = append(msgs, drive(m, ui.Toast("hola", ui.ToastInfo), wait)...)
	m.push(newSettings(d))
	msgs = append(msgs, drive(m, pressKey(m, "down"), wait)...)
	if hasMsg[ui.AnimTickMsg](msgs) || m.animating() {
		t.Error("the BBS interface must not send animation ticks")
	}
}

func TestBBSSplashCrawlTicksStop(t *testing.T) {
	f := newFakeSermo(t)
	d := bbsDeps(t, f, "")
	d.NoAnim = false
	m := newTestModel(t, d, StartSplash)
	if !m.animating() {
		t.Fatal("the splash text must type out")
	}
	if strings.Contains(screenText(m), i18n.T("bbs.splash.enter")) {
		t.Error("the Enter line must wait for the text")
	}
	d.AnimT = 60
	if m.animating() {
		t.Error("the ticks must stop when the text is complete")
	}
	d.Config.ReducedMotion = true
	d.AnimT = 0
	if m.animating() || !strings.Contains(screenText(m), i18n.T("bbs.splash.enter")) {
		t.Error("reduced motion must show all text at once")
	}
}

func TestSettingsSwitchesInterface(t *testing.T) {
	f := newFakeSermo(t)
	d := testDeps(t, f, "")
	m := newTestModel(t, d, StartHome)
	drive(m, m.Init(), wait)
	m.push(newSettings(d))
	if !strings.Contains(screenText(m), "Interfaz") || !strings.Contains(screenText(m), "◀ Moderna ▶") {
		t.Fatalf("settings misses the interface row:\n%s", screenText(m))
	}
	for range rowInterface {
		pressKey(m, "down")
	}
	drive(m, pressKey(m, "right"), wait)
	if !d.BBS() {
		t.Fatal("the interface must change at once")
	}
	if c, _ := config.Load(); c.Interface != config.InterfaceBBS {
		t.Errorf("saved interface = %q", c.Interface)
	}
	out := screenText(m)
	if !strings.Contains(out, "< BBS >") || !strings.Contains(out, " Ajustes > _") {
		t.Errorf("the BBS view must show at once:\n%s", out)
	}
	submit(m, "6")
	drive(m, pressKey(m, "enter"), wait)
	if d.BBS() {
		t.Error("the BBS command must switch back to the modern interface")
	}
}

func TestBBSContentSize(t *testing.T) {
	f := newFakeSermo(t)
	m := newTestModel(t, bbsDeps(t, f, ""), StartHome)
	if cw, ch := m.contentSize(); cw != 100 || ch != 30-bbsChromeHeight {
		t.Errorf("content size %dx%d, want the full width and the height without the chrome", cw, ch)
	}
}

func TestBBSHelpListsCommandsInTable(t *testing.T) {
	f := newFakeSermo(t)
	m := bbsModel(t, bbsDeps(t, f, ""), newFakeCommander("#", "R"))
	submit(m, "?")
	out := screenText(m)
	for _, want := range []string{"| " + i18n.T("bbs.help.command"), "ESTA PANTALLA", "| ##", "[R]prueba", "GLOBALES", "[S]ajustes", "+=====", "| Q "} {
		if !strings.Contains(out, want) {
			t.Errorf("help misses %q:\n%s", want, out)
		}
	}
}

func TestBBSLoginShowsSession(t *testing.T) {
	f := newFakeSermo(t)
	d := bbsDeps(t, f, "")
	m := bbsModel(t, d, nil)
	l := newLogin(m.ctx, d, "", noSleep)
	m.push(l)
	if !strings.Contains(screenText(m), "[ENTER] Entrar") {
		t.Errorf("login hints:\n%s", screenText(m))
	}
	l.state, l.code.UserCode, l.code.VerificationURI = loginWaiting, "WXYZ-1234", f.URL+"/device"
	out := screenText(m)
	for _, want := range []string{">> Pidiendo un código ... [ OK ]", ">> Abre esta dirección", f.URL + "/device",
		"WXYZ-1234", ">> Esperando tu confirmación ...", "[O]Abrir  [C]opiar  [Esc] Cancelar"} {
		if !strings.Contains(out, want) {
			t.Errorf("login misses %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Clave") {
		t.Error("the login must not ask for a password")
	}
}

func TestBBSHintLineLeavesOutHintsThatDoNotFit(t *testing.T) {
	f := newFakeSermo(t)
	m := bbsModel(t, bbsDeps(t, f, ""), nil)
	line := strings.Split(screenText(m), "\n")[22]
	if !strings.Contains(line, "[?]Ayuda  [Q]Salir") || strings.Contains(line, "[V") {
		t.Errorf("hint line at 80 columns: %q", line)
	}
}
