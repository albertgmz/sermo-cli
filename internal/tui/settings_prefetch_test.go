package tui

import (
	"strings"
	"testing"

	"github.com/albertgmz/sermo-cli/internal/config"
)

func TestSettingsPrefetchCyclesAndSaves(t *testing.T) {
	f := newFakeSermo(t)
	d := testDeps(t, f, "")
	m := newTestModel(t, d, StartHome)
	m.push(newSettings(d))
	out := screenText(m)
	if !strings.Contains(out, "Precarga") || !strings.Contains(out, "más datos") {
		t.Fatalf("settings misses the prefetch row or its note:\n%s", out)
	}
	for range 4 {
		pressKey(m, "down")
	}
	for _, want := range []struct{ value, label string }{
		{config.PrefetchLight, "Ligera"}, {config.PrefetchFull, "Completa"}, {"", "Desactivada"},
	} {
		pressKey(m, "right")
		c, err := config.Load()
		if err != nil || c.Prefetch != want.value {
			t.Fatalf("saved prefetch = %q, %v, want %q", c.Prefetch, err, want.value)
		}
		if !strings.Contains(screenText(m), "◀ "+want.label+" ▶") {
			t.Fatalf("settings must show %q:\n%s", want.label, screenText(m))
		}
	}
	pressKey(m, "left")
	if c, _ := config.Load(); c.Prefetch != config.PrefetchFull {
		t.Fatalf("left from off must give full, saved = %q", c.Prefetch)
	}
}
