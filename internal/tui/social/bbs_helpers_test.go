package social

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/albertgmz/sermo-cli/internal/config"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// The BBS chrome takes 5 of the 24 lines of the window.
const bbsW, bbsH = 80, 19

// bbsDeps returns Deps like testDeps with the BBS interface.
func bbsDeps(t *testing.T, f *fakeSermo, member bool) *ui.Deps {
	t.Helper()
	d := testDeps(t, f, member)
	d.Config.Interface = config.InterfaceBBS
	return d
}

// startBBS runs a screen in the content area of a window of 80x24.
func startBBS(t *testing.T, s ui.Screen) *harness {
	t.Helper()
	h := &harness{t: t, s: s, w: bbsW, h: bbsH}
	h.send(tea.WindowSizeMsg{Width: h.w, Height: h.h})
	h.run(s.Init())
	return h
}

func (h *harness) commander() ui.Commander {
	h.t.Helper()
	c, ok := h.s.(ui.Commander)
	if !ok {
		h.t.Fatalf("%T is not a ui.Commander", h.s)
	}
	return c
}

// command runs the screen command name with arg like the BBS prompt.
func (h *harness) command(name, arg string) {
	h.t.Helper()
	for _, c := range h.commander().Commands() {
		if c.Run != nil && strings.EqualFold(c.Name, name) {
			h.run(c.Run(arg))
			return
		}
	}
	h.t.Fatalf("no command %q in %v", name, h.hints())
}

// hasCommand reports whether the screen runs the command name.
func (h *harness) hasCommand(name string) bool {
	for _, c := range h.commander().Commands() {
		if c.Run != nil && strings.EqualFold(c.Name, name) {
			return true
		}
	}
	return false
}

// hints returns the labels of the hint line.
func (h *harness) hints() string {
	var out []string
	for _, c := range h.commander().Commands() {
		if c.Label != "" {
			out = append(out, c.Label)
		}
	}
	return strings.Join(out, "  ")
}

// wantHints checks the labels of the hint line and the prompt.
func (h *harness) wantHints(prompt string, labels ...string) {
	h.t.Helper()
	got := h.hints()
	for _, l := range labels {
		if !strings.Contains(got, l) {
			h.t.Errorf("hints %q miss %q", got, l)
		}
	}
	if p := h.commander().Prompt(); p != prompt {
		h.t.Errorf("prompt = %q, want %q", p, prompt)
	}
}

// wantFit checks that each line of the view is exactly the width and
// that the view has no more lines than the height.
func (h *harness) wantFit() {
	h.t.Helper()
	lines := strings.Split(h.s.View(h.w, h.h), "\n")
	if len(lines) > h.h {
		h.t.Errorf("view has %d lines, want %d or less", len(lines), h.h)
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w != h.w && strings.TrimSpace(ansi.Strip(l)) != "" {
			h.t.Errorf("line %d is %d cells, want %d: %q", i, w, h.w, ansi.Strip(l))
		}
	}
}

// capture writes the screen with a hint line and a prompt line to the
// directory in $SERMO_BBSCAP, at 80x24 and at 120x40. Without the
// variable it does nothing.
func (h *harness) capture(name string) {
	h.t.Helper()
	dir := os.Getenv("SERMO_BBSCAP")
	if dir == "" {
		return
	}
	for _, size := range [][2]int{{120, 40}, {80, 24}} {
		h.w, h.h = size[0], size[1]-5
		h.send(tea.WindowSizeMsg{Width: h.w, Height: h.h})
		out := h.screen() + "\n " + h.hints() + "\n " + h.commander().Prompt() + " > _\n"
		file := filepath.Join(dir, "social-"+name+"-"+itoa(size[0])+"x"+itoa(size[1])+".txt")
		if err := os.WriteFile(file, []byte(out), 0o600); err != nil {
			h.t.Fatal(err)
		}
	}
}
