package fx

import "math"

// hash3 gives a deterministic value in [0, 1) for three integers.
func hash3(x, y, z int) float64 {
	h := uint32(x)*0x27d4eb2d ^ uint32(y)*0x165667b1 ^ uint32(z)*0x9e3779b1
	h ^= h >> 15
	h *= 0x2c1b3c6d
	h ^= h >> 12
	h *= 0x297a2d39
	h ^= h >> 15
	return float64(h) / (1 << 32)
}

// valueNoise gives smooth noise in [0, 1) with a feature size of 1 unit.
func valueNoise(x, y float64, seed int) float64 {
	x0, y0 := math.Floor(x), math.Floor(y)
	ix, iy := int(x0), int(y0)
	u, v := smooth(x-x0), smooth(y-y0)
	a := hash3(ix, iy, seed)
	b := hash3(ix+1, iy, seed)
	c := hash3(ix, iy+1, seed)
	d := hash3(ix+1, iy+1, seed)
	top := a + (b-a)*u
	bottom := c + (d-c)*u
	return top + (bottom-top)*v
}

// fbm adds two octaves of value noise. The result is in [0, 1).
func fbm(x, y float64) float64 {
	return valueNoise(x, y, 1)*0.65 + valueNoise(x*2.1, y*2.1, 2)*0.35
}

func smooth(t float64) float64 {
	return t * t * (3 - 2*t)
}
