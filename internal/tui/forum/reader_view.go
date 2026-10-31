package forum

import (
	"image/color"
	"sort"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/albertgmz/sermo-cli/internal/api"
	"github.com/albertgmz/sermo-cli/internal/api/gen"
	"github.com/albertgmz/sermo-cli/internal/i18n"
	"github.com/albertgmz/sermo-cli/internal/render"
	"github.com/albertgmz/sermo-cli/internal/tui/ui"
)

// renderAll renders the body of each post at the reading width.
func (r *reader) renderAll() {
	r.bodies = make([]renderedBody, len(r.posts))
	for i := range r.posts {
		r.bodies[i] = r.renderBody(r.posts[i])
	}
	r.assemble()
}

func (r *reader) renderBody(p gen.Post) renderedBody {
	html := p.BodyHtml
	if s, ok := r.spoilers[p.Id]; ok {
		html = s
	}
	atts := make([]render.Attachment, 0, len(p.Attachments))
	for _, a := range p.Attachments {
		name := ""
		if a.Filename != nil {
			name = *a.Filename
		}
		atts = append(atts, render.Attachment{ID: a.FileId, URL: a.Url, Filename: name, ContentType: a.ContentType})
	}
	text, links, err := r.d.Render.Post(html, atts, ui.ReadingWidth(max(r.w, 20)), r.d.Glamour)
	if err != nil {
		return renderedBody{text: r.d.Styles.Error.Render(err.Error())}
	}
	return renderedBody{text: strings.Trim(text, "\n"), links: links}
}

// assemble joins the posts into the viewport content and keeps the first
// line of each post.
func (r *reader) assemble() {
	var b strings.Builder
	r.starts = make([]int, len(r.posts))
	line := 0
	for i := range r.posts {
		r.starts[i] = line
		block := r.postBlock(i)
		b.WriteString(block)
		b.WriteString("\n")
		line += strings.Count(block, "\n") + 1
	}
	r.vp.SetContent(strings.TrimSuffix(b.String(), "\n"))
}

func (r *reader) postBlock(i int) string {
	p := r.posts[i]
	parts := []string{r.postHeader(i), "", r.bodies[i].text}
	if line := r.reactionLine(p.Reactions); line != "" {
		parts = append(parts, "", line)
	}
	w := ui.ReadingWidth(max(r.w, 20))
	parts = append(parts, r.d.Styles.Muted.Render(strings.Repeat("─", w)), "")
	return strings.Join(parts, "\n")
}

func (r *reader) postHeader(i int) string {
	p := r.posts[i]
	st := r.d.Styles
	mark := " "
	if i == r.focus {
		mark = st.Accent.Render(glyphFocus)
	}
	name := lipgloss.NewStyle().Bold(true).Foreground(r.authorColor(p.Author)).Render(authorName(p.Author))
	meta := " · " + ago(r.d.Now(), p.CreatedAt) + " (" + absDate(p.CreatedAt) + ")"
	if p.EditedAt != nil {
		meta += " · " + i18n.T("forum.post.edited")
	}
	if r.bookmarked[p.Id] {
		meta += " · " + i18n.T("forum.post.bookmarked")
	}
	left := mark + " " + name + st.Muted.Render(meta)
	return fit(left, st.Muted.Render("#"+strconv.Itoa(p.Position+1)), ui.ReadingWidth(max(r.w, 20)))
}

// authorColor picks an accent color of the site for a member from a hash
// of the user ID. A guest gets the muted color.
func (r *reader) authorColor(a gen.ContentAuthor) color.Color {
	if !api.IsMember(a) {
		return r.d.Theme.Muted
	}
	return stableColor(r.accents, api.AuthorID(a), r.d.Theme.Accent)
}

// reactionLine shows the count of each reaction type in its own color.
func (r *reader) reactionLine(s gen.ReactionSummary) string {
	if s.Total == 0 {
		return ""
	}
	ids := make([]int, 0, len(s.Counts))
	for k := range s.Counts {
		if id, err := strconv.Atoi(k); err == nil {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(a, b int) bool { return r.typeIndex(ids[a]) < r.typeIndex(ids[b]) })
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		text := r.reactionLabel(id) + " " + strconv.Itoa(s.Counts[strconv.Itoa(id)])
		if s.Mine != nil && *s.Mine == id {
			text += " " + glyphMine
		}
		parts = append(parts, lipgloss.NewStyle().Foreground(r.reactionColor(id)).Render(text))
	}
	return strings.Join(parts, "   ")
}

func (r *reader) typeIndex(id int) int {
	for i, t := range r.rtypes {
		if t.Id == id {
			return i
		}
	}
	return len(r.rtypes) + id
}

func (r *reader) reactionLabel(id int) string {
	for _, t := range r.rtypes {
		if t.Id == id {
			return t.Emoji + " " + t.Title
		}
	}
	return "#" + strconv.Itoa(id)
}

func (r *reader) reactionColor(id int) color.Color {
	i := r.typeIndex(id)
	if len(r.accents) > 0 {
		return r.accents[i%len(r.accents)]
	}
	n := max(len(r.rtypes)-1, 1)
	return r.d.Theme.Highlight.At(float64(i%(n+1)) / float64(n))
}

func (r *reader) View(width, height int) string {
	switch {
	case r.loading && r.view == nil:
		return ui.Loading(r.d.Styles, r.sp, "", width, height)
	case r.err != nil:
		return errorView(r.d, r.err, width, height)
	case r.picker != nil:
		return r.picker.view(r.d, width, height)
	}
	body := r.vp.View()
	if len(r.posts) == 0 {
		body = ui.Empty(r.d.Styles, i18n.T("forum.thread.empty"), width, max(height-3, 1))
	}
	return r.header(width) + "\n" + body + "\n" + r.footer(width)
}

func (r *reader) header(width int) string {
	st, th := r.d.Styles, r.view.Thread
	title := th.Title
	if th.Prefix != nil {
		title = "[" + th.Prefix.Label + "] " + title
	}
	info := []string{r.view.Node.Title, i18n.Plural("forum.thread.posts", th.ReplyCount+1, nil)}
	if th.IsLocked {
		info = append(info, glyphLocked+" "+i18n.T("forum.thread.locked"))
	}
	if th.IsSticky {
		info = append(info, glyphSticky+" "+i18n.T("forum.thread.sticky"))
	}
	line := st.Muted.Render(strings.Join(info, " · "))
	if r.watching {
		line += st.Accent.Render(" · " + i18n.T("forum.thread.watching"))
	}
	if r.view.Poll.Exists {
		line += st.Accent.Render(" · " + i18n.T("forum.thread.poll"))
	}
	return ui.Header(st, r.d.Theme, title, width) + "\n" + ansi.Truncate(line, width, "…")
}

func (r *reader) footer(width int) string {
	st := r.d.Styles
	out := st.Text.Render(i18n.Tf("forum.thread.page", map[string]any{"Page": r.page, "Pages": r.pages}))
	if p := r.focusedPost(); p != nil {
		out += st.Muted.Render("  #" + strconv.Itoa(p.Position+1))
	}
	if r.loading {
		out += "  " + r.sp.View()
	}
	return ansi.Truncate(out, width, "…")
}
