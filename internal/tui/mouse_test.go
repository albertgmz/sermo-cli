package tui

import (
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/config"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/forum"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
	"github.com/albertgmz/sermo-cli/internal/tui/ui/zonetest"
)

// click clicks the zone id of the current frame of m and runs the result.
func click(t *testing.T, m *model, id string) []tea.Msg {
	t.Helper()
	msg := zonetest.Click(t, m.d.Zones, m.render(), id)
	_, cmd := m.Update(msg)
	return drive(m, cmd, wait)
}

func TestMouseSettingTurnsZonesOff(t *testing.T) {
	f := newFakeSermo(t)
	d := testDeps(t, f, "")
	m := newTestModel(t, d, StartHome)
	if v := m.View(); v.MouseMode != tea.MouseModeCellMotion || !zonetest.HasMarkers(m.render()) {
		t.Fatalf("the mouse must be on by default: mode %v", v.MouseMode)
	}
	drive(m, m.push(newSettings(d)), wait)
	s := m.top().(*settings)
	s.list.Select(rowMouse)
	if !strings.Contains(screenText(m), i18n.T("core.settings.mouse")) {
		t.Fatal("settings must show the mouse row")
	}
	drive(m, pressKey(m, "right"), wait)
	c, err := config.Load()
	if err != nil || c.Mouse != config.MouseOff {
		t.Fatalf("config mouse = %q, %v", c.Mouse, err)
	}
	if v := m.View(); v.MouseMode != tea.MouseModeNone || zonetest.HasMarkers(m.render()) {
		t.Fatalf("the mouse must be off: mode %v", v.MouseMode)
	}
	drive(m, pressKey(m, "right"), wait)
	if v := m.View(); v.MouseMode != tea.MouseModeCellMotion || !zonetest.HasMarkers(m.render()) {
		t.Fatal("the mouse must be on again")
	}
}

func TestMouseOffFromConfigAtStart(t *testing.T) {
	f := newFakeSermo(t)
	d := testDeps(t, f, "")
	d.Config.Mouse = config.MouseOff
	m := newTestModel(t, d, StartHome)
	if v := m.View(); v.MouseMode != tea.MouseModeNone || zonetest.HasMarkers(m.render()) {
		t.Fatalf("mouse off in the config: mode %v", v.MouseMode)
	}
}

func TestClickHotkeyOpensSection(t *testing.T) {
	f := newFakeSermo(t)
	m := newTestModel(t, testDeps(t, f, ""), StartHome)
	click(t, m, "app.key.b")
	if got := m.top().Title(); got != i18n.T("social.search.title") {
		t.Fatalf("top = %q, want the search", got)
	}
	click(t, m, "app.key.n")
	if _, ok := m.top().(*login); !ok {
		t.Fatalf("a guest must get the sign-in, top = %T", m.top())
	}
}

func TestClickAlertsBadgeOpensNotifications(t *testing.T) {
	f := newFakeSermo(t)
	m := newTestModel(t, testDeps(t, f, testToken), StartHome)
	drive(m, m.Init(), wait)
	click(t, m, "app.alerts")
	if got := m.top().Title(); got != i18n.T("social.notif.title") {
		t.Fatalf("top = %q, want the notifications", got)
	}
	click(t, m, "app.messages")
	if got := m.top().Title(); got != i18n.T("social.conv.title") {
		t.Fatalf("top = %q, want the conversations", got)
	}
}

func TestClickSettingsArrowChangesThemeAndSaves(t *testing.T) {
	f := newFakeSermo(t)
	d := testDeps(t, f, "")
	m := newTestModel(t, d, StartHome)
	drive(m, m.push(newSettings(d)), wait)
	s := m.top().(*settings)
	click(t, m, s.zid+"next.0")
	c, err := config.Load()
	if err != nil || c.Theme != "ambar-crt" || d.Theme.ID != "ambar-crt" {
		t.Fatalf("config theme = %q, %v", c.Theme, err)
	}
	click(t, m, s.zid+"prev.0")
	if d.Theme.ID == "ambar-crt" {
		t.Fatal("the left arrow must go back")
	}
	click(t, m, s.list.RowZone(rowLanguage))
	if s.list.Cursor() != rowLanguage {
		t.Fatalf("a click on a row must select it, cursor = %d", s.list.Cursor())
	}
	click(t, m, s.zid+"next."+strconv.Itoa(rowMotion))
	if s.list.Cursor() != rowMotion || !d.Config.ReducedMotion {
		t.Fatal("an arrow must select its row and change it")
	}
}

