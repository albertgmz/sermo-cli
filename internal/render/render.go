// Package render changes the bodyHtml of a post into text for the terminal.
//
// The pipeline is: parse the HTML, change images, links, attachments,
// spoilers and quote authors into numbered text labels, convert the
// result to Markdown, and render the Markdown with Glamour.
package render

import (
	"net/url"
	"strings"
	"sync"

	"charm.land/glamour/v2"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/strikethrough"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/net/html"
)

// LinkKind tells what a Link points to.
type LinkKind int

// The kinds of Link.
const (
	LinkURL LinkKind = iota
	LinkImage
	LinkAttachment
)

// Link is a numbered target in a rendered post. N is the number that the
// text shows. URL is absolute.
type Link struct {
	N     int
	Label string
	URL   string
	Kind  LinkKind
}

// Attachment is a file of a post. ID is the file ID that the body uses to
// show the attachment inline.
type Attachment struct {
	ID          int
	URL         string
	Filename    string
	ContentType string
}

type termKey struct {
	width int
	style string
}

// Renderer renders post bodies. It keeps one Glamour renderer for each
// width and style. It is safe for concurrent use.
type Renderer struct {
	base  *url.URL
	mu    sync.Mutex
	terms map[termKey]*glamour.TermRenderer
}

// New returns a Renderer that resolves relative URLs against baseURL.
func New(baseURL string) *Renderer {
	u, err := url.Parse(baseURL)
	if err != nil || !u.IsAbs() {
		u = nil
	}
	return &Renderer{base: u, terms: map[termKey]*glamour.TermRenderer{}}
}

// Post renders the bodyHtml of a post at width columns. The style is
// "dark", "light" or "notty". The attachments that the body does not show
// inline are listed after the body. Post returns the text and the links in
// the order of their numbers.
func (r *Renderer) Post(body string, attachments []Attachment, width int, style string) (string, []Link, error) {
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	w := newRewriter(r.base, attachments)
	w.walk(doc)
	w.appendAttachments(doc, attachments)
	md, err := toMarkdown(doc)
	if err != nil {
		return "", nil, err
	}
	out, err := r.glamour(md, width, style)
	if err != nil {
		return "", nil, err
	}
	if style == "notty" {
		// Glamour adds hyperlink sequences for bare URLs in all styles.
		out = ansi.Strip(out)
	}
	return trimBlankLines(out), w.links, nil
}

func toMarkdown(doc *html.Node) (string, error) {
	conv := converter.NewConverter(converter.WithPlugins(
		base.NewBasePlugin(),
		commonmark.NewCommonmarkPlugin(),
		strikethrough.NewStrikethroughPlugin(),
	))
	md, err := conv.ConvertNode(doc)
	return string(md), err
}

func (r *Renderer) glamour(md string, width int, style string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := termKey{width, style}
	tr, ok := r.terms[key]
	if !ok {
		opts := []glamour.TermRendererOption{glamour.WithStandardStyle(style), glamour.WithWordWrap(width)}
		if style == bbsStyle {
			opts = bbsOptions(width)
		}
		var err error
		tr, err = glamour.NewTermRenderer(opts...)
		if err != nil {
			return "", err
		}
		r.terms[key] = tr
	}
	return tr.Render(md)
}

func trimBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	blank := func(l string) bool { return strings.TrimSpace(ansi.Strip(l)) == "" }
	for len(lines) > 0 && blank(lines[0]) {
		lines = lines[1:]
	}
	for len(lines) > 0 && blank(lines[len(lines)-1]) {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}
