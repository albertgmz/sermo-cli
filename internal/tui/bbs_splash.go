package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/albertgmz/sermo-cli/internal/banner"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// bbsSplashWidth is the largest width of the BBS splash box.
const bbsSplashWidth = 78

// bbsSplash draws the BBS welcome: a double box with the banner, the
// slogan and the facts of the site, and then the session lines that type
// out under it.
func (m *model) bbsSplash() string {
	s := m.d.Styles
	width := min(m.w, bbsSplashWidth)
	steps := m.bbsSteps(width)
	typed, done := crawl(steps, m.crawlTime(), m.d.Motion())
	after := []string{""}
	for _, l := range typed {
		after = append(after, m.paintStep(strings.ReplaceAll(l, "█", "_")))
	}
	after = append(after, "")
	if done {
		after = append(after, center(s.Accent.Render(i18n.T("bbs.splash.enter")), width))
	}
	logo := banner.Lines(m.site, width-8)
	head := m.splashBox(logo, width)
	if strings.Count(head, "\n")+1+len(after) > m.h {
		head = m.splashBox([]string{strings.ToUpper(m.site)}, width)
	}
	lines := append(strings.Split(head, "\n"), after...)
	pad := strings.Repeat(" ", (m.w-width)/2)
	top := max((m.h-len(lines))/2, 0)
	out := make([]string, m.h)
	for i := range out {
		l := ""
		if j := i - top; j >= 0 && j < len(lines) {
			l = pad + lines[j]
		}
		out[i] = fit(l, m.w)
	}
	return strings.Join(out, "\n")
}

func (m *model) crawlTime() float64 {
	if !m.d.Motion() {
		return staticTime
	}
	return m.d.AnimT
}

// bbsCrawlDone reports whether all session lines show.
func (m *model) bbsCrawlDone() bool {
	_, done := crawl(m.bbsSteps(min(m.w, bbsSplashWidth)), m.crawlTime(), m.d.Motion())
	return done
}

// splashBox is the double box with the banner, the slogan and the info box.
func (m *model) splashBox(logo []string, width int) string {
	s := m.d.Styles
	inner := width - 4
	body := []string{""}
	logoW := widest(logo)
	for _, l := range logo {
		body = append(body, center(s.Accent.Render(pad(l, logoW)), inner))
	}
	body = append(body, "")
	if m.siteDesc != "" {
		slogan := s.Muted.Render("-=[  ") + s.Text.Render(m.siteDesc) + s.Muted.Render("  ]=-")
		body = append(body, center(slogan, inner), "")
	}
	body = append(body, m.infoBox(inner)...)
	return ui.DoubleBox(s, width, append(body, ""))
}

// infoBox is the small box of real facts: the members, the visitors, the
// terminal size and the character set. A fact that did not load is left out.
func (m *model) infoBox(width int) []string {
	s := m.d.Styles
	var left, right []string
	if m.stats != nil {
		left = append(left, i18n.Tf("bbs.splash.registered", map[string]any{"Count": ui.Thousands(m.stats.Members)}))
	}
	if m.haveOnline {
		left = append(left, i18n.Tf("bbs.splash.online", map[string]any{"Count": ui.Thousands(m.online)}))
	}
	right = append(right, i18n.Tf("bbs.splash.terminal", map[string]any{"Width": m.w, "Height": m.h}), i18n.T("bbs.splash.charset"))
	w := width - 2
	col := (w - 6) / 2
	bar := s.Muted.Render("|")
	out := []string{" " + s.Muted.Render("."+strings.Repeat("-", w-2)+".")}
	for i := range max(len(left), len(right)) {
		l, r := "", ""
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		out = append(out, " "+bar+"  "+s.Text.Render(pad(l, col))+"  "+s.Text.Render(pad(r, w-6-col))+bar)
	}
	return append(out, " "+s.Muted.Render("'"+strings.Repeat("-", w-2)+"'"))
}

// bbsSteps are the session lines of the splash. Each line fills width
// cells. The line of the connection ends in a row of dots and "[ OK ]".
func (m *model) bbsSteps(width int) []crawlStep {
	connect := i18n.Tf("bbs.splash.connect", map[string]any{"Host": hostOf(m.d.Server)}) + " "
	ok := " " + i18n.T("bbs.ok")
	dots := max(width-4-ansi.StringWidth(connect)-ansi.StringWidth(ok), 3)
	steps := []crawlStep{{">> " + connect + strings.Repeat(".", dots) + ok, 50, 0.3}}
	if m.d.Me == nil {
		return append(steps, crawlStep{">> " + i18n.T("bbs.splash.guest"), 40, 0.3})
	}
	steps = append(steps, crawlStep{">> " + i18n.Tf("bbs.splash.member", map[string]any{"Name": m.d.Me.Username}), 40, 0.3})
	if m.haveCounters {
		unread := i18n.Tf("bbs.splash.unread", map[string]any{
			"Messages": i18n.Plural("bbs.splash.messages", m.counters.Conversations, nil),
			"Alerts":   i18n.Plural("bbs.splash.alerts", m.counters.Notifications, nil),
		})
		steps = append(steps, crawlStep{">> " + unread, 40, 0.3})
	}
	return steps
}

// paintStep colors a session line: the >> mark and [ OK ] get colors.
func (m *model) paintStep(line string) string {
	s := m.d.Styles
	ok := i18n.T("bbs.ok")
	mark, found := "", false
	if line, found = strings.CutPrefix(line, ">> "); found {
		mark = s.Accent.Render(">> ")
	}
	tail := ""
	if rest, found := strings.CutSuffix(line, ok); found {
		line, tail = rest, s.Success.Render(ok)
	}
	if line != "" {
		line = s.Text.Render(line)
	}
	return "  " + mark + line + tail
}

// center puts the text in the middle of width cells.
func center(text string, width int) string {
	return strings.Repeat(" ", max((width-ansi.StringWidth(text))/2, 0)) + text
}
