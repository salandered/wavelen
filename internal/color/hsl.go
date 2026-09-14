package color

import "math"

// See https://en.wikipedia.org/wiki/HSL_and_HSV

// HSL is defined on the gamma encoded channels.
// Holding saturation and lightness and moving only the hue stays inside sRGB by default

// Returns h as a hue in degrees, and saturation / lightness (both in [0, 1]).
// A gray would return 0 for hue and saturation.
func hexToHSL(h Hex) (hue, saturation, lightness float64) {
	red, green, blue := h.RNorm(), h.GNorm(), h.BNorm()

	high := max(red, green, blue)
	low := min(red, green, blue)
	span := high - low
	lightness = (high + low) / 2

	if span == 0 {
		return 0, 0, lightness
	}
	// A color with a channel at an end is fully saturated, and the division reaches 1 from
	// either side depending on rounding. #000011 lands a step above it.
	saturation = min(span/(1-math.Abs(2*lightness-1)), 1)

	return hueDegrees(red, green, blue, high, span), saturation, lightness
}

/*
Returns h as a hue in degrees, and saturation / value (both in [0, 1]).
HSV is HSB in Figma and Photoshop.

Hue is the same angle HSL reads.
Saturation divides the span by the high channel.
*/
func hexToHSV(h Hex) (hue, saturation, value float64) {
	red, green, blue := h.RNorm(), h.GNorm(), h.BNorm()

	high := max(red, green, blue)
	span := high - min(red, green, blue)

	if span == 0 {
		return 0, 0, high
	}
	return hueDegrees(red, green, blue, high, span), span / high, high
}

// Hue in degrees for channels whose high and span are known. HSL and HSV share it.
// A gray has a span of 0 and no hue; both callers gate on that before coming here.
func hueDegrees(red, green, blue, high, span float64) float64 {
	var hue float64
	switch high {
	case red:
		hue = math.Mod((green-blue)/span, 6)
	case green:
		hue = (blue-red)/span + 2
	default:
		hue = (red-green)/span + 4
	}

	hue *= 60
	if hue < 0 {
		hue += 360
	}
	return hue
}

// hslToHex is the inverse of hexToHSL
func hslToHex(hue, saturation, lightness float64) Hex {
	hue = math.Mod(math.Mod(hue, 360)+360, 360)

	chroma := (1 - math.Abs(2*lightness-1)) * saturation
	second := chroma * (1 - math.Abs(math.Mod(hue/60, 2)-1))
	base := lightness - chroma/2

	var red, green, blue float64
	switch int(hue / 60) {
	case 0:
		red, green = chroma, second
	case 1:
		red, green = second, chroma
	case 2:
		green, blue = chroma, second
	case 3:
		green, blue = second, chroma
	case 4:
		red, blue = second, chroma
	default:
		red, blue = chroma, second
	}

	return srgbToHex(red+base, green+base, blue+base)
}
