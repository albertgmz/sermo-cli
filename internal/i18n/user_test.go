package i18n

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// userDir writes the files into a temp directory. The keys are paths like
// "es/mine.json". LoadDir of an empty directory restores the embedded
// catalogs after the test.
func userDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		LoadDir(t.TempDir())
		SetLocale("es")
	})
	return dir
}

func TestUserFileOverridesOneID(t *testing.T) {
	dir := userDir(t, map[string]string{"es/mine.json": `{"render.spoiler": "[destripe]"}`})
	if problems := LoadDir(dir); len(problems) != 0 {
		t.Fatalf("problems: %v", problems)
	}
	if got := T("render.spoiler"); got != "[destripe]" {
		t.Errorf("T = %q, want the user text", got)
	}
	if got := T("render.imageNoAlt"); got == "render.imageNoAlt" {
		t.Error("the other IDs must stay")
	}
	LoadDir(t.TempDir())
	if got := T("render.spoiler"); got != "[spoiler]" {
		t.Errorf("after a load without user files T = %q", got)
	}
}

func TestUserFileAddsLanguageWithSpanishFallback(t *testing.T) {
	dir := userDir(t, map[string]string{"pt/core.json": `{"render.spoiler": "[revelação]", "render.attachments": {"one": "{{.Count}} anexo", "other": "{{.Count}} anexos"}}`})
	if problems := LoadDir(dir); len(problems) != 0 {
		t.Fatalf("problems: %v", problems)
	}
	if !slices.Equal(Locales(), []string{"es", "en", "pt"}) {
		t.Errorf("Locales() = %v", Locales())
	}
	SetLocale("pt")
	if Locale() != "pt" {
		t.Fatalf("Locale() = %q, want pt", Locale())
	}
	if got := T("render.spoiler"); got != "[revelação]" {
		t.Errorf("T = %q", got)
	}
	if got := Plural("render.attachments", 2, nil); got != "2 anexos" {
		t.Errorf("Plural = %q", got)
	}
	if got := Tf("render.quoteAuthor", map[string]any{"Author": "Ana"}); got != "Ana escribió:" {
		t.Errorf("a missing ID must fall back to Spanish, got %q", got)
	}
}

func TestBadUserFilesAreSkipped(t *testing.T) {
	dir := userDir(t, map[string]string{
		"es/good.json":   `{"render.spoiler": "[ok]"}`,
		"es/broken.json": `{"render.spoiler": `,
		"es/wrong.json":  `{"render.spoiler": 12}`,
		"no lang/a.json": `{"render.spoiler": "x"}`,
		"es/notes.txt":   "not a catalog",
	})
	problems := LoadDir(dir)
	files := make([]string, len(problems))
	for i, p := range problems {
		files[i] = p.File
		if !errors.Is(p.Err, ErrSyntax) && !errors.Is(p.Err, ErrLanguage) {
			t.Errorf("%s: unexpected error %v", p.File, p.Err)
		}
	}
	slices.Sort(files)
	if want := []string{"es/broken.json", "es/wrong.json", "no lang"}; !slices.Equal(files, want) {
		t.Errorf("problem files %v, want %v", files, want)
	}
	if got := T("render.spoiler"); got != "[ok]" {
		t.Errorf("the good file must load, T = %q", got)
	}
	if slices.Contains(Locales(), "no lang") {
		t.Error("a bad language must not show")
	}
}

func TestMissingUserDirKeepsEmbedded(t *testing.T) {
	if problems := LoadDir(filepath.Join(t.TempDir(), "none")); len(problems) != 0 {
		t.Fatalf("problems: %v", problems)
	}
	if !slices.Equal(Locales(), []string{"es", "en"}) {
		t.Errorf("Locales() = %v", Locales())
	}
}

func TestExportWritesEveryID(t *testing.T) {
	ids, err := catalogIDs()
	if err != nil {
		t.Fatal(err)
	}
	for _, lang := range []string{"en", "pt"} {
		dir := t.TempDir()
		written, skipped, err := Export(dir, lang)
		if err != nil || len(written) == 0 || len(skipped) != 0 {
			t.Fatalf("Export(%s) = %v, %v, %v", lang, written, skipped, err)
		}
		source := lang
		if lang == "pt" {
			source = "es" // a new language starts from the Spanish text
		}
		got := exportedIDs(t, filepath.Join(dir, lang))
		for id := range ids[source] {
			if !got[id] {
				t.Errorf("%s: export misses %s", lang, id)
			}
		}
		if len(got) != len(ids[source]) {
			t.Errorf("%s: export has %d IDs, want %d", lang, len(got), len(ids[source]))
		}
	}
}

func TestExportKeepsExistingFiles(t *testing.T) {
	dir := t.TempDir()
	mine := filepath.Join(dir, "es", "core.json")
	if err := os.MkdirAll(filepath.Dir(mine), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mine, []byte(`{"x": "mine"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, skipped, err := Export(dir, "es")
	if err != nil || !slices.Equal(skipped, []string{mine}) {
		t.Fatalf("skipped %v, err %v", skipped, err)
	}
	if b, _ := os.ReadFile(mine); string(b) != `{"x": "mine"}` {
		t.Errorf("export changed a user file: %s", b)
	}
	if _, _, err := Export(dir, "../x"); !errors.Is(err, ErrLanguage) {
		t.Errorf("bad language err = %v", err)
	}
}

func exportedIDs(t *testing.T, dir string) map[string]bool {
	t.Helper()
	paths, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	out := map[string]bool{}
	for _, p := range paths {
		buf, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(buf, &m); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		for id := range m {
			out[id] = true
		}
	}
	return out
}

func TestLanguageName(t *testing.T) {
	if got := LanguageName("es"); got != T("core.lang.es") {
		t.Errorf("LanguageName(es) = %q", got)
	}
	if got := LanguageName("pt"); got != "português" {
		t.Errorf("LanguageName(pt) = %q", got)
	}
}
