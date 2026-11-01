package tui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// The zone IDs of the chrome.
const (
	zoneAlerts   = "app.alerts"
	zoneMessages = "app.messages"
)

func hotkeyZone(k string) string { return "app.key." + k }

// hintZone is the zone of the part j of the hint i of the BBS hint line.
func hintZone(i, j int) string { return "app.hint." + strconv.Itoa(i) + "." + strconv.Itoa(j) }

// updateMouse handles the clicks on the parts of the app: the splash, the
// toast, the badges, the hotkey bar and the BBS hint line. Other clicks go
// to the top screen.
func (m *model) updateMouse(msg tea.Msg) (tea.Cmd, bool) {
	r, ok := msg.(tea.MouseReleaseMsg)
	if !ok {
		return nil, false
	}
	if m.splash != nil {
		m.splash = nil
		return nil, true
	}
	if m.toast != nil && m.inToast(r) {
		m.toast = nil
		m.slide.stop()
		return nil, true
	}
	if m.confirmQuit {
		// A click answers no, like a key other than Q.
		m.confirmQuit = false
		return nil, true
	}
	if m.d.BBS() {
		return m.clickHint(r)
	}
	return m.clickChrome(r)
}

func (m *model) inToast(r tea.MouseReleaseMsg) bool {
	x, y, w, h := m.toastRect()
	return r.X >= x && r.X < x+w && r.Y >= y && r.Y < y+h
}

// clickChrome runs the hotkey of a badge or of a hotkey bar item. While
// a text field has focus, the hotkeys do not run, as with the keys.
func (m *model) clickChrome(r tea.MouseReleaseMsg) (tea.Cmd, bool) {
	k := ""
	switch {
	case m.d.Clicked(zoneAlerts, r):
		k = "n"
	case m.d.Clicked(zoneMessages, r):
		k = "m"
	}
	for _, h := range hotkeys {
		if m.d.Clicked(hotkeyZone(h.key), r) {
			k = h.key
		}
	}
	if k == "" {
		return nil, false
	}
	if m.top().CapturesInput() {
		return nil, true
	}
	return m.hotkey(k), true
}

// clickHint runs the command of a hint of the BBS hint line.
func (m *model) clickHint(r tea.MouseReleaseMsg) (tea.Cmd, bool) {
	for i, h := range m.hints() {
		for j, p := range hintParts(h.label) {
			if m.d.Clicked(hintZone(i, j), r) {
				return m.runHint(h.name, p.key), true
			}
		}
	}
	return nil, false
}

// runHint runs the command name of a hint. The key in the brackets of the
// clicked part selects an other command of the screen when one has that
// name, for example the > of "[<] [>] Páginas". A command that needs a
// number does not run: the user types the number at the prompt. A part
// without a key is a label only and does not run.
func (m *model) runHint(name, key string) tea.Cmd {
	if key == "" {
		return nil
	}
	if !strings.EqualFold(key, name) && m.hasCommand(key) {
		name = key
	}
	if name == "#" {
		return nil
	}
	if m.top().CapturesInput() {
		if c, ok := m.screenCommand(name); ok {
			return c.Run("")
		}
		return nil
	}
	return m.runCommand(name)
}

func (m *model) hasCommand(name string) bool {
	for _, c := range m.screenCommands() {
		if strings.EqualFold(c.Name, name) {
			return true
		}
	}
	return false
}

// hintPart is one part of a hint label: a key in brackets and the text up
// to the next key.
type hintPart struct {
	key, text string
}

// hintParts splits a label before each "[". A part without a key in
// brackets has the key "".
func hintParts(label string) []hintPart {
	var out []hintPart
	for label != "" {
		end := len(label)
		if next := strings.Index(label[1:], "["); next >= 0 {
			end = next + 1
		}
		part := hintPart{text: label[:end]}
		if close := strings.Index(part.text, "]"); strings.HasPrefix(part.text, "[") && close > 0 {
			part.key = part.text[1:close]
		}
		out = append(out, part)
		label = label[end:]
	}
	return out
}

// markHint paints the label of hint i and puts each part in its zone.
func (m *model) markHint(i int, label string) string {
	var b strings.Builder
	for j, p := range hintParts(label) {
		b.WriteString(m.d.Mark(hintZone(i, j), m.paintHint(p.text)))
	}
	return b.String()
}
