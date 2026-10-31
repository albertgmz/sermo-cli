package tui

import (
	"math"
	"strings"

	"github.com/charmbracelet/harmonica"
	"github.com/charmbracelet/x/ansi"
)

// Spring values of the toast slide. The toast overshoots a little and
// comes to rest in about one second.
const (
	slideFrequency = 7.0
	slideDamping   = 0.7
	slideEpsilon   = 0.01
)

// toastSlide moves a toast from the right edge to its place. The position
// is a fraction of the slide distance: 1 is off the screen, 0 is at rest.
type toastSlide struct {
	spring harmonica.Spring
	pos    float64
	vel    float64
}

func newToastSlide() toastSlide {
	return toastSlide{spring: harmonica.NewSpring(harmonica.FPS(maxFPS), slideFrequency, slideDamping)}
}

// start puts the toast off the screen with no speed.
func (s *toastSlide) start() { s.pos, s.vel = 1, 0 }

// stop puts the toast at rest.
func (s *toastSlide) stop() { s.pos, s.vel = 0, 0 }

// step moves the toast one frame. It stops the toast when it is near rest.
func (s *toastSlide) step() {
	if !s.moving() {
		return
	}
	s.pos, s.vel = s.spring.Update(s.pos, s.vel, 0)
	if math.Abs(s.pos) < slideEpsilon && math.Abs(s.vel) < slideEpsilon {
		s.stop()
	}
}

func (s *toastSlide) moving() bool { return s.pos != 0 || s.vel != 0 }

// startToastSlide starts the slide when the terminal can show motion.
// Otherwise the toast shows at rest.
func (m *model) startToastSlide() {
	if m.d.Animate() && !m.d.Plain() {
		m.slide.start()
		return
	}
	m.slide.stop()
}

// toastOffset is the number of cells to the right of the place of a toast
// that is width cells wide.
func (m *model) toastOffset(width int) int {
	return int(math.Round(m.slide.pos * float64(width+toastMargin)))
}

// clipRight cuts each line of s so that it ends before the cell width.
func clipRight(s string, width int) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, max(width, 0), "")
	}
	return strings.Join(lines, "\n")
}
