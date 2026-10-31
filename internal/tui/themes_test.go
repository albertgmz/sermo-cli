package tui

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/albertgmz/sermo-cli/internal/banner"
	"github.com/albertgmz/sermo-cli/internal/config"
	"github.com/albertgmz/sermo-cli/internal/theme"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// writeUserThemes writes theme files into the themes directory of the test
// config. Each file is a copy of the built-in malecon file with the edits.
// Each edit replaces the text before "=>" with the text after it.
func writeUserThemes(t *testing.T, files map[string][]string) string {
	t.Helper()
	base, err := os.ReadFile(filepath.Join("..", "theme", "builtin", "malecon.json"))
	if err != nil {
		t.Fatal(err)
	}
	dir, err := config.ThemesDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, edits := range files {
		s := string(base)
		for _, e := range edits {
			from, to, _ := strings.Cut(e, "=>")
			s = strings.Replace(s, from, to, 1)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestThemesLoadedAppliesSavedCustomThemeAndToastsProblems(t *testing.T) {
	f := newFakeSermo(t)
	d := testDeps(t, f, "")
	writeUserThemes(t, map[string][]string{
		"noche.json": {`"Malecón"=>"Noche"`},
		"roto.json":  {`"#eaf6ff"=>"#333333"`},
	})
	d.Config.Theme = "noche"
	m := newTestModel(t, d, StartHome)
	m.Update(tea.WindowSizeMsg{Width: 300, Height: 40})
	if d.Theme.ID != "habana-neon" {
		t.Fatalf("before the load the theme is %q, want the default", d.Theme.ID)
	}
	msgs := drive(m, m.Init(), wait)
	if !hasMsg[themesLoadedMsg](msgs) {
		t.Fatal("Init did not load the user themes")
	}
	if d.Theme.ID != "noche" || !d.Theme.Custom || d.Theme.Name != "Noche" {
		t.Fatalf("theme = %q custom %v, want the user theme noche", d.Theme.ID, d.Theme.Custom)
	}
	if want := "No se pudo cargar 1 tema: roto.json: el texto no se lee bien"; !strings.Contains(screenText(m), want) {
		t.Fatalf("no problem toast %q:\n%s", want, screenText(m))
	}
}

func TestThemesLoadedWithoutProblemsShowsNoToast(t *testing.T) {
	f := newFakeSermo(t)
	m := newTestModel(t, testDeps(t, f, ""), StartHome)
	msgs := drive(m, m.Init(), wait)
	if !hasMsg[themesLoadedMsg](msgs) || hasMsg[ui.ToastMsg](msgs) {
		t.Fatalf("messages = %v, want a load and no toast", msgs)
	}
}

func TestThemeProblemsToastUsesPlural(t *testing.T) {
	f := newFakeSermo(t)
	d := testDeps(t, f, "")
	d.Config.Locale = "en"
	writeUserThemes(t, map[string][]string{
		"a.json": {`"#93b6cf"=>"nope"`},
		"b.json": {`"name"=>"effect": "lava", "name"`},
	})
	m := newTestModel(t, d, StartHome)
	m.Update(tea.WindowSizeMsg{Width: 300, Height: 40})
	drive(m, m.Init(), wait)
	want := "2 themes did not load: a.json: a color is not in the #rrggbb format; b.json: the effect is unknown"
	if !strings.Contains(screenText(m), want) {
		t.Fatalf("no toast %q:\n%s", want, screenText(m))
	}
}

func TestSettingsCyclesIntoCustomTheme(t *testing.T) {
	f := newFakeSermo(t)
	d := testDeps(t, f, "")
	dir := writeUserThemes(t, map[string][]string{"zafiro.json": {`"Malecón"=>"Zafiro"`}})
	m := newTestModel(t, d, StartHome)
	drive(m, m.Init(), wait)
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})
	m.push(newSettings(d))
	pressKey(m, "h")
	if d.Theme.ID != "zafiro" {
		t.Fatalf("theme = %q, want the custom theme zafiro", d.Theme.ID)
	}
	if c, err := config.Load(); err != nil || c.Theme != "zafiro" {
		t.Fatalf("saved theme = %q, %v", c.Theme, err)
	}
	out := screenText(m)
	if !strings.Contains(out, "Zafiro (personalizado)") {
		t.Fatalf("settings misses the custom mark:\n%s", out)
	}
	if !strings.Contains(out, dir) {
		t.Fatalf("settings misses the themes folder %q:\n%s", dir, out)
	}
}

func TestSplashUsesThemeEffectAndFont(t *testing.T) {
	f := newFakeSermo(t)
	d := testDeps(t, f, "")
	d.Config.Effect = ""
	th := theme.ByID("")
	th.Effect, th.BannerFont = "fire", "standard"
	d.SetTheme(th)
	s := newSplash(d, rand.New(rand.NewPCG(1, 2)))
	if s.effect.Name != "fire" {
		t.Errorf("effect = %q, want the theme effect fire", s.effect.Name)
	}
	d.Config.Effect = "crt"
	if s := newSplash(d, rand.New(rand.NewPCG(1, 2))); s.effect.Name != "crt" {
		t.Errorf("effect = %q, want the config effect crt", s.effect.Name)
	}
	out := ansi.Strip(s.View("Foros Cuba", 100, 30))
	logo := banner.LinesWithFont("Foros Cuba", 90, "standard")
	if !strings.Contains(out, strings.TrimSpace(logo[1])) {
		t.Errorf("splash misses the standard font banner:\n%s", out)
	}
}
