package forum

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// postAction is an action of the footer of a BBS post: the key of its
// label and the method that it runs on the post in focus.
type postAction struct {
	key, label string
	run        func(*reader) tea.Cmd
}

var postActions = []postAction{
	{"C", "bbs.reader.quote", (*reader).quote},
	{"R", "bbs.reader.reply", (*reader).reply},
	{"E", "bbs.reader.edit", (*reader).edit},
}

// lineZone is the zone ID of the line j of the viewport.
func (r *reader) lineZone(j int) string { return r.zid + "ln." + strconv.Itoa(j) }

// linkZone is the prefix of the zone IDs of the link labels of post i.
func (r *reader) linkZone(i int) string { return r.zid + "link." + strconv.Itoa(i) + "." }

// actionZone is the zone ID of the footer action key of post i.
func (r *reader) actionZone(key string, i int) string {
	return r.zid + "act." + key + "." + strconv.Itoa(i)
}

// markBody puts each line of the viewport text body in a zone, and the
// link labels and the footer actions of the posts in their zones. The
// empty lines under the last post get no zone.
func (r *reader) markBody(body string) string {
	if len(r.posts) == 0 {
		return body
	}
	lines := strings.Split(body, "\n")
	top, total := r.vp.YOffset(), r.vp.TotalLineCount()
	for k := 0; k < len(lines) && top+k < total; {
		i, end := r.postAt(top+k), k+1
		for end < len(lines) && top+end < total && r.postAt(top+end) == i {
			end++
		}
		block := r.d.MarkLinks(r.linkZone(i), strings.Join(lines[k:end], "\n"), len(r.bodies[i].links))
		for j, l := range strings.Split(block, "\n") {
			lines[k+j] = r.markLine(i, k+j, top+k+j, l)
		}
		k = end
	}
	return strings.Join(lines, "\n")
}

// markLine puts the line j of the viewport in its zone. y is its content
// line in post i. The footer of a BBS post also gets its action zones.
func (r *reader) markLine(i, j, y int, l string) string {
	if r.d.BBS() && y == r.footerLine(i) {
		for _, a := range postActions {
			l = r.d.MarkText(r.actionZone(a.key, i), l, i18n.T(a.label))
		}
	}
	return r.d.Mark(r.lineZone(j), l)
}

// postAt returns the index of the post that holds the content line y.
func (r *reader) postAt(y int) int {
	i := 0
	for k, s := range r.starts {
		if s <= y {
			i = k
		}
	}
	return i
}

// footerLine returns the content line of the footer of the BBS post i:
// the line above the last line of the post.
func (r *reader) footerLine(i int) int {
	end := r.vp.TotalLineCount()
	if i+1 < len(r.starts) {
		end = r.starts[i+1]
	}
	return end - 2
}

// click handles a click: a choice row, the page counter, a link label, a
// footer action or a post. A click on a post puts the focus on it.
func (r *reader) click(msg tea.MouseReleaseMsg) tea.Cmd {
	switch {
	case r.picker != nil:
		return r.pick(msg)
	case r.view == nil || r.loading || r.err != nil || len(r.posts) == 0:
		return nil
	}
	if dir, ok := r.d.ClickedSide(r.zid+"page", msg); ok {
		return r.turn(dir)
	}
	return r.clickPost(msg)
}

// clickPost handles a click on the line of a post.
func (r *reader) clickPost(msg tea.MouseReleaseMsg) tea.Cmd {
	y, ok := r.clickedLine(msg)
	if !ok {
		return nil
	}
	i := r.postAt(y)
	if n, ok := r.d.ClickedLink(r.linkZone(i), len(r.bodies[i].links), msg); ok {
		return ui.Open(r.bodies[i].links[n-1].URL)
	}
	r.focusPost(i)
	for _, a := range postActions {
		if r.d.Clicked(r.actionZone(a.key, i), msg) {
			return a.run(r)
		}
	}
	return nil
}

// clickedLine returns the content line under the click.
func (r *reader) clickedLine(msg tea.MouseReleaseMsg) (int, bool) {
	for j := range r.vp.Height() {
		if r.d.Clicked(r.lineZone(j), msg) {
			return r.vp.YOffset() + j, true
		}
	}
	return 0, false
}

// focusPost puts the focus on post i and keeps the scroll offset.
func (r *reader) focusPost(i int) {
	if i == r.focus {
		return
	}
	r.focus = i
	r.keepOffset(r.assemble)
}
