package theme

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validFile returns a valid theme file with the name and the extra fields.
// Each value in edits replaces the text before "=>" with the text after it.
func validFile(name string, edits ...string) string {
	s := `{
  "name": "` + name + `",
  "colors": {"text": "#eaf6ff", "muted": "#93b6cf", "accent": "#ff8a5b", "error": "#ff5e62",
             "success": "#4fe3c1", "warning": "#ffc15e", "selection": "#12355b"},
  "gradients": {"border": ["#0b4f8a", "#ff8a5b"], "title": ["#ffb26b", "#ff6f91"],
                "highlight": ["#4fc3f7", "#ff8a5b"], "splash": ["#031a33", "#1fa2d6", "#ff6f91"]}
}`
	for _, e := range edits {
		from, to, _ := strings.Cut(e, "=>")
		s = strings.Replace(s, from, to, 1)
	}
	return s
}

// withFields returns a valid theme file with more top level fields.
func withFields(name, fields string) string {
	return validFile(name, "}\n}=>}, "+fields+"\n}")
}

func writeThemes(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func ids(s Set) []string {
	var out []string
	for _, th := range s.All() {
		out = append(out, th.ID)
	}
	return out
}

func TestLoadAddsUserThemesSortedByName(t *testing.T) {
	dir := writeThemes(t, map[string]string{
		"zeta.json":   validFile("Atardecer"),
		"alpha.json":  withFields("Vedado", `"border": "rounded", "bannerFont": "slant", "effect": "fire"`),
		"notes.txt":   "not a theme",
		"README.json": validFile("Upper"),
	})
	s, problems := Load(dir)
	if len(problems) != 1 || problems[0].File != "README.json" || !errors.Is(problems[0].Err, ErrID) {
		t.Fatalf("problems = %v, want one ErrID for README.json", problems)
	}
	want := []string{"habana-neon", "ambar-crt", "fosforo-verde", "malecon", "zeta", "alpha"}
	if got := ids(s); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("IDs = %v, want %v", got, want)
	}
	if !s.ByID("zeta").Custom || s.ByID("habana-neon").Custom {
		t.Error("only user themes must be custom")
	}
}

func TestLoadReadsOptionalFields(t *testing.T) {
	s, _ := Load(writeThemes(t, map[string]string{
		"alpha.json": withFields("Vedado", `"border": "rounded", "bannerFont": "slant", "effect": "fire"`),
		"zeta.json":  validFile("Atardecer"),
	}))
	if a := s.ByID("alpha"); a.BorderStyle != "rounded" || a.BannerFont != "slant" || a.Effect != "fire" {
		t.Errorf("alpha options = %q %q %q", a.BorderStyle, a.BannerFont, a.Effect)
	}
	if z := s.ByID("zeta"); z.BorderStyle != "double" || z.BannerFont != "" || z.Effect != "" {
		t.Errorf("zeta options = %q %q %q, want the defaults", z.BorderStyle, z.BannerFont, z.Effect)
	}
}

func TestLoadOverridesBuiltin(t *testing.T) {
	s, problems := Load(writeThemes(t, map[string]string{"malecon.json": validFile("Mi Malecón")}))
	if len(problems) != 0 {
		t.Fatalf("problems = %v", problems)
	}
	want := []string{"habana-neon", "ambar-crt", "fosforo-verde", "malecon"}
	if got := ids(s); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("IDs = %v, want %v", got, want)
	}
	if m := s.ByID("malecon"); m.Name != "Mi Malecón" || !m.Custom {
		t.Errorf("malecon = %q custom %v, want the user file", m.Name, m.Custom)
	}
	if ByID("malecon").Name != "Malecón" {
		t.Error("Load changed the package built-ins")
	}
}

func TestLoadReportsBadFilesAndKeepsTheOthers(t *testing.T) {
	bad := map[string]struct {
		data string
		want error
	}{
		"bad-hex.json":       {validFile("A", `"#ff8a5b", "error"=>"#ff8a5", "error"`), ErrColor},
		"no-text.json":       {validFile("B", `"text": "#eaf6ff", =>`), ErrMissing},
		"no-name.json":       {validFile(""), ErrMissing},
		"one-stop.json":      {validFile("C", `["#0b4f8a", "#ff8a5b"]=>["#0b4f8a"]`), ErrGradient},
		"bad-effect.json":    {withFields("D", `"effect": "lava"`), ErrEffect},
		"bad-font.json":      {withFields("E", `"bannerFont": "comic"`), ErrFont},
		"bad-border.json":    {withFields("F", `"border": "wavy"`), ErrBorder},
		"Bad_ID.json":        {validFile("G"), ErrID},
		"broken.json":        {`{"name": `, ErrSyntax},
		"unknown-field.json": {withFields("H", `"colour": "red"`), ErrSyntax},
		"dark-text.json":     {validFile("I", `"#eaf6ff"=>"#333333"`), ErrContrast},
		"dark-muted.json":    {validFile("J", `"#93b6cf"=>"#444444"`), ErrContrast},
		"big.json":           {validFile("K", `"name"=>"`+strings.Repeat(" ", 64<<10)+`"name"`), ErrTooLarge},
	}
	files := map[string]string{"good.json": validFile("Bueno")}
	for name, b := range bad {
		files[name] = b.data
	}
	s, problems := Load(writeThemes(t, files))
	if len(problems) != len(bad) {
		t.Errorf("got %d problems, want %d: %v", len(problems), len(bad), problems)
	}
	for _, p := range problems {
		b, ok := bad[p.File]
		if !ok {
			t.Errorf("unexpected problem %v", p)
			continue
		}
		if !errors.Is(p.Err, b.want) {
			t.Errorf("%s: error %v, want %v", p.File, p.Err, b.want)
		}
	}
	if got := ids(s); len(got) != 5 || got[4] != "good" {
		t.Errorf("IDs = %v, want the built-ins and good", got)
	}
}

func TestLoadMissingDirHasNoProblems(t *testing.T) {
	s, problems := Load(filepath.Join(t.TempDir(), "themes"))
	if len(problems) != 0 {
		t.Errorf("problems = %v", problems)
	}
	if len(s.All()) != len(All()) {
		t.Errorf("got %d themes, want the %d built-ins", len(s.All()), len(All()))
	}
}

func TestZeroSetHoldsTheBuiltins(t *testing.T) {
	var s Set
	if len(s.All()) != len(All()) || s.ByID("nope").ID != "habana-neon" {
		t.Errorf("zero Set = %v", ids(s))
	}
}
