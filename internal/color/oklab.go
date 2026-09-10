package color

import "math"

// -- MATH IS AI GENERATED --
// I don't understand most of it
// --------------------------

// The round trip between a "#rrggbb" and OkLab, in the two legs the rest of the package needs:
// hex -> OkLab for reading a color, OkLab -> linear -> hex for writing one back.
// The matrices are Ottosson's published sRGB <-> OkLab coefficients.

// Below it a color has no useful hue. Low on purpose: beige is C ~0.033.
// Both the sort key and the harmony rotations gate on this, and the key is stored, so changing it
// restates every color_key and needs a migration.
const neutralChroma = 0.02

// The hue of an OkLab pair as the cosine and sine the gamut searches are written in terms of.
func unitHue(a, b float64) (cos, sin float64) {
	hue := math.Atan2(b, a)
	return math.Cos(hue), math.Sin(hue)
}

// The sRGB -> OkLab leg, deliberately a second copy of the one inside perceptualSortKey rather
// than a helper both call. That key is stored in color_key, and moving those three expressions
// could change a rounding by one, which restates every stored key and needs a migration.
// Edit both.
func hexToOklab(h Hex) (lightness, a, b float64) {
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

/*
The inverse of hexToOklab as far as linear sRGB, reporting whether the color fits in the gamut.
The matrix is the counterpart of the forward one above.

Gamut is judged here rather than after the transfer function: the two are monotonic in each
other, and srgbFromLinear would have to answer for a negative input first.
*/
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

// The normalized "#rrggbb" for three linear channels. They are in gamut up to gamutEpsilon,
// so the clamp only absorbs that.
func linearToHex(red, green, blue float64) Hex {
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

func srgbFromLinear(c float64) float64 {
	if c <= 0.0031308 {
		return c * 12.92
	}
	return 1.055*math.Pow(c, 1/2.4) - 0.055
}
