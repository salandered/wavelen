package color

import "math"

// -- MATH IS AI GENERATED --
// I don't understand most of it
// --------------------------

// A harmony turns the hue and keeps everything else. The rotations are the whole definition.
const (
	complementDeg = 180
	triadDeg      = 120
)

// How far outside [0, 1] a linear channel may land before the color counts as out of gamut.
// Only absorbs float noise at the boundary; a real overshoot is orders of magnitude larger.
const gamutEpsilon = 1e-9

// Halvings per binary search. 20 steps resolve to under 1e-6 of chroma or of lightness, both far
// below what a byte per channel can show.
const gamutSteps = 20

// Lightnesses sampled when the requested chroma does not fit at the one asked for. A grid rather
// than a hill climb: the most chroma a hue can hold peaks once as lightness rises, but sRGB is
// only nearly convex at a fixed hue, and a search that assumes the peak is the only one can be
// wrong without saying so. 128 costs 2560 matrix evaluations on a path that only runs when the
// color is out of gamut, and the result is refined off the grid anyway.
const lightnessSamples = 128

// Above any chroma sRGB holds, so the search for the most a hue can carry starts bracketed.
const chromaCeiling = 0.5

// Complement is h with its hue turned half a circle in OkLCh, chroma kept and lightness moved
// only as far as sRGB forces.
//
// A neutral has no hue to turn and is returned unchanged, so the complement of a gray is that
// gray. So is a value ParseHex would reject, matching Feel's rule that nothing here validates.
func Complement(h Hex) Hex {
	return rotate(h, complementDeg)
}

// Triad is the other two corners of the equilateral triangle h sits on, in hue order: h plus
// 120 degrees, then h plus 240. Neutrals and rejected values come back as h twice.
func Triad(h Hex) (Hex, Hex) {
	return rotate(h, triadDeg), rotate(h, 2*triadDeg)
}

// Turns the hue by deg, then finds the closest color sRGB can actually show.
func rotate(h Hex, deg float64) Hex {
	if len(h) != HexLen {
		return h
	}

	lightness, a, b := toOklab(h)

	chroma := math.Hypot(a, b)
	if chroma < neutralChroma {
		return h // the same cutoff Feel groups neutrals by
	}

	// Trig wraps on its own, so nothing has to normalize the angle back into a range.
	hue := math.Atan2(b, a) + deg*math.Pi/180

	return fitToSRGB(lightness, chroma, hue)
}

// The sRGB -> OkLab leg, deliberately a second copy of the one inside Feel rather than a helper
// both call. Feel's result is stored in color_key, and moving those three expressions could
// change a rounding by one, which restates every stored key and needs a migration. Edit both.
func toOklab(h Hex) (lightness, a, b float64) {
	red := srgbToLinear(channel(h, 1))
	green := srgbToLinear(channel(h, 3))
	blue := srgbToLinear(channel(h, 5))

	long := math.Cbrt(0.4122214708*red + 0.5363325363*green + 0.0514459929*blue)
	med := math.Cbrt(0.2119034982*red + 0.6806995451*green + 0.1073969566*blue)
	short := math.Cbrt(0.0883024619*red + 0.2817188376*green + 0.6299787005*blue)

	return 0.2104542553*long + 0.7936177850*med - 0.0040720468*short,
		1.9779984951*long - 2.4285922050*med + 0.4505937099*short,
		0.0259040371*long + 0.7827717662*med - 0.8086757660*short
}

