package color

import "math"

// Short intro: https://en.wikipedia.org/wiki/Oklab_color_space
// Formulas are based on: https://bottosson.github.io/posts/oklab/
// See also: https://www.smashingmagazine.com/2024/10/interview-bjorn-ottosson-creator-oklab-color-space/

// Chroma below this is treated as neutral and has no useful hue.
const neutralChroma = 0.02

// Note: Changing expressions could affect the data stored in db (e.g. sort key).
// https://bottosson.github.io/posts/oklab/#converting-from-linear-srgb-to-oklab
func hexToOklab(h Hex) (lightness, a, b float64) {
	red := srgbToLinear(h.RNorm())
	green := srgbToLinear(h.GNorm())
	blue := srgbToLinear(h.BNorm())

	long := math.Cbrt(0.4122214708*red + 0.5363325363*green + 0.0514459929*blue)
	med := math.Cbrt(0.2119034982*red + 0.6806995451*green + 0.1073969566*blue)
	short := math.Cbrt(0.0883024619*red + 0.2817188376*green + 0.6299787005*blue)

	return 0.2104542553*long + 0.7936177850*med - 0.0040720468*short,
		1.9779984951*long - 2.4285922050*med + 0.4505937099*short,
		0.0259040371*long + 0.7827717662*med - 0.8086757660*short
}

// Convert OkLab to linear sRGB and report if the result is in gamut.
// Check gamut before the sRGB transfer function because it may receive negative values.
// https://bottosson.github.io/posts/oklab/#converting-from-linear-srgb-to-oklab
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

// Converts normalized sRGB channels to a hex string
func linearToHex(red, green, blue float64) Hex {
	return srgbToHex(srgbFromLinear(red), srgbFromLinear(green), srgbFromLinear(blue))
}

// Returns a hex for three channels that already carry the transfer function.
func srgbToHex(red, green, blue float64) Hex {
	const digits = "0123456789abcdef"

	out := make([]byte, HexLen)
	out[0] = '#'
	for i, c := range [3]float64{red, green, blue} {
		v := quantize(c)
		out[1+2*i] = digits[v>>4]
		out[2+2*i] = digits[v&0xf]
	}
	return Hex(out)
}

func quantize(c float64) int {
	return int(math.Round(min(max(c, 0), 1) * 255))
}

// https://registry.color.org/rgb-registry/files/bgsRGB.pdf
func srgbToLinear(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// https://registry.color.org/rgb-registry/files/bgsRGB.pdf
func srgbFromLinear(c float64) float64 {
	if c <= 0.0031308 {
		return c * 12.92
	}
	return 1.055*math.Pow(c, 1/2.4) - 0.055
}
