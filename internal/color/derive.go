package color

import "math"

// Sweeps lightness across a fixed hue, avoiding extreme gamut edges
const (
	rampSteps     = 7
	rampDarkest   = 0.3
	rampLightest  = 0.9
	rampLightStep = (rampLightest - rampDarkest) / (rampSteps - 1)
)

// Turns the hue by deg on space's wheel.
func rotate(space Space, h Hex, deg float64) Hex {
	if space == HSL {
		return rotateHSL(h, deg)
	}
	return rotateOkLCh(h, deg)
}

// Turns the hue by deg in OkLCh, then finds the closest color sRGB can actually show.
func rotateOkLCh(h Hex, deg float64) Hex {
	if len(h) != HexLen {
		return h
	}

	lightness, a, b := hexToOklab(h)

	chroma := math.Hypot(a, b)
	if chroma < neutralChroma {
		return h // perceptualSortKey groups neutrals by this cutoff too
	}

	// Trig wraps on its own, so nothing has to normalize the angle back into a range.
	hue := math.Atan2(b, a) + deg*math.Pi/180

	return fitToSRGB(lightness, chroma, hue)
}

/*
Turns the hue by deg on the HSL wheel.
Adjusts hue within HSL space without requiring gamut fitting.
The neutral gate is the one rotateOkLCh uses rather than a saturation of zero, so the same colors
count as having no hue worth turning on either wheel.
*/
func rotateHSL(h Hex, deg float64) Hex {
	if len(h) != HexLen {
		return h
	}

	_, a, b := hexToOklab(h)
	if math.Hypot(a, b) < neutralChroma {
		return h
	}

	hue, saturation, lightness := hexToHSL(h)
	return hslToHex(hue+deg, saturation, lightness)
}

/*
h as a scale of rampSteps colors: hue held, lightness swept, chroma clamped to what the hue holds
at each step.

A neutral has near zero chroma to clamp, so it ramps to grays without a special case.
h is in its own ramp only if its lightness lands on a step.
*/
func ramp(h Hex) []Hex {
	out := make([]Hex, rampSteps)
	if len(h) != HexLen {
		for i := range out {
			out[i] = h
		}
		return out
	}

	_, a, b := hexToOklab(h)
	chroma := math.Hypot(a, b)
	cos, sin := unitHue(a, b)

	for i := range out {
		lightness := rampDarkest + rampLightStep*float64(i)
		out[i] = colorAt(lightness, min(chroma, maxChromaAt(lightness, cos, sin)), cos, sin)
	}
	return out
}

// The tone scale shares ramp's step count so the two render the same width.
const toneSteps = rampSteps

/*
h as a scale of toneSteps colors: hue and lightness held, chroma swept from 0 up to the most
sRGB holds at that lightness. A tone is a color mixed with a gray of its own lightness, so step 0
is that gray and the last step is h's hue at its most saturated.

Nothing leaves gamut, so no call to [fitToSRGB]. [maxChromaAt] gives the ceiling
and every step is below it by construction.

A neutral has no hue to sweep. [unitHue] would read an angle out of the rounding and answer a scale
in an invented hue, so this gates on the cutoff the rotations use and returns h repeated instead.
*/
func tones(h Hex) []Hex {
	out := make([]Hex, toneSteps)

	var lightness, a, b float64
	if len(h) == HexLen {
		lightness, a, b = hexToOklab(h)
	}

	if math.Hypot(a, b) < neutralChroma {
		for i := range out {
			out[i] = h
		}
		return out
	}

	cos, sin := unitHue(a, b)
	step := maxChromaAt(lightness, cos, sin) / (toneSteps - 1)

	for i := range out {
		out[i] = colorAt(lightness, step*float64(i), cos, sin)
	}
	return out
}