func TestClickBBSSettingsArrowAndHint(t *testing.T) {
	f := newFakeSermo(t)
	d := bbsDeps(t, f, "")
	m := bbsModel(t, d, newSettings(d))
	s := m.top().(*settings)
	click(t, m, s.zid+"next.0")
	if d.Theme.ID != "ambar-crt" {
		t.Fatalf("theme = %q after a click on >", d.Theme.ID)
	}
	// The hint "[<] Anterior" runs the command <.
	click(t, m, "app.hint.2.0")
	if d.Theme.ID == "ambar-crt" {
		t.Fatal("the hint < must give the previous theme")
	}
	click(t, m, s.list.RowZone(2))
	if s.list.Cursor() != 2 {
		t.Fatalf("cursor = %d, want 2", s.list.Cursor())
	}
	before := m.top()
	click(t, m, "app.hint.0.0")
	if m.top() != before || m.input != "" {
		t.Fatal("the hint ## must only keep the prompt")
	}
}

func TestClickBBSGlobalHintRunsCommand(t *testing.T) {
	f := newFakeSermo(t)
	d := bbsDeps(t, f, "")
	m := bbsModel(t, d, noCommands{forum.NewNodes(d)})
	// A screen without commands has the global hints. The hint 3 is [B]uscar.
	click(t, m, "app.hint.3.0")
	if got := m.top().Title(); got != i18n.T("social.search.title") {
		t.Fatalf("the hint B must open the search, top = %q", got)
	}
}

func TestClickContinuesSplash(t *testing.T) {
	for _, bbs := range []bool{false, true} {
		f := newFakeSermo(t)
		d := testDeps(t, f, "")
		if bbs {
			d.Config.Interface = config.InterfaceBBS
		}
		m := newTestModel(t, d, StartSplash)
		m.Update(tea.MouseClickMsg{X: 5, Y: 5, Button: tea.MouseLeft})
		if m.splash == nil {
			t.Fatal("a press must not end the splash")
		}
		m.Update(zonetest.At(5, 5))
		if m.splash != nil {
			t.Fatalf("bbs %v: a click must end the splash", bbs)
		}
	}
}

func TestClickDismissesToast(t *testing.T) {
	for _, bbs := range []bool{false, true} {
		f := newFakeSermo(t)
		d := testDeps(t, f, "")
		if bbs {
			d.Config.Interface = config.InterfaceBBS
		}
		m := newTestModel(t, d, StartHome)
		m.Update(ui.ToastMsg{Text: "Guardado", Kind: ui.ToastSuccess})
		m.slide.stop()
		x, y, _, _ := m.toastRect()
		m.Update(zonetest.At(x+1, y))
		if m.toast != nil {
			t.Fatalf("bbs %v: a click on the toast must hide it", bbs)
		}
	}
}

func TestClickOnHintLabelWithoutKeyDoesNothing(t *testing.T) {
	f := newFakeSermo(t)
	d := bbsDeps(t, f, "")
	fc := newFakeCommander("", "R")
	m := bbsModel(t, d, fc)
	// The label "[]prueba" of the command "" has no key in its brackets.
	click(t, m, "app.hint.0.0")
	if len(fc.ran) != 0 {
		t.Fatalf("a label without a key must not run, ran %q", fc.ran)
	}
	click(t, m, "app.hint.1.0")
	if len(fc.ran) != 1 || fc.ran[0] != "R:" {
		t.Fatalf("the hint R must run R, ran %q", fc.ran)
	}
}

func TestClickAnswersNoToQuit(t *testing.T) {
	f := newFakeSermo(t)
	m := newTestModel(t, testDeps(t, f, testToken), StartHome)
	drive(m, m.Init(), wait)
	drive(m, m.push(newSettings(m.d)), wait)
	pressKey(m, "q")
	if !m.confirmQuit {
		t.Fatal("q off home must ask")
	}
	m.Update(zonetest.At(1, 0))
	if m.confirmQuit {
		t.Fatal("a click must answer no")
	}
	if _, ok := m.top().(*settings); !ok {
		t.Fatalf("a click on the quit question must do nothing else, top = %T", m.top())
	}
}
