package color

import "math"

// Values are rounded. Hex is the lossless one.

// RGB channels, 0 to 255. The only block that is exact.
type RGB struct {
	R, G, B int
}

// Hue in whole degrees, the rest in whole percent.
type HSL struct {
	Hue, Saturation, Lightness int
}

// HSB in Figma and Photoshop. Hue in whole degrees, the rest in whole percent.
type HSV struct {
	Hue, Saturation, Value int
}

// Lightness in percent, Chroma absolute, Hue in whole degrees.
// A near neutral answers 0 for hue, see [Describe].
type OkLabCh struct {
	Lightness float64
	Chroma    float64
	Hue       int
}

// Info is every value derived from one hex.
type Info struct {
	Hex          Hex
	RGB          RGB
	HSL          HSL
	HSV          HSV
	OkLCh        OkLabCh
	AgainstWhite Contrast
	AgainstBlack Contrast
}

// Precisions
const (
	okLightnessDecimals = 1
	okChromaDecimals    = 3
)

// Describe returns the color [Info].
// OkLCh hue is zero for near neutrals (below the [neutralChroma])
func Describe(h Hex) Info {
	if len(h) != HexLen {
		return Info{Hex: h}
	}

	hslHue, hslSaturation, hslLightness := hexToHSL(h)
	hsvHue, hsvSaturation, hsvValue := hexToHSV(h)

	lightness, a, b := hexToOklab(h)
	chroma := math.Hypot(a, b)

	oklabHue := 0.0
	if chroma >= neutralChroma {
		oklabHue = oklabHueDegrees(a, b)
	}

	white, black := contrastsOfBW(h)

	return Info{
		Hex: h,
		RGB: RGB{
			R: h.RByte(),
			G: h.GByte(),
			B: h.BByte(),
		},
		HSL: HSL{
			Hue:        wholeDegrees(hslHue),
			Saturation: wholePercent(hslSaturation),
			Lightness:  wholePercent(hslLightness),
		},
		HSV: HSV{
			Hue:        wholeDegrees(hsvHue),
			Saturation: wholePercent(hsvSaturation),
			Value:      wholePercent(hsvValue),
		},
		OkLCh: OkLabCh{
			// [hexToOklab] returns lightness in [0, 1]
			Lightness: roundTo(lightness*100, okLightnessDecimals),
			// chroma is 0 to ~0.32
			Chroma: roundTo(chroma, okChromaDecimals),
			Hue:    wholeDegrees(oklabHue),
		},
		AgainstWhite: white,
		AgainstBlack: black,
	}
}

// The OkLab hue angle in [0, 360) degrees.
func oklabHueDegrees(a, b float64) float64 {
	deg := math.Atan2(b, a) * 180 / math.Pi
	if deg < 0 {
		deg += 360
	}
	return deg
}

// Returns v in [0, 359].
func wholeDegrees(v float64) int {
	return int(math.Round(v)) % 360
}

func wholePercent(v float64) int {
	return int(math.Round(v * 100))
}

func roundTo(v float64, decimals int) float64 {
	scale := math.Pow(10, float64(decimals))
	return math.Round(v*scale) / scale
}
