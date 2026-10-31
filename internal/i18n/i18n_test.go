package i18n

import (
	"sort"
	"testing"
)

func withLocale(t *testing.T, lang string) {
	t.Helper()
	SetLocale(lang)
	t.Cleanup(func() { SetLocale("es") })
}

func TestDefaultLocaleIsSpanish(t *testing.T) {
	if got := Locale(); got != "es" {
		t.Fatalf("Locale() = %q, want es", got)
	}
	if got := T("render.spoiler"); got != "[spoiler]" {
		t.Fatalf("T = %q", got)
	}
	if got := Tf("render.quoteAuthor", map[string]any{"Author": "Ana"}); got != "Ana escribió:" {
		t.Fatalf("Tf = %q", got)
	}
}

func TestSwitchToEnglish(t *testing.T) {
	withLocale(t, "en")
	if got := Locale(); got != "en" {
		t.Fatalf("Locale() = %q, want en", got)
	}
	if got := Tf("render.quoteAuthor", map[string]any{"Author": "Ana"}); got != "Ana wrote:" {
		t.Fatalf("Tf = %q", got)
	}
}

func TestUnknownLocaleFallsBackToSpanish(t *testing.T) {
	withLocale(t, "en")
	SetLocale("fr")
	if got := Locale(); got != "es" {
		t.Fatalf("Locale() = %q, want es", got)
	}
	if got := Tf("render.quoteAuthor", map[string]any{"Author": "Ana"}); got != "Ana escribió:" {
		t.Fatalf("Tf = %q", got)
	}
}

func TestTemplateData(t *testing.T) {
	got := Tf("errors.rateLimited", map[string]any{"Seconds": 30})
	if got != "Vas muy rápido. Espera 30 s y vuelve a intentarlo." {
		t.Fatalf("Tf = %q", got)
	}
}

func TestPlural(t *testing.T) {
	cases := []struct {
		lang  string
		count int
		want  string
	}{
		{"es", 1, "1 adjunto"},
		{"es", 3, "3 adjuntos"},
		{"es", 0, "0 adjuntos"},
		{"es", 1000000, "1000000 adjuntos"},
		{"en", 1, "1 attachment"},
		{"en", 2, "2 attachments"},
	}
	for _, c := range cases {
		withLocale(t, c.lang)
		if got := Plural("render.attachments", c.count, nil); got != c.want {
			t.Errorf("%s Plural(%d) = %q, want %q", c.lang, c.count, got, c.want)
		}
	}
}

func TestPluralKeepsCallerData(t *testing.T) {
	data := map[string]any{"Other": "x"}
	_ = Plural("render.attachments", 2, data)
	if _, ok := data["Count"]; ok {
		t.Fatal("Plural changed the caller map")
	}
}

func TestMissingIDReturnsID(t *testing.T) {
	for _, lang := range []string{"es", "en"} {
		withLocale(t, lang)
		if got := T("no.such.id"); got != "no.such.id" {
			t.Errorf("%s T = %q", lang, got)
		}
		if got := Plural("no.such.id", 2, nil); got != "no.such.id" {
			t.Errorf("%s Plural = %q", lang, got)
		}
	}
}

func TestCatalogParity(t *testing.T) {
	ids, err := catalogIDs()
	if err != nil {
		t.Fatal(err)
	}
	es, en := ids["es"], ids["en"]
	if len(es) == 0 || len(en) == 0 {
		t.Fatalf("empty catalogs: es=%d en=%d", len(es), len(en))
	}
	if missing := absent(es, en); len(missing) > 0 {
		t.Errorf("IDs in es but not in en: %v", missing)
	}
	if missing := absent(en, es); len(missing) > 0 {
		t.Errorf("IDs in en but not in es: %v", missing)
	}
}

func absent(from, in map[string]bool) []string {
	var out []string
	for id := range from {
		if !in[id] {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}
