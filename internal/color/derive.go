package color

import "math"

// -- MATH IS AI GENERATED --
// I don't understand most of it
// --------------------------

// What each entry of the harmony table in color.go runs. Each takes one color and answers with
// others near it, and each asks the gamut search for the result rather than clamping channels.

// Ramp sweeps lightness instead of turning the hue. Below rampDarkest sRGB holds almost no chroma
// at any hue, so the clamp strips the hue out and every input converges on the same tinted near
// black. The range starts where the steps stay apart, and 7 of them keep neighbours a tenth of
// lightness apart, which is where each one reads as a different color.
const (
	rampSteps     = 7
	rampDarkest   = 0.3
	rampLightest  = 0.9
	rampLightStep = (rampLightest - rampDarkest) / (rampSteps - 1)
)

// Turns the hue by deg, then finds the closest color sRGB can actually show.
func rotate(h Hex, deg float64) Hex {
	if len(h) != HexLen {
		return h
	}

	lightness, a, b := hexToOklab(h)

	chroma := math.Hypot(a, b)
	if chroma < neutralChroma {
		return h // the same cutoff perceptualSortKey groups neutrals by
	}

	// Trig wraps on its own, so nothing has to normalize the angle back into a range.
	hue := math.Atan2(b, a) + deg*math.Pi/180

	return fitToSRGB(lightness, chroma, hue)
}

/*
h as a scale of rampSteps colors: hue held, lightness swept, chroma clamped to what the hue holds
at each step.

Clamping chroma is the opposite trade from fitToSRGB, which is why this doesn't call it. Holding
chroma snaps most steps back onto the few lightnesses that carry it, so 7 requests answer with 3
colors. A scale needs its lightnesses more than it needs one chroma.

A neutral has near zero chroma to clamp, so it ramps to grays without a special case. h is in its
own ramp only if its lightness lands on a step: the scale is near the input, not through it.
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

Nothing has to leave the gamut, so this doesn't call fitToSRGB. maxChromaAt gives the ceiling
and every step is below it by construction.

A neutral has no hue to sweep: unitHue would read an angle out of the rounding and answer a
scale in an invented hue, so the same cutoff the rotations gate on returns h repeated instead.
*/
func tones(h Hex) []Hex {
	out := make([]Hex, toneSteps)

	var lightness, a, b float64
	if len(h) == HexLen {
		lightness, a, b = hexToOklab(h)
	}

	// A value ParseHex would reject leaves a and b at zero, so one gate covers it too.
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
