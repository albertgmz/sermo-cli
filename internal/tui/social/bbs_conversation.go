package social

import (
	"context"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// convHead is the number of lines of the fixed head of the BBS reader.
const convHead = 4

// routeEditors gives msg to the open editor of either kind.
func (c *conversation) routeEditors(msg tea.Msg) (tea.Cmd, bool) {
	if cmd, ok := routeEditor(&c.ed, msg); ok {
		return cmd, true
	}
	return routeLineEditor(&c.led, msg)
}

// textWidth is the width of the rendered messages.
func (c *conversation) textWidth() int {
	if c.d.BBS() {
		return c.w
	}
	return ui.ReadingWidth(c.w)
}

// Prompt is the label of the BBS prompt.
func (c *conversation) Prompt() string {
	if c.led != nil {
		return i18n.T("bbs.social.editor.prompt")
	}
	return i18n.T("bbs.social.conv.reading")
}

// Commands are the BBS commands of the reader.
func (c *conversation) Commands() []ui.Command {
	if c.led != nil {
		return editorHint()
	}
	if c.msgs == nil {
		return []ui.Command{back()}
	}
	return []ui.Command{
		{Name: "", Label: c.counter()},
		{Name: "", Label: i18n.T("bbs.social.conv.next"), Run: func(string) tea.Cmd { return c.step(1) }},
		{Name: "P", Label: i18n.T("bbs.social.conv.prev"), Run: func(string) tea.Cmd { return c.step(-1) }},
		{Name: "R", Label: i18n.T("bbs.social.conv.reply"), Run: func(string) tea.Cmd { return c.reply("") }},
		back(),
		{Name: "C", Label: i18n.T("bbs.social.conv.quote"), Run: c.quoteNumber},
		{Name: "#", Label: i18n.T("bbs.social.conv.goTo"), Run: c.goTo},
		{Name: "O", Label: i18n.T("bbs.social.conv.link"), Run: c.openLink},
		{Name: "<", Run: func(string) tea.Cmd { return c.turn(c.page - 1) }},
		{Name: ">", Run: func(string) tea.Cmd { return c.turn(c.page + 1) }},
	}
}

// base is the number of the messages before the shown page.
func (c *conversation) base() int { return (c.page - 1) * pageSize }

// counter is the hint like "Mensajes 1-2 de 39" for the shown messages.
func (c *conversation) counter() string {
	first, last := c.cur, c.cur
	for i, at := range c.starts {
		if at < c.off+c.area() {
			last = i
		}
	}
	return i18n.Tf("bbs.social.conv.counter", map[string]any{
		"A": c.base() + first + 1, "B": c.base() + max(last, first) + 1, "N": max(c.total, len(c.msgs)),
	})
}

// area is the number of lines for the messages under the head.
func (c *conversation) area() int { return max(c.h-convHead, 1) }

// step shows the next message for d 1 and the message before for d -1.
// At the end of a page it loads the page next to it.
func (c *conversation) step(d int) tea.Cmd {
	i := c.cur + d
	switch {
	case i >= 0 && i < len(c.msgs):
		c.show(i)
	case i < 0 && c.page > 1:
		c.jumpAt = pageSize
		return c.fetch(c.page-1, false)
	case i >= len(c.msgs) && c.page < c.pages:
		c.jumpAt = 1
		return c.fetch(c.page+1, false)
	default:
		return ui.Toast(i18n.T("bbs.social.conv.noMore"), ui.ToastInfo)
	}
	return nil
}

// goTo shows the message with the number arg. It loads its page when the
// message is on another page.
func (c *conversation) goTo(arg string) tea.Cmd {
	n, err := strconv.Atoi(arg)
	if err != nil || n < 1 || n > max(c.total, len(c.msgs)) {
		return noRow()
	}
	page, i := (n-1)/pageSize+1, (n-1)%pageSize
	if page == c.page {
		c.show(i)
		return nil
	}
	c.jumpAt = i + 1
	return c.fetch(page, false)
}

// quoteNumber quotes the message with the number arg on this page, or the
// shown message without arg.
func (c *conversation) quoteNumber(arg string) tea.Cmd {
	if arg == "" {
		return c.quote()
	}
	n, err := strconv.Atoi(arg)
	i := n - 1 - c.base()
	if err != nil || i < 0 || i >= len(c.msgs) {
		return noRow()
	}
	return c.quoteAt(i)
}

// openLink opens the link with the number arg of the shown message.
func (c *conversation) openLink(arg string) tea.Cmd {
	n, err := strconv.Atoi(arg)
	if err == nil && c.cur < len(c.links) {
		for _, l := range c.links[c.cur] {
			if l.N == n {
				return ui.Open(l.URL)
			}
		}
	}
	return ui.Toast(i18n.T("bbs.social.conv.noLink"), ui.ToastError)
}

// replyBBS opens the line editor with the start text.
func (c *conversation) replyBBS(text string) tea.Cmd {
	client, id := c.d.API, c.id
	heading := i18n.Tf("bbs.social.editor.reply", map[string]any{"Title": c.Title()})
	c.led = newLineEditor(c.d, heading, api.MessageTarget(id), "", func(ctx context.Context, v draft) (tea.Msg, error) {
		m, err := client.ReplyConversation(ctx, id, v.Body)
		if err != nil {
			return nil, err
		}
		return replySentMsg{id: m.Id}, nil
	})
	c.led.quote = c.quote
	c.led.setSize(c.w, c.h)
	return c.led.addText(text)
}

// bbsScroll moves the BBS view with the arrow and page keys.
func (c *conversation) bbsScroll(k string) {
	last := max(len(c.bbsLines)-c.area(), 0)
	switch k {
	case "up", "k":
		c.off--
	case "down", "j":
		c.off++
	case "pgup":
		c.off -= c.area()
	case "pgdown", "space":
		c.off += c.area()
	case "home", "g":
		c.off = 0
	case "end", "G":
		c.off = last
	}
	c.off = max(0, min(c.off, last))
	c.cur = 0
	for i, at := range c.starts {
		if at <= c.off {
			c.cur = i
		}
	}
}

// buildBBS draws the messages of the page in ASCII boxes: a head row with
// the number, the author and the date, the text and a row with the
// commands.
func (c *conversation) buildBBS() {
	c.width = c.w
	s := c.d.Styles
	c.links, c.starts = nil, nil
	var lines []string
	foot := boxRow(s, c.w, apart("", i18n.T("bbs.social.conv.actions"), c.w-4))
	for i, m := range c.msgs {
		c.starts = append(c.starts, len(lines))
		head := apart("#"+strconv.Itoa(c.base()+i+1)+"   "+api.AuthorName(m.Author), bbsDate(c.d, m.CreatedAt), c.w-4)
		lines = append(lines, boxRow(s, c.w, head), ui.Rule(s, c.w, '-'), boxRow(s, c.w, ""))
		text, links, err := c.d.Render.PostBBS(m.BodyHtml, attachments(m.Attachments), max(c.w-6, 10))
		if err != nil {
			text = err.Error()
		}
		c.links = append(c.links, links)
		for _, l := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
			lines = append(lines, boxRow(s, c.w, " "+l))
		}
		lines = append(lines, boxRow(s, c.w, ""), ui.Rule(s, c.w, '-'), foot, ui.Rule(s, c.w, '='))
	}
	c.bbsLines = lines
}

func (c *conversation) bbsView(width, height int) string {
	if v, ok := c.load.bbsView(width, height); ok {
		return v
	}
	s := c.d.Styles
	path := i18n.Tf("bbs.social.conv.path", map[string]any{"Title": c.Title()})
	lines := []string{
		ui.Rule(s, width, '-'),
		boxRow(s, width, s.Subtitle.Render(path)),
		boxRow(s, width, participants(c.conv)),
		ui.Rule(s, width, '='),
	}
	end := min(c.off+c.area(), len(c.bbsLines))
	lines = append(lines, c.bbsLines[min(c.off, end):end]...)
	return pad(lines, height)
}
