package fx

import (
	"math"
	"math/rand/v2"

	colorful "github.com/lucasb-eyer/go-colorful"
)

// ColorFunc gives the color of the cell at column x and row y in a w x h area
// at the time t in seconds. It must be a pure function.
type ColorFunc func(x, y, w, h int, t float64) colorful.Color

// Effect is an animated background with a name.
type Effect struct {
	Name  string
	Color ColorFunc
}

type builder struct {
	name  string
	build func(p Gradient) ColorFunc
}

var builders = []builder{
	{"plasma", plasma},
	{"fire", fire},
	{"gradient", flow},
	{"starfield", starfield},
	{"crt", crt},
}

// All returns each effect, tinted by the palette p. The order is plasma,
// fire, gradient, starfield and crt.
func All(p Gradient) []Effect {
	out := make([]Effect, len(builders))
	for i, b := range builders {
		out[i] = Effect{Name: b.name, Color: b.build(p)}
	}
	return out
}

// ByName returns the effect with the given name, tinted by the palette p.
// The second value is false when no effect has that name.
func ByName(name string, p Gradient) (Effect, bool) {
	for _, b := range builders {
		if b.name == name {
			return Effect{Name: b.name, Color: b.build(p)}, true
		}
	}
	return Effect{}, false
}

// Random returns one effect from All, tinted by the palette p.
func Random(r *rand.Rand, p Gradient) Effect {
	b := builders[r.IntN(len(builders))]
	return Effect{Name: b.name, Color: b.build(p)}
}

var white = colorful.Color{R: 1, G: 1, B: 1}

// cyclic gives a table that goes along p and back, so that the end of the
// table touches the start without a seam.
func cyclic(p Gradient, k float64) lut {
	return newLUT(func(u float64) colorful.Color { return fade(p.At(pingPong(2*u)), k) })
}

func fract(x float64) float64 {
	return x - math.Floor(x)
}

// plasma adds four sine waves. One wave turns around a center that moves.
func plasma(p Gradient) ColorFunc {
	l := cyclic(p, 0.85)
	return func(x, y, w, h int, t float64) colorful.Color {
		fx := float64(x) * 0.1
		fy := float64(y) * 0.2
		cx := float64(w) * 0.1 * (0.5 + 0.3*math.Sin(t*0.31))
		cy := float64(h) * 0.2 * (0.5 + 0.3*math.Cos(t*0.23))
		dx, dy := fx-cx, fy-cy
		v := math.Sin(fx+t*0.7) +
			math.Sin(fy*1.3+t*0.5) +
			math.Sin((fx+fy)*0.6+t*0.4) +
			math.Sin(math.Sqrt(dx*dx+dy*dy)*1.1-t*0.9)
		return l.at(fract(v/8 + 0.5 + t*0.03))
	}
}

// fire moves noise up from the bottom row. The heat is high at the bottom
// and the hot end of the palette shows there.
func fire(p Gradient) ColorFunc {
	l := newLUT(func(u float64) colorful.Color {
		return fade(p.At(u), smooth(clamp01((u-0.05)/0.45)))
	})
	return func(x, y, w, h int, t float64) colorful.Color {
		up := float64(h-1-y) / float64(max(h-1, 1))
		fy := float64(y)
		sx := float64(x)*0.09 + math.Sin(fy*0.15+t*1.7)*0.35
		n := fbm(sx, fy*0.14+t*1.8)
		base := 1 - up
		heat := math.Pow(base, 1.2) * (0.3 + 1.1*n)
		return l.at(heat)
	}
}

// flow moves a diagonal band of the palette across the area.
func flow(p Gradient) ColorFunc {
	l := cyclic(p, 0.9)
	return func(x, y, w, h int, t float64) colorful.Color {
		fy := float64(y)
		d := (float64(x) + fy*2) / float64(w+2*h)
		wave := 0.04 * math.Sin(fy*0.25+t*1.3)
		return l.at(fract(d*1.2 + wave - t*0.08))
	}
}

type starLayer struct {
	speed, density, bright float64
}

var starLayers = [...]starLayer{
	{speed: 6, density: 0.005, bright: 1},
	{speed: 2.5, density: 0.008, bright: 0.7},
	{speed: 0.8, density: 0.012, bright: 0.45},
}

const (
	starTints  = 8
	starLevels = 16
)

// starfield draws three layers of stars that move left at different speeds
// over a dark nebula.
func starfield(p Gradient) ColorFunc {
	sky := newLUT(func(u float64) colorful.Color { return fade(p.At(0.2+0.6*u), 0.06+0.3*u) })
	stars := starTable(p, sky.at(0))
	return func(x, y, w, h int, t float64) colorful.Color {
		for i, layer := range starLayers {
			if c, ok := star(stars, x, y, i, layer, t); ok {
				return c
			}
		}
		fy := float64(y)
		neb := fbm(float64(x)*0.05+t*0.02, fy*0.1)
		return sky.at(0.2*fy/float64(max(h, 1)) + 0.8*neb*neb)
	}
}

// starTable holds the star colors for each tint and brightness level.
func starTable(p Gradient, sky colorful.Color) [starTints][starLevels]colorful.Color {
	var tab [starTints][starLevels]colorful.Color
	for i := range tab {
		tint := p.At(float64(i)/(starTints-1)).BlendOkLab(white, 0.45)
		for j := range tab[i] {
			tab[i][j] = sky.BlendOkLab(tint, float64(j+1)/starLevels).Clamped()
		}
	}
	return tab
}

func star(tab [starTints][starLevels]colorful.Color, x, y, layer int, s starLayer, t float64) (colorful.Color, bool) {
	sx := x + int(math.Floor(t*s.speed))
	hv := hash3(sx, y, 10+layer)
	if hv >= s.density {
		return colorful.Color{}, false
	}
	k := hv / s.density
	twinkle := 0.65 + 0.35*math.Sin(t*(1.5+3*k)+k*40)
	level := int(s.bright * twinkle * (starLevels - 1))
	return tab[int(k*starTints)%starTints][level], true
}

// crt draws a dark phosphor screen with scanlines, a bright band that rolls
// down, a small flicker and dark corners.
func crt(p Gradient) ColorFunc {
	phosphor := vivid(p)
	l := newLUT(func(u float64) colorful.Color {
		return fade(phosphor.BlendOkLab(white, clamp01((u-0.75)*1.6)), u)
	})
	return func(x, y, w, h int, t float64) colorful.Color {
		fw, fh := float64(max(w-1, 1)), float64(max(h-1, 1))
		scan := 1.0
		if y%2 == 1 {
			scan = 0.55
		}
		center := (fract(t*0.13)*1.4 - 0.2) * fh
		d := (float64(y) - center) / (fh*0.08 + 1)
		band := 0.45 * math.Exp(-d*d)
		flicker := 0.94 + 0.04*math.Sin(t*47) + 0.02*math.Sin(t*13.3)
		nx, ny := float64(x)/fw*2-1, float64(y)/fh*2-1
		vignette := 1 - 0.3*(0.6*nx*nx+ny*ny)
		grain := (hash3(x, y, int(t*30)) - 0.5) * 0.06
		return l.at((0.45+band)*scan*flicker*vignette + grain)
	}
}

// vivid returns the stop of p with the highest chroma.
func vivid(p Gradient) colorful.Color {
	best, bestC := p.At(0.5), -1.0
	for _, c := range p {
		if _, ch, _ := c.OkLch(); ch > bestC {
			best, bestC = c.Clamped(), ch
		}
	}
	return best
}
