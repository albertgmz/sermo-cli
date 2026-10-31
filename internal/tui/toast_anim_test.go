package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/colorprofile"

	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

func TestToastSlidesInAndStops(t *testing.T) {
	f := newFakeSermo(t)
	d := testDeps(t, f, "")
	d.NoAnim = false
	m := newTestModel(t, d, StartHome)
	if m.animating() {
		t.Fatal("the home screen must be idle before the toast")
	}
	_, cmd := m.Update(ui.ToastMsg{Text: "Guardado", Kind: ui.ToastSuccess})
	if !m.animating() || !hasMsg[ui.AnimTickMsg](firstLevel(cmd)) {
		t.Fatal("a new toast must start the animation clock")
	}
	if m.toastOffset(20) <= 0 {
		t.Fatalf("the toast must start off its place, offset %d", m.toastOffset(20))
	}
	now := time.Now()
	for i := 0; i < 120 && m.animating(); i++ {
		now = now.Add(time.Second / maxFPS)
		m.Update(ui.AnimTickMsg{Time: now})
	}
	if m.animating() {
		t.Fatal("the toast must come to rest in 4 seconds of ticks")
	}
	if m.toastOffset(20) != 0 {
		t.Fatalf("a toast at rest must have offset 0, got %d", m.toastOffset(20))
	}
	if !strings.Contains(screenText(m), "Guardado") {
		t.Fatal("toast not shown at rest")
	}
}

func TestToastWithoutMotionShowsAtRest(t *testing.T) {
	f := newFakeSermo(t)
	plain := testDeps(t, f, "")
	plain.NoAnim = false
	plain.Profile = colorprofile.ASCII
	for name, d := range map[string]*ui.Deps{"no-anim": testDeps(t, f, ""), "plain": plain} {
		m := newTestModel(t, d, StartHome)
		_, cmd := m.Update(ui.ToastMsg{Text: "Guardado", Kind: ui.ToastSuccess})
		if m.toastOffset(20) != 0 {
			t.Fatalf("%s: the toast must show at rest at once", name)
		}
		if hasMsg[ui.AnimTickMsg](firstLevel(cmd)) {
			t.Fatalf("%s: a toast scheduled an AnimTickMsg", name)
		}
		if !strings.Contains(screenText(m), "Guardado") {
			t.Fatalf("%s: toast not shown", name)
		}
	}
}
