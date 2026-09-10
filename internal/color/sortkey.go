package color

import "math"

// -- MATH IS AI GENERATED --
// I don't understand most of it
// --------------------------

// The perceptual sort key formula. Kept apart from the rest of the math because it is the one
// result that is stored: color_key in both tables holds it, so an edit here is a migration, not
// a deploy.

// What the ordering looks like is these three numbers. Changing any of them changes every stored
// key, so it needs a migration that recomputes both tables, not just a new build.
// The third one is neutralChroma, which lives in oklab.go because the rotations gate on it too.
const (
	hueBuckets   = 12 // a family of shades stays together instead of interleaving by hue angle
	hueOriginDeg = 20 // red is OkLCh hue ~29, so this opens the first bucket with red
)

// The key packs three parts, each with room to spare so none can carry into the one above it.
// Largest key is 121_000_999.
const (
	groupStep     = 10_000_000
	lightnessStep = 1_000
	chromaMax     = 999
)

// Perceptual ordering key of h: neutrals first by lightness, then the hue groups,
// each running dark to light. Ordinal only, the distance between two keys means nothing. It is
// what sort=color orders by, and it is stored in color_key.
func perceptualSortKey(h Hex) int32 {
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
