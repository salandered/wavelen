package color

import "math"

// intro: https://bottosson.github.io/posts/gamutclipping/

// Putting a color back inside sRGB after derive.go asked for one that may not be there.

// How far outside [0, 1] a linear channel may land before the color is treated as out of gamut.
const gamutEpsilon = 1e-9

// Halvings per binary search. They resolve to under 1e-6 of chroma or of lightness, both far
// below what a byte per channel can show.
const gamutSteps = 20

// Lightnesses sampled when the requested chroma does not fit at the one asked for. A grid rather
// than a hill climb: the most chroma a hue can hold peaks once as lightness rises, but sRGB is
// only nearly convex at a fixed hue.
//
// Note: A climb that assumes one peak can land on the wronglightness, and the result would not show it.
// The grid runs only when the color is out of gamut.
const lightnessSamples = 128

// Sits above any chroma sRGB holds, so the search for the most a hue can carry starts bracketed.
const chromaCeiling = 0.5

// Adjusts lightness first and chroma second to fit within sRGB bounds.
// See https://colorjs.io/docs/gamut-mapping
func fitToSRGB(lightness, chroma, hue float64) Hex {
	cos, sin := math.Cos(hue), math.Sin(hue)

	if red, green, blue, ok := oklabToLinear(lightness, chroma*cos, chroma*sin); ok {
		return linearToHex(red, green, blue) // nothing has to move
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
	// between the two. Walking to it keeps the move minimal rather than grid-sized.
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

// The most chroma sRGB holds at this lightness and hue.
// Chroma 0 is gray, which is in gamut for any lightness in [0, 1].
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
	return linearToHex(red, green, blue)
}

func fits(c float64) bool {
	return c >= -gamutEpsilon && c <= 1+gamutEpsilon
}
