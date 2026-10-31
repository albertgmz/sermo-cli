package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albertgmz/sermo-cli/internal/config"
	"github.com/albertgmz/sermo-cli/internal/i18n"
)

func TestLocalesExportWritesFiles(t *testing.T) {
	setup(t)
	t.Cleanup(func() { i18n.LoadDir(t.TempDir()) })
	dir, _ := config.LocalesDir()
	r := run(t, testEnv(), "locales", "export", "pt")
	if r.runErr != nil {
		t.Fatalf("err = %v, stderr %s", r.runErr, r.err)
	}
	core := filepath.Join(dir, "pt", "core.json")
	if _, err := os.Stat(core); err != nil {
		t.Fatalf("no %s: %v", core, err)
	}
	for _, want := range []string{i18n.Tf("cli.locales.export.written", map[string]any{"File": core}), i18n.T("cli.locales.export.done")} {
		if !strings.Contains(r.out, want) {
			t.Errorf("output has no %q:\n%s", want, r.out)
		}
	}
	r = run(t, testEnv(), "locales", "export", "pt")
	if !strings.Contains(r.out, i18n.Tf("cli.locales.export.skipped", map[string]any{"File": core})) {
		t.Errorf("a second export must keep the files:\n%s", r.out)
	}
}

func TestLocalesExportDefaultsToCurrentLanguage(t *testing.T) {
	setup(t)
	dir, _ := config.LocalesDir()
	if r := run(t, testEnv(), "--lang", "en", "locales", "export"); r.runErr != nil {
		t.Fatalf("err = %v", r.runErr)
	}
	if _, err := os.Stat(filepath.Join(dir, "en", "cli.json")); err != nil {
		t.Errorf("no English export: %v", err)
	}
}

func TestLocalesExportRejectsBadLanguage(t *testing.T) {
	setup(t)
	r := run(t, testEnv(), "locales", "export", "../x")
	if r.runErr == nil || !strings.Contains(r.runErr.Error(), i18n.Tf("cli.locales.export.badLanguage", map[string]any{"Lang": "../x"})) {
		t.Errorf("err = %v", r.runErr)
	}
}

func TestCommandsUseUserTranslations(t *testing.T) {
	setup(t)
	t.Cleanup(func() { i18n.LoadDir(t.TempDir()) })
	dir, _ := config.LocalesDir()
	if err := os.MkdirAll(filepath.Join(dir, "pt"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pt", "cli.json"), []byte(`{"cli.locales.export.done": "Pronto."}`), 0o600); err != nil {
		t.Fatal(err)
	}
	r := run(t, testEnv(), "--lang", "pt", "locales", "export", "it")
	if r.runErr != nil || !strings.Contains(r.out, "Pronto.") {
		t.Errorf("the user language must apply to commands: err %v, out:\n%s", r.runErr, r.out)
	}
}
