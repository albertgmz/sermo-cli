package fx

import colorful "github.com/lucasb-eyer/go-colorful"

// lutSize is the number of colors in a lookup table. Effects sample a table
// so that a frame does not blend colors cell by cell.
const lutSize = 128

// lut is a table of colors for the positions 0 to 1.
type lut []colorful.Color

// newLUT fills a table with f, called at evenly spaced positions in [0, 1].
func newLUT(f func(u float64) colorful.Color) lut {
	l := make(lut, lutSize)
	for i := range l {
		l[i] = f(float64(i) / (lutSize - 1)).Clamped()
	}
	return l
}

// at returns the table color nearest to position u. It clamps u into [0, 1].
func (l lut) at(u float64) colorful.Color {
	return l[int(clamp01(u)*(lutSize-1)+0.5)]
}

var black = colorful.Color{}

// fade blends black into c. The value k=0 gives black and k=1 gives c.
func fade(c colorful.Color, k float64) colorful.Color {
	return black.BlendOkLab(c, clamp01(k))
}
