package social

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/i18n"
)

// linkZone is the prefix of the zones of the link labels of message i.
func (c *conversation) linkZone(i int) string { return c.zid + "link." + strconv.Itoa(i) + "." }

// msgZone is the zone of the shown lines of message i.
func (c *conversation) msgZone(i int) string { return c.zid + "msg." + strconv.Itoa(i) }

// actZone is the zone of the action key of the footer of message i.
func (c *conversation) actZone(i int, key string) string {
	return c.zid + "act." + strconv.Itoa(i) + "." + key
}

// msgAt returns the index of the message that has the line at.
func (c *conversation) msgAt(at int) int {
	i := 0
	for k, start := range c.starts {
		if start <= at {
			i = k
		}
	}
	return i
}

// markMessages groups the shown lines by message and marks each group.
// top is the index of the first shown line.
func (c *conversation) markMessages(lines []string, top int, mark func(i int, text string) string) string {
	if !zonesOn(c.d) || len(c.starts) == 0 {
		return strings.Join(lines, "\n")
	}
	var out []string
	for k := 0; k < len(lines); {
		i, end := c.msgAt(top+k), k+1
		for end < len(lines) && c.msgAt(top+end) == i {
			end++
		}
		out = append(out, mark(i, strings.Join(lines[k:end], "\n")))
		k = end
	}
	return strings.Join(out, "\n")
}

// markModern marks the link labels of message i and the message.
func (c *conversation) markModern(i int, text string) string {
	if i < len(c.links) {
		text = c.d.MarkLinks(c.linkZone(i), text, len(c.links[i]))
	}
	return c.d.Mark(c.msgZone(i), text)
}

// markBBS marks the link labels of message i and the actions of its
// footer.
func (c *conversation) markBBS(i int, text string) string {
	if i < len(c.links) {
		text = c.d.MarkLinks(c.linkZone(i), text, len(c.links[i]))
	}
	lines := strings.Split(text, "\n")
	foot := i18n.T("bbs.social.conv.actions")
	for k, l := range lines {
		if !strings.Contains(l, foot) {
			continue
		}
		for _, a := range actionParts(foot) {
			l = c.d.MarkText(c.actZone(i, a.key), l, a.text)
		}
		lines[k] = l
	}
	return strings.Join(lines, "\n")
}

// click opens a clicked link label. In the BBS interface it runs a
// clicked footer action. Else it turns the page for a click on the page
// counter and focuses a clicked message.
func (c *conversation) click(msg tea.MouseReleaseMsg) tea.Cmd {
	if c.picker.open {
		return c.picker.update(msg)
	}
	if c.load.loading || c.load.err != nil {
		return nil
	}
	for i := range c.links {
		if cmd, ok := clickLink(c.d, c.linkZone(i), c.links[i], msg); ok {
			return cmd
		}
	}
	if c.d.BBS() {
		return c.clickAction(msg)
	}
	return c.clickModern(msg)
}

func (c *conversation) clickModern(msg tea.MouseReleaseMsg) tea.Cmd {
	if page, ok := pageTurn(c.d, c.zid+"page", msg, c.page); ok {
		return c.turn(page)
	}
	for i := range c.msgs {
		if c.d.Clicked(c.msgZone(i), msg) {
			c.cur = i
			return nil
		}
	}
	return nil
}

// clickAction runs the footer action of a message: [C] quotes that
// message and [R] opens a reply.
func (c *conversation) clickAction(msg tea.MouseReleaseMsg) tea.Cmd {
	for i := range c.msgs {
		for _, a := range actionParts(i18n.T("bbs.social.conv.actions")) {
			if !c.d.Clicked(c.actZone(i, a.key), msg) {
				continue
			}
			if strings.EqualFold(a.key, "C") {
				return c.quoteAt(i)
			}
			return c.reply("")
		}
	}
	return nil
}
