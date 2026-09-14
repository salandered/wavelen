package color

import "math"

// Read https://www.alanzucconi.com/2015/09/30/colour-sorting/

// Perceptual sort key.
// Note: This value is stored as color_key, so changing the formula requires a data migration.

// These constants define the sort order.
// neutralChroma is shared with the harmony rotations and lives in oklab.go.
const (
	hueBuckets   = 12 // a family of shades stays together instead of interleaving by hue angle
	hueOriginDeg = 20 // red is OkLCh hue ~29, so this opens the first bucket with red
)

// Pack the three sort components with enough space to prevent overlap.
// The maximum value remains safely within int32.
const (
	groupStep     = 10_000_000
	lightnessStep = 1_000
	chromaMax     = 999
)

// Sort key: neutrals by lightness first, then hue groups from dark to light.
// The key is ordinal; differences between keys have no perceptual meaning.
// sort=color uses this key.
func perceptualSortKey(h Hex) int32 {
	if len(h) != HexLen {
		return 0
	}
	r := srgbToLinear(h.RNorm())
	g := srgbToLinear(h.GNorm())
	b := srgbToLinear(h.BNorm())

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
		// atan2 returns (-180, 180]. Shift by hueOriginDeg and wrap to [0, 360).
		// This matches the hue rotation used elsewhere.
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
