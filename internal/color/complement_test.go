package color_test

import (
	"testing"

	"github.com/salandered/wavelen/internal/color"
	"github.com/stretchr/testify/require"
)

const lightnessStep = 1_000

// Feel packs lightness in the middle of the key, so it is readable without exporting anything.
func lightnessOf(h color.Hex) int { return int(color.Feel(h)) % groupStep / lightnessStep }

var neutrals = []color.Hex{"#000000", "#696969", "#808080", "#d3d3d3", "#ffffff"}

// Half a circle is 6 of Feel's 12 buckets, so the group moves by exactly 6 and wraps.
func TestComplementTurnsTheHueHalfACircle(t *testing.T) {
	for _, tc := range []struct {
		hex   color.Hex
		group int
	}{
		{"#ff0000", 7},  // red -> teal
		{"#ffa500", 8},  // orange -> blue
		{"#00ffff", 12}, // cyan -> pink
		{"#0000ff", 3},  // blue -> olive, wrapping past 12
		{"#8a2be2", 4},
		{"#ff00ff", 5},
		{"#ff1493", 6},
		{"#4682b4", 2},
	} {
		t.Run(string(tc.hex), func(t *testing.T) {
			require.Equal(t, tc.group, groupOf(color.Complement(tc.hex)))
		})
	}
}

// A third of a circle is 4 buckets, twice that is 8.
func TestTriadTurnsTheHueByThirds(t *testing.T) {
	for _, tc := range []struct {
		hex           color.Hex
		second, third int
	}{
		{"#ff0000", 5, 9},
		{"#ff00ff", 3, 7}, // wraps past 12 twice
		{"#4682b4", 12, 4},
		{"#8a2be2", 2, 6},
	} {
		t.Run(string(tc.hex), func(t *testing.T) {
			second, third := color.Triad(tc.hex)
			require.Equal(t, tc.second, groupOf(second))
			require.Equal(t, tc.third, groupOf(third))
		})
	}
}

// The property that separates chroma reduction from clamping the three channels.
func TestComplementKeepsLightness(t *testing.T) {
	for _, h := range []color.Hex{
		"#ff0000", "#ffa500", "#ffff00", "#7fff00", "#00ff00",
		"#00ffff", "#0000ff", "#8a2be2", "#ff00ff", "#ff1493", "#4682b4",
	} {
		t.Run(string(h), func(t *testing.T) {
			require.InDelta(t, lightnessOf(h), lightnessOf(color.Complement(h)), 1)
		})
	}
}

// Only for a pair where neither step has to give up chroma. A color saturated enough to clip
// cannot come back - the chroma the gamut took is gone.
func TestComplementRoundTripsWhenNeitherStepLeavesTheGamut(t *testing.T) {
	for _, h := range []color.Hex{"#4682b4", "#bc8f8f"} {
		t.Run(string(h), func(t *testing.T) {
			require.Equal(t, h, color.Complement(color.Complement(h)))
		})
	}
}

func TestComplementLeavesNeutralsUnchanged(t *testing.T) {
	for _, h := range neutrals {
		t.Run(string(h), func(t *testing.T) {
			require.Equal(t, h, color.Complement(h))
		})
	}
}

func TestTriadOfANeutralIsThatNeutralTwice(t *testing.T) {
	for _, h := range neutrals {
		t.Run(string(h), func(t *testing.T) {
			second, third := color.Triad(h)
			require.Equal(t, h, second)
			require.Equal(t, h, third)
		})
	}
}

// Yellow is as light as sRGB gets while still being saturated, and no blue exists at that
// lightness. Holding lightness means the chroma goes instead, so the answer is near white
// rather than the violet a designer might expect. Pinned because it is the visible cost of
// the rule, not an accident.
func TestComplementOfYellowGoesPaleRatherThanDark(t *testing.T) {
	got := color.Complement("#ffff00")

	require.Equal(t, color.Hex("#f4f3ff"), got)
	require.Equal(t, lightnessOf("#ffff00"), lightnessOf(got))
	require.Zero(t, groupOf(got)) // so much chroma is gone that it lands back among the neutrals
}

func TestComplementReturnsTheInputForAValueParseHexWouldReject(t *testing.T) {
	require.Equal(t, color.Hex("#fff"), color.Complement("#fff"))
	require.Equal(t, color.Hex(""), color.Complement(""))
}

// A sweep, mostly to catch a NaN reaching the output through the gamut search.
//
// The lightness tolerance is 5 rather than 1 because of the byte grid, not the math. Near black
// one step of a channel is a big step in lightness, so the closest storable color to the right
// answer can be that far off. Above L=100 the worst case over this sweep is 2.
func TestComplementAnswersAStorableHexAtTheSameLightnessForEveryInput(t *testing.T) {
	const digits = "0123456789abcdef"
	const nearBlackTolerance = 5

	for r := range 16 {
		for g := range 16 {
			for b := range 16 {
				in := color.Hex([]byte{
					'#', digits[r], digits[r], digits[g], digits[g], digits[b], digits[b],
				})

				out := color.Complement(in)
				parsed, err := color.ParseHex(string(out))
				require.NoErrorf(t, err, "complement of %s was %q", in, out)
				require.Equal(t, out, parsed)
				require.InDeltaf(t, lightnessOf(in), lightnessOf(out), nearBlackTolerance,
					"complement of %s", in)
			}
		}
	}
}
