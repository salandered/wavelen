package color

import "math"

/*
WCAG 2 contrast: https://www.w3.org/TR/WCAG22/#dfn-contrast-ratio

Two colors can be compared to measure the accessibility.
Base case - compare the text color against its background.
Check the https://webaim.org/resources/contrastchecker/
*/

// WCAG 2 Level
type Level string

const (
	LevelAAA     Level = "aaa"
	LevelAA      Level = "aa"
	LevelAALarge Level = "aa-large"
	LevelFail    Level = "fail"
)

// Level ratio.
// https://www.w3.org/WAI/WCAG21/Understanding/contrast-minimum.html
const (
	ratioAAA     = 7
	ratioAA      = 4.5
	ratioAALarge = 3
)

const (
	whiteLuminance = 1
	blackLuminance = 0
)

// Decimals a ratio is written in.
const contrastRatioDecimals = 2

// Contrast against one background.
type Contrast struct {
	Ratio float64
	Level Level
}

// Contrast of h against black and white
func contrastsOfBW(h Hex) (white, black Contrast) {
	luminance := relativeLuminance(h)

	return contrastOf(contrastRatio(luminance, whiteLuminance)),
		contrastOf(contrastRatio(luminance, blackLuminance))
}

func contrastOf(ratio float64) Contrast {
	scale := math.Pow(10, float64(contrastRatioDecimals))
	// The ratio is cut (not rounded), the level reads the cut value.
	// Otherwise a value like 2.999789 can round to 3.00 and return a wrong level
	trancated := math.Trunc(ratio*scale) / scale
	return Contrast{Ratio: trancated, Level: levelOf(trancated)}
}

// Relative luminance of h, in [0, 1].
// https://www.w3.org/TR/WCAG22/#dfn-relative-luminance
func relativeLuminance(h Hex) float64 {
	return 0.2126*srgbToLinear(h.RNorm()) +
		0.7152*srgbToLinear(h.GNorm()) +
		0.0722*srgbToLinear(h.BNorm())
}

// Contrast ratio between two luminances, from 1 to 21.
func contrastRatio(a, b float64) float64 {
	// 0.05 is added to luminances, two blacks answer 1 instead of dividing by zero.
	return (max(a, b) + 0.05) / (min(a, b) + 0.05)
}

func levelOf(ratio float64) Level {
	switch {
	case ratio >= ratioAAA:
		return LevelAAA
	case ratio >= ratioAA:
		return LevelAA
	case ratio >= ratioAALarge:
		return LevelAALarge
	}
	return LevelFail
}
