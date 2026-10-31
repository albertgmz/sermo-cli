package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albertgmz/sermo-cli/internal/config"
	"github.com/albertgmz/sermo-cli/internal/i18n"
)

// writeUserLocales writes catalog files into the locales directory of the
// test config. The keys are paths like "pt/core.json".
func writeUserLocales(t *testing.T, files map[string]string) {
	t.Helper()
	dir, err := config.LocalesDir()
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { i18n.LoadDir(t.TempDir()) })
}

func TestUserLocalesLoadAtStartup(t *testing.T) {
	f := newFakeSermo(t)
	d := testDeps(t, f, "")
	writeUserLocales(t, map[string]string{
		"pt/core.json": `{"core.hotkey.forums": "Fóruns", "core.lang.pt": "Português"}`,
		"es/bad.json":  `{"core.hotkey.forums": `,
	})
	d.Config.Locale = "pt"
	m := newTestModel(t, d, StartHome)
	msgs := drive(m, m.Init(), wait)
	if i18n.Locale() != "pt" {
		t.Fatalf("Locale() = %q, want the user language of the config", i18n.Locale())
	}
	out := screenText(m)
	if !strings.Contains(out, "[F]óruns") || !strings.Contains(out, "[M]ensajes") {
		t.Errorf("the user text must show, with Spanish for the missing IDs:\n%s", out)
	}
	toast := i18n.Plural("locales.loadFailed", 1, map[string]any{"Files": "es/bad.json: " + i18n.T("locales.err.syntax")})
	if !hasToast(msgs, toast) {
		t.Errorf("no toast %q for the bad file: %v", toast, msgs)
	}
}

func TestSettingsListsUserLanguage(t *testing.T) {
	f := newFakeSermo(t)
	d := testDeps(t, f, "")
	writeUserLocales(t, map[string]string{"pt/core.json": `{"core.hotkey.forums": "Fóruns"}`})
	m := newTestModel(t, d, StartHome)
	drive(m, m.Init(), wait)
	m.push(newSettings(d))
	for range rowLanguage {
		pressKey(m, "down")
	}
	pressKey(m, "right")
	pressKey(m, "right")
	if i18n.Locale() != "pt" || d.Config.Locale != "pt" {
		t.Fatalf("locale = %q, config %q, want pt after es and en", i18n.Locale(), d.Config.Locale)
	}
	if !strings.Contains(screenText(m), "◀ português ▶") {
		t.Errorf("settings must name the user language:\n%s", screenText(m))
	}
	pressKey(m, "right")
	if i18n.Locale() != "es" {
		t.Errorf("the language row must cycle back to es, got %q", i18n.Locale())
	}
}
