package color

import "math"

// See https://en.wikipedia.org/wiki/HSL_and_HSV

// HSL is defined on the gamma encoded channels.
// Holding saturation and lightness and moving only the hue stays inside sRGB by default

// Returns h as a hue in degrees, and saturation / lightness (both in [0, 1]).
// A gray would return 0 for hue and saturation.
func hexToHSL(h Hex) (hue, saturation, lightness float64) {
	red, green, blue := channel(h, 1), channel(h, 3), channel(h, 5)

	high := max(red, green, blue)
	low := min(red, green, blue)
	span := high - low
	lightness = (high + low) / 2

	if span == 0 {
		return 0, 0, lightness
	}
	saturation = span / (1 - math.Abs(2*lightness-1))

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
	return hue, saturation, lightness
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
