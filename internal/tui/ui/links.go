package ui

import (
	"regexp"
	"strconv"

	tea "charm.land/bubbletea/v2"
)

// linkRe matches a link label like "[12]" of a rendered post.
var linkRe = regexp.MustCompile(`\[([0-9]+)\]`)

// MarkLinks puts the link labels "[N]" of text in zones, for N from 1 to
// n. The zone ID is prefix followed by N. Only the first label of each
// number gets a zone.
func (d *Deps) MarkLinks(prefix, text string, n int) string {
	if !d.zonesOn() || n == 0 {
		return text
	}
	done := map[int]bool{}
	return linkRe.ReplaceAllStringFunc(text, func(label string) string {
		k, err := strconv.Atoi(label[1 : len(label)-1])
		if err != nil || k < 1 || k > n || done[k] {
			return label
		}
		done[k] = true
		return d.Mark(prefix+strconv.Itoa(k), label)
	})
}

// ClickedLink returns the number of the link label that msg clicks. The
// labels are the ones of MarkLinks with the same prefix and n.
func (d *Deps) ClickedLink(prefix string, n int, msg tea.Msg) (int, bool) {
	if !IsClick(msg) {
		return 0, false
	}
	for k := 1; k <= n; k++ {
		if d.Clicked(prefix+strconv.Itoa(k), msg) {
			return k, true
		}
	}
	return 0, false
}
