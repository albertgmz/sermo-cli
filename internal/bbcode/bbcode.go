// Package bbcode writes the BB code that the composer puts in a post.
package bbcode

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// tagRE finds an opening or closing tag. Group 1 is the slash and group 2 is the name.
var tagRE = regexp.MustCompile(`\[(/?)([A-Za-z]{1,10})(?:=[^\]\n]*)?\]`)

// nameRE finds the characters that a quote option cannot hold in a name.
var nameRE = regexp.MustCompile(`["\[\],\s]+`)

// literal tags show their content as it is. Tags in them are not tags.
var literal = map[string]bool{"CODE": true, "ICODE": true, "PLAIN": true}

// Quote returns the BB code that quotes a post. The format is the XenForo
// format that Sermo reads: [QUOTE="name, post: 5, member: 9"]body[/QUOTE].
// A memberID of 0 or less means a guest and the member pair is left out.
// QUOTE tags in body without a partner become plain text, so the quote
// cannot end too early or take the closing tag of a quote around it.
func Quote(author string, postID, memberID int, body string) string {
	var b strings.Builder
	b.WriteString(`[QUOTE="`)
	if name := strings.TrimSpace(nameRE.ReplaceAllString(author, " ")); name != "" {
		b.WriteString(name + ", ")
	}
	fmt.Fprintf(&b, "post: %d", postID)
	if memberID > 0 {
		fmt.Fprintf(&b, ", member: %d", memberID)
	}
	b.WriteString(`"]`)
	b.WriteString(balance(strings.TrimSpace(body)))
	b.WriteString("[/QUOTE]")
	return b.String()
}

type span struct{ start, end int }

// balancer holds the state of one balance pass.
type balancer struct {
	src       string
	bad, open []span
}

// balance puts the QUOTE tags without a partner, and the literal tags
// without a closing tag, between PLAIN tags.
func balance(src string) string {
	b := &balancer{src: src}
	for at := 0; at < len(src); {
		m := tagRE.FindStringSubmatchIndex(src[at:])
		if m == nil {
			break
		}
		tag := span{at + m[0], at + m[1]}
		name := strings.ToUpper(src[at+m[4] : at+m[5]])
		at = b.step(tag, name, m[3] > m[2])
	}
	return plain(src, append(b.bad, b.open...))
}

// step records one tag and returns the offset where the scan continues.
func (b *balancer) step(tag span, name string, closing bool) int {
	switch {
	case literal[name] && !closing:
		if end := closerEnd(b.src, name, tag.end); end >= 0 {
			return end
		}
		b.bad = append(b.bad, tag)
	case name == "QUOTE" && !closing:
		b.open = append(b.open, tag)
	case name == "QUOTE" && len(b.open) > 0:
		b.open = b.open[:len(b.open)-1]
	case name == "QUOTE":
		b.bad = append(b.bad, tag)
	}
	return tag.end
}

// closerEnd returns the end of the first closing tag of name after from, or -1.
func closerEnd(src, name string, from int) int {
	closer := "[/" + name + "]"
	i := strings.Index(strings.ToUpper(src[from:]), closer)
	if i < 0 {
		return -1
	}
	return from + i + len(closer)
}

func plain(src string, spans []span) string {
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	var b strings.Builder
	at := 0
	for _, s := range spans {
		b.WriteString(src[at:s.start])
		b.WriteString("[PLAIN]" + src[s.start:s.end] + "[/PLAIN]")
		at = s.end
	}
	b.WriteString(src[at:])
	return b.String()
}

// Wrap puts the text from selStart to selEnd between an opening and a
// closing tag, for example [B] and [/B]. The offsets count runes and are
// clamped to the text. Wrap returns the new text and the new cursor
// offset in runes. The cursor goes between the tags when the selection is
// empty, and after the closing tag when it is not empty.
func Wrap(text, tag string, selStart, selEnd int) (string, int) {
	r := []rune(text)
	selStart, selEnd = clamp(selStart, len(r)), clamp(selEnd, len(r))
	if selStart > selEnd {
		selStart, selEnd = selEnd, selStart
	}
	tag = strings.ToUpper(tag)
	open, end := []rune("["+tag+"]"), []rune("[/"+tag+"]")
	out := make([]rune, 0, len(r)+len(open)+len(end))
	out = append(out, r[:selStart]...)
	out = append(out, open...)
	out = append(out, r[selStart:selEnd]...)
	out = append(out, end...)
	out = append(out, r[selEnd:]...)
	cursor := selStart + len(open)
	if selEnd > selStart {
		cursor = selEnd + len(open) + len(end)
	}
	return string(out), cursor
}

func clamp(v, hi int) int {
	return min(max(v, 0), hi)
}
