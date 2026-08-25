package color

import "math"

// -- MATH IS AI GENERATED --
// I don't understand most of it
// --------------------------

// What the ordering looks like is these three numbers. Changing any of them changes every stored
// key, so it needs a migration that recomputes both tables, not just a new build.
const (
	hueBuckets    = 12   // a family of shades stays together instead of interleaving by hue angle
	hueOriginDeg  = 20   // red is OkLCh hue ~29, so this opens the first bucket with red
	neutralChroma = 0.02 // below it a color has no useful hue. Low on purpose: beige is C ~0.033
)

// The key packs three parts, each with room to spare so none can carry into the one above it.
// Largest key is 121_000_999.
const (
	groupStep     = 10_000_000
	lightnessStep = 1_000
	chromaMax     = 999
)

// Feel is the perceptual ordering key of h: neutrals first by lightness, then the hue groups,
// each running dark to light. It is what sort=color orders by, and it is stored.
//
// A value ParseHex would reject yields 0. Nothing reaches this with one - the CHECK constraint on
// both hex columns rejects the row either way.
func Feel(h Hex) int32 {
	if len(h) != HexLen {
		return 0
	}
	r := srgbToLinear(channel(h, 1))
	g := srgbToLinear(channel(h, 3))
	b := srgbToLinear(channel(h, 5))

	// The matrices are Ottosson's published sRGB <-> OkLab coefficients.
	long := math.Cbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*b)
	med := math.Cbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*b)
	short := math.Cbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*b)

	lightness := 0.2104542553*long + 0.7936177850*med - 0.0040720468*short
	a := 1.9779984951*long - 2.4285922050*med + 0.4505937099*short
	bb := 0.0259040371*long + 0.7827717662*med - 0.8086757660*short

	chroma := math.Sqrt(a*a + bb*bb)

	group := 0
	if chroma >= neutralChroma {
		// Atan2 answers (-180, 180]. Subtracting the origin and wrapping once lands in [0, 360),
		// so the rotation and the normalization are the same step.
		hue := math.Atan2(bb, a)*180/math.Pi - hueOriginDeg
		if hue < 0 {
			hue += 360
		}
		group = 1 + int(hue/(360/hueBuckets))
	}

	return int32(group*groupStep +
		int(math.Round(lightness*1000))*lightnessStep +
		min(int(math.Round(chroma*1000)), chromaMax))
}

// One channel of a normalized "#rrggbb", read at i and i+1.
func channel(h Hex, i int) float64 {
	return float64(hexDigit(h[i])<<4|hexDigit(h[i+1])) / 255
}

// h is normalized, anything else is unreachable.
func hexDigit(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	}
	return 0
}

func srgbToLinear(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}
