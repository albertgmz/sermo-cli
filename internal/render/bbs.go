package render

import (
	"strings"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/styles"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/albertgmz/sermo-cli/internal/i18n"
)

// bbsStyle is the cache key of the Glamour renderer for PostBBS.
const bbsStyle = "bbs"

// authorAttr holds the quote author on a blockquote for PostBBS.
const authorAttr = "data-bbs-author"

// PostBBS renders the bodyHtml of a post as plain text for the BBS
// interface. The links and the images get the same numbers as in Post.
// Glamour renders the text with its ASCII style. PostBBS draws each quote
// and each code block in an ASCII box. Quotes in quotes give boxes in boxes.
// The output has no escape codes and no line is wider than width.
func (r *Renderer) PostBBS(body string, attachments []Attachment, width int) (string, []Link, error) {
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	w := newRewriter(r.base, attachments)
	w.bbs = true
	w.walk(doc)
	w.appendAttachments(doc, attachments)
	root := find(doc, atom.Body)
	if root == nil {
		root = doc
	}
	lines, err := r.bbsBlocks(root, width)
	if err != nil {
		return "", nil, err
	}
	return trimBlankLines(strings.Join(lines, "\n")), w.links, nil
}

// bbsBlocks renders the children of n at width columns. Text goes to
// Glamour in groups. Quotes and code blocks get boxes. An element that holds
// a quote or a code block is opened, so that its boxes show too.
func (r *Renderer) bbsBlocks(n *html.Node, width int) ([]string, error) {
	var out []string
	var group []*html.Node
	flush := func() error {
		if len(group) == 0 {
			return nil
		}
		lines, err := r.bbsText(group, width)
		group = nil
		out = joinBlock(out, lines)
		return err
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if !isBox(c) && !holdsBox(c) {
			group = append(group, c)
			continue
		}
		if err := flush(); err != nil {
			return nil, err
		}
		lines, err := r.bbsBox(c, width)
		if err != nil {
			return nil, err
		}
		out = joinBlock(out, lines)
	}
	return out, flush()
}

func (r *Renderer) bbsBox(n *html.Node, width int) ([]string, error) {
	switch n.DataAtom {
	case atom.Blockquote:
		inner, err := r.bbsBlocks(n, max(width-4, 1))
		return quoteBox(attr(n, authorAttr), inner, width), err
	case atom.Pre:
		return codeBox(codeLang(n), textOf(n), width), nil
	}
	return r.bbsBlocks(n, width)
}

func isBox(n *html.Node) bool {
	return n.Type == html.ElementNode && (n.DataAtom == atom.Blockquote || n.DataAtom == atom.Pre)
}

func holdsBox(n *html.Node) bool {
	for d := range n.Descendants() {
		if isBox(d) {
			return true
		}
	}
	return false
}

// joinBlock adds the lines of a block to out with one blank line between
// two blocks.
func joinBlock(out, lines []string) []string {
	if len(lines) == 0 {
		return out
	}
	if len(out) > 0 {
		out = append(out, "")
	}
	return append(out, lines...)
}

// bbsText renders a group of nodes with the Glamour ASCII style.
func (r *Renderer) bbsText(nodes []*html.Node, width int) ([]string, error) {
	div := block(atom.Div)
	for _, n := range nodes {
		n.Parent.RemoveChild(n)
		div.AppendChild(n)
	}
	md, err := toMarkdown(div)
	if err != nil {
		return nil, err
	}
	out, err := r.glamour(md, width, bbsStyle)
	if err != nil {
		return nil, err
	}
	text := trimBlankLines(ansi.Strip(out))
	if text == "" {
		return nil, nil
	}
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		lines[i] = ansi.Truncate(strings.TrimRight(l, " "), width, "")
	}
	return lines, nil
}

// bbsOptions are the Glamour options for PostBBS: the ASCII style with no
// margin, so that the screen controls the margin.
func bbsOptions(width int) []glamour.TermRendererOption {
	cfg := styles.ASCIIStyleConfig
	zero := uint(0)
	cfg.Document.Margin = &zero
	return []glamour.TermRendererOption{glamour.WithStyles(cfg), glamour.WithWordWrap(width)}
}

// quoteBox draws lines in a box with the author in the top line:
//
//	.-- ana escribió: ---.
//	| text               |
//	'--------------------'
func quoteBox(author string, lines []string, width int) []string {
	top := "." + strings.Repeat("-", max(width-2, 0)) + "."
	if author != "" {
		label := ansi.Truncate(".-- "+i18n.Tf("render.quoteAuthor", map[string]any{"Author": author})+" ", max(width-2, 0), "")
		top = label + strings.Repeat("-", max(width-1-ansi.StringWidth(label), 0)) + "."
	}
	return boxed(top, "'"+strings.Repeat("-", max(width-2, 0))+"'", lines, width)
}

// codeBox draws code in a box with the language in the top line:
//
//	+-[ go ]-----+
//	| code       |
//	+------------+
func codeBox(lang, code string, width int) []string {
	rule := "+" + strings.Repeat("-", max(width-2, 0)) + "+"
	top := rule
	if lang != "" {
		label := ansi.Truncate("+-[ "+lang+" ]", max(width-1, 0), "")
		top = label + strings.Repeat("-", max(width-1-ansi.StringWidth(label), 0)) + "+"
	}
	code = strings.TrimRight(strings.ReplaceAll(code, "\t", "    "), "\n")
	inner := max(width-4, 1)
	lines := strings.Split(ansi.Hardwrap(code, inner, true), "\n")
	return boxed(top, rule, lines, width)
}

// boxed puts lines between the top and the bottom line, with a side line
// at each end.
func boxed(top, bottom string, lines []string, width int) []string {
	inner := max(width-4, 1)
	out := make([]string, 0, len(lines)+2)
	out = append(out, top)
	for _, l := range lines {
		l = ansi.Truncate(l, inner, "")
		out = append(out, "| "+l+strings.Repeat(" ", inner-ansi.StringWidth(l))+" |")
	}
	return append(out, bottom)
}

// codeLang returns the language of a code block from its "language-x"
// class, or "" when the block has no language.
func codeLang(pre *html.Node) string {
	if lang := langClass(pre); lang != "" {
		return lang
	}
	for d := range pre.Descendants() {
		if lang := langClass(d); lang != "" {
			return lang
		}
	}
	return ""
}

func langClass(n *html.Node) string {
	for _, c := range strings.Fields(attr(n, "class")) {
		if lang, ok := strings.CutPrefix(c, "language-"); ok {
			return lang
		}
	}
	return ""
}
