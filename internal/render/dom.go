package render

import (
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/albertgmz/sermo-cli/internal/i18n"
)

// rewriter changes the parsed body before the Markdown conversion.
type rewriter struct {
	base  *url.URL
	links []Link
	atts  map[int]Attachment
	shown map[int]bool
	// bbs puts the quote authors on the blockquote for PostBBS.
	bbs bool
}

func newRewriter(base *url.URL, attachments []Attachment) *rewriter {
	w := &rewriter{base: base, atts: map[int]Attachment{}, shown: map[int]bool{}}
	for _, a := range attachments {
		w.atts[a.ID] = a
	}
	return w
}

// walk rewrites the children of n in document order.
func (w *rewriter) walk(n *html.Node) {
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		if c.Type == html.ElementNode {
			w.element(c)
		}
		c = next
	}
}

func (w *rewriter) element(n *html.Node) {
	switch {
	case n.DataAtom == atom.Img:
		w.image(n)
	case n.DataAtom == atom.A:
		w.anchor(n)
	case hasClass(n, "bb-attach"):
		w.attachment(n)
	case hasClass(n, "bb-spoiler"):
		w.spoilerPlaceholder(n)
	case hasClass(n, "spoiler"):
		w.spoiler(n)
	case hasClass(n, "bb-quote-author"):
		w.quoteAuthor(n)
	case hasClass(n, "bb-code-header"):
		n.Parent.RemoveChild(n)
	default:
		w.walk(n)
	}
}

// add records a link and returns its number.
func (w *rewriter) add(kind LinkKind, label, u string) int {
	n := len(w.links) + 1
	w.links = append(w.links, Link{N: n, Label: label, URL: u, Kind: kind})
	return n
}

func (w *rewriter) image(n *html.Node) {
	alt := strings.TrimSpace(attr(n, "alt"))
	src := w.abs(attr(n, "src"))
	if src == "" {
		replace(n, text(alt))
		return
	}
	num := w.add(LinkImage, alt, src)
	label := i18n.Tf("render.imageNoAlt", map[string]any{"N": num})
	if alt != "" {
		label = i18n.Tf("render.image", map[string]any{"N": num, "Alt": alt})
	}
	replace(n, text(label))
}

// anchor keeps the text of a link and puts the link number after it.
func (w *rewriter) anchor(n *html.Node) {
	w.walk(n)
	if href := w.abs(attr(n, "href")); href != "" {
		num := w.add(LinkURL, strings.TrimSpace(textOf(n)), href)
		n.AppendChild(text(" [" + strconv.Itoa(num) + "]"))
	}
	unwrap(n)
}

// attachment shows an inline attachment as a label. The text of the
// element is only the file ID, so an unknown attachment is removed.
func (w *rewriter) attachment(n *html.Node) {
	id, _ := strconv.Atoi(attr(n, "data-attach-id"))
	a, ok := w.atts[id]
	if !ok || w.shown[id] {
		n.Parent.RemoveChild(n)
		return
	}
	w.shown[id] = true
	replace(n, text(w.attachmentLabel(a)))
}

func (w *rewriter) attachmentLabel(a Attachment) string {
	num := w.add(LinkAttachment, a.Filename, w.abs(a.URL))
	return i18n.Tf("render.attachment", map[string]any{"N": num, "Filename": a.Filename})
}

// appendAttachments lists the attachments that the body did not show.
func (w *rewriter) appendAttachments(doc *html.Node, attachments []Attachment) {
	var rest []Attachment
	for _, a := range attachments {
		if !w.shown[a.ID] {
			rest = append(rest, a)
		}
	}
	body := find(doc, atom.Body)
	if len(rest) == 0 || body == nil {
		return
	}
	body.AppendChild(block(atom.P, text(i18n.Plural("render.attachments", len(rest), nil))))
	list := block(atom.Ul)
	for _, a := range rest {
		list.AppendChild(block(atom.Li, text(w.attachmentLabel(a))))
	}
	body.AppendChild(list)
}

// spoilerPlaceholder labels a spoiler that the body holds only as a
// placeholder. The content of such a spoiler is not in the body.
func (w *rewriter) spoilerPlaceholder(n *html.Node) {
	label := spoilerLabel(attr(n, "data-spoiler-title"))
	if n.DataAtom == atom.Div {
		replace(n, block(atom.P, text(label)))
		return
	}
	replace(n, text(label))
}

// spoiler labels a spoiler that holds its content, and keeps the content.
func (w *rewriter) spoiler(n *html.Node) {
	label := text(spoilerLabel(""))
	if summary := find(n, atom.Summary); summary != nil {
		label = block(atom.P, text(spoilerLabel(strings.TrimSpace(textOf(summary)))))
		summary.Parent.RemoveChild(summary)
	} else {
		label.Data += " "
	}
	n.InsertBefore(label, n.FirstChild)
	w.walk(n)
}

func spoilerLabel(title string) string {
	if title == "" {
		return i18n.T("render.spoiler")
	}
	return i18n.Tf("render.spoilerTitled", map[string]any{"Title": title})
}

// quoteAuthor changes the author of a quote into a line of its own.
func (w *rewriter) quoteAuthor(n *html.Node) {
	w.walk(n)
	author := strings.TrimSpace(textOf(n))
	if author == "" {
		n.Parent.RemoveChild(n)
		return
	}
	if q := quoteOf(n); w.bbs && q != nil {
		q.Attr = append(q.Attr, html.Attribute{Key: authorAttr, Val: author})
		n.Parent.RemoveChild(n)
		return
	}
	line := i18n.Tf("render.quoteAuthor", map[string]any{"Author": author})
	replace(n, block(atom.P, block(atom.Strong, text(line))))
}

// quoteOf returns the nearest blockquote that holds n, or nil.
func quoteOf(n *html.Node) *html.Node {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.DataAtom == atom.Blockquote {
			return p
		}
	}
	return nil
}

// abs returns the absolute form of raw. It returns "" for a URL that the
// user cannot open, for example a javascript: URL.
func (w *rewriter) abs(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || raw == "" {
		return ""
	}
	if w.base != nil {
		u = w.base.ResolveReference(u)
	}
	switch u.Scheme {
	case "http", "https", "mailto":
		return u.String()
	}
	return ""
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasClass(n *html.Node, class string) bool {
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == class {
			return true
		}
	}
	return false
}

func textOf(n *html.Node) string {
	var b strings.Builder
	for d := range n.Descendants() {
		if d.Type == html.TextNode {
			b.WriteString(d.Data)
		}
	}
	return b.String()
}

func find(n *html.Node, a atom.Atom) *html.Node {
	for d := range n.Descendants() {
		if d.Type == html.ElementNode && d.DataAtom == a {
			return d
		}
	}
	return nil
}

func text(s string) *html.Node {
	return &html.Node{Type: html.TextNode, Data: s}
}

func block(a atom.Atom, children ...*html.Node) *html.Node {
	n := &html.Node{Type: html.ElementNode, DataAtom: a, Data: a.String()}
	for _, c := range children {
		n.AppendChild(c)
	}
	return n
}

func replace(old, n *html.Node) {
	old.Parent.InsertBefore(n, old)
	old.Parent.RemoveChild(old)
}

// unwrap puts the children of n in its place and removes n.
func unwrap(n *html.Node) {
	for c := n.FirstChild; c != nil; c = n.FirstChild {
		n.RemoveChild(c)
		n.Parent.InsertBefore(c, n)
	}
	n.Parent.RemoveChild(n)
}