// The hex at the requested hue and chroma, at the lightness closest to the one asked for that
// sRGB can actually show.
//
// Turning a hue at full chroma leaves the gamut often - it is nowhere near a cylinder in OkLCh.
// Lightness is what gives way, because a saturated color and its opposite are rarely equally
// bright: a vivid violet is dark and the chartreuse across from it is not, and there is no dark
// vivid chartreuse to compromise on. Holding lightness instead would answer with the one color
// that is dark at that hue, which is a near neutral - the right lightness and no color left.
//
// Clamping the three channels would be shorter than either and wrong: pulling a negative red up
// to zero moves the hue, so the answer stops being a rotation of anything.
func fitToSRGB(lightness, chroma, hue float64) Hex {
	cos, sin := math.Cos(hue), math.Sin(hue)

	if red, green, blue, ok := oklabToLinear(lightness, chroma*cos, chroma*sin); ok {
		return toHex(red, green, blue) // nothing has to move
	}

	// The closest lightness that carries the chroma, and the lightness carrying the most of it
	// in case none does. One pass answers both.
	nearest, fits := 0.0, false
	peak, peakChroma := 0.0, -1.0

	for i := range lightnessSamples + 1 {
		l := float64(i) / lightnessSamples
		held := maxChromaAt(l, cos, sin)

		if held > peakChroma {
			peak, peakChroma = l, held
		}
		if held >= chroma && (!fits || math.Abs(l-lightness) < math.Abs(nearest-lightness)) {
			nearest, fits = l, true
		}
	}

	if !fits {
		// no lightness at this hue holds that much chroma, so the chroma gives way after all
		return colorAt(peak, peakChroma, cos, sin)
	}

	// The grid landed inside the gamut and the requested lightness is outside it, so the edge is
	// between the two. Walking to it is what keeps the move minimal rather than grid-sized.
	low, high := nearest, lightness
	for range gamutSteps {
		mid := (low + high) / 2
		if maxChromaAt(mid, cos, sin) >= chroma {
			low = mid
		} else {
			high = mid
		}
	}
	return colorAt(low, chroma, cos, sin)
}

// The most chroma sRGB holds at this lightness and hue. Chroma 0 is a gray, which is in gamut for
// any lightness in [0, 1], so the search is bracketed from below without a check.
func maxChromaAt(lightness, cos, sin float64) float64 {
	low, high := 0.0, chromaCeiling
	for range gamutSteps {
		mid := (low + high) / 2
		if _, _, _, ok := oklabToLinear(lightness, mid*cos, mid*sin); ok {
			low = mid
		} else {
			high = mid
		}
	}
	return low
}

func colorAt(lightness, chroma, cos, sin float64) Hex {
	red, green, blue, _ := oklabToLinear(lightness, chroma*cos, chroma*sin)
	return toHex(red, green, blue)
}

// The inverse of toOklab as far as linear sRGB, reporting whether the color fits in the gamut.
// The matrix is Ottosson's, the counterpart of the forward one above.
//
// Gamut is judged here rather than after the transfer function: the two are monotonic in each
// other, and srgbFromLinear would have to answer for a negative input first.
func oklabToLinear(lightness, a, b float64) (red, green, blue float64, inGamut bool) {
	long := lightness + 0.3963377774*a + 0.2158037573*b
	med := lightness - 0.1055613458*a - 0.0638541728*b
	short := lightness - 0.0894841775*a - 1.2914855480*b

	long, med, short = long*long*long, med*med*med, short*short*short

	red = 4.0767416621*long - 3.3077115913*med + 0.2309699292*short
	green = -1.2684380046*long + 2.6097574011*med - 0.3413193965*short
	blue = -0.0041960863*long - 0.7034186147*med + 1.7076147010*short

	return red, green, blue, fits(red) && fits(green) && fits(blue)
}

func fits(c float64) bool {
	return c >= -gamutEpsilon && c <= 1+gamutEpsilon
}

// The normalized "#rrggbb" for three linear channels. They are in gamut up to gamutEpsilon,
// so the clamp only absorbs that.
func toHex(red, green, blue float64) Hex {
	const digits = "0123456789abcdef"

	out := make([]byte, HexLen)
	out[0] = '#'
	for i, c := range [3]float64{red, green, blue} {
		v := quantize(srgbFromLinear(c))
		out[1+2*i] = digits[v>>4]
		out[2+2*i] = digits[v&0xf]
	}
	return Hex(out)
}

func quantize(c float64) int {
	return int(math.Round(min(max(c, 0), 1) * 255))
}

// The inverse of srgbToLinear in oklch.go.
func srgbFromLinear(c float64) float64 {
	if c <= 0.0031308 {
		return c * 12.92
	}
	return 1.055*math.Pow(c, 1/2.4) - 0.055
}
