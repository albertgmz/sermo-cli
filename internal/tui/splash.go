package tui

import (
	"math"
	"math/rand/v2"
	"net/url"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/albertgmz/sermo-cli/internal/banner"
	"github.com/albertgmz/sermo-cli/internal/fx"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// staticTime is the effect time of the one frame for reduced motion.
const staticTime = 2.5

// splash is the full screen welcome: an effect with the site banner on a
// dark plate and a modem handshake that types out under the banner.
type splash struct {
	d      *ui.Deps
	effect fx.Effect
}

func newSplash(d *ui.Deps, r *rand.Rand) *splash {
	e, ok := fx.ByName(d.Config.Effect, d.Theme.Splash)
	if !ok {
		e = fx.Random(r, d.Theme.Splash)
	}
	return &splash{d: d, effect: e}
}

// Animating reports whether the splash needs animation ticks.
func (s *splash) Animating() bool { return s.d.Animate() }

func (s *splash) time() float64 {
	if !s.d.Animate() {
		return staticTime
	}
	return s.d.AnimT
}

// View draws the splash for the site name in width x height cells.
func (s *splash) View(site string, width, height int) string {
	t := s.time()
	steps := handshake(hostOf(s.d.Server), site)
	lines, done := crawl(steps, t, s.d.Animate())
	prompt := ""
	if done && (!s.d.Animate() || math.Mod(t, 1.2) < 0.85) {
		prompt = i18n.T("core.splash.anyKey")
	}
	mask := splashMask(site, steps, lines, prompt, width, height)
	return fx.Render(s.effect, width, height, t, mask, fx.RenderOptions{ASCII: s.d.Plain()})
}

// crawlStep is one line of the handshake. A cps of 0 shows the line at
// once. The pause is the wait in seconds after the line.
type crawlStep struct {
	text  string
	cps   float64
	pause float64
}

func handshake(host, site string) []crawlStep {
	return []crawlStep{
		{"ATZ", 12, 0.3},
		{"OK", 0, 0.4},
		{"ATDT " + host, 16, 0.9},
		{"CONNECT 115200", 0, 0.5},
		{i18n.Tf("core.splash.welcome", map[string]any{"Site": site}), 40, 0.3},
	}
}

// crawl returns the visible text of each step at time t and reports whether
// all steps are complete. The line that types shows a cursor block. When
// animate is false, all lines show complete.
func crawl(steps []crawlStep, t float64, animate bool) ([]string, bool) {
	out := make([]string, len(steps))
	if !animate {
		for i, st := range steps {
			out[i] = st.text
		}
		return out, true
	}
	at := 0.4
	for i, st := range steps {
		n := len([]rune(st.text))
		dur := 0.0
		if st.cps > 0 {
			dur = float64(n) / st.cps
		}
		if t < at+dur {
			shown := int(math.Max(0, (t-at)*st.cps))
			out[i] = string([]rune(st.text)[:min(shown, n)]) + cursor(t)
			return out, false
		}
		out[i] = st.text
		at += dur + st.pause
		if t < at {
			if i+1 < len(out) {
				out[i+1] = cursor(t)
			}
			return out, false
		}
	}
	return out, true
}

func cursor(t float64) string {
	if math.Mod(t, 0.6) < 0.3 {
		return "█"
	}
	return " "
}

// splashMask builds the mask lines: a double line box around the banner,
// the handshake lines and the prompt. All lines get the same width so that
// the plate does not change size while the text types.
func splashMask(site string, steps []crawlStep, lines []string, prompt string, width, height int) []string {
	crawlW := ansi.StringWidth(i18n.T("core.splash.anyKey"))
	for _, st := range steps {
		crawlW = max(crawlW, ansi.StringWidth(st.text)+1)
	}
	logo := banner.Lines(site, width-10)
	if len(logo)+len(steps)+8 > height {
		logo = []string{strings.ToUpper(site)}
	}
	inner := min(width-6, max(widest(logo), crawlW)+4)
	center := func(l string, w int) string { return pad(strings.Repeat(" ", max((inner-w)/2, 0))+l, inner) }
	body := []string{""}
	for _, l := range logo {
		body = append(body, center(l, widest(logo)))
	}
	body = append(body, "")
	for _, l := range lines {
		body = append(body, center(l, crawlW))
	}
	body = append(body, "", center(prompt, ansi.StringWidth(prompt)), "")
	out := []string{"╔" + strings.Repeat("═", inner) + "╗"}
	for _, l := range body {
		out = append(out, "║"+pad(l, inner)+"║")
	}
	return append(out, "╚"+strings.Repeat("═", inner)+"╝")
}

func widest(lines []string) int {
	w := 0
	for _, l := range lines {
		w = max(w, ansi.StringWidth(l))
	}
	return w
}

func pad(s string, w int) string {
	s = ansi.Truncate(s, w, "")
	return s + strings.Repeat(" ", w-ansi.StringWidth(s))
}

// hostOf returns the host name of a server URL, or the URL when it does not
// parse.
func hostOf(server string) string {
	u, err := url.Parse(server)
	if err != nil || u.Host == "" {
		return server
	}
	return u.Host
}
