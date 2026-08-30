package color_test

import (
	"testing"

	"github.com/salandered/wavelen/internal/color"
	"github.com/stretchr/testify/require"
)

const lightnessStep = 1_000

// Feel packs lightness in the middle of the key and chroma at the bottom, so both are readable
// without exporting anything. Chroma is in thousandths, and Feel caps it at 999.
func lightnessOf(h color.Hex) int { return int(color.Feel(h)) % groupStep / lightnessStep }
func chromaOf(h color.Hex) int    { return int(color.Feel(h)) % lightnessStep }

var neutrals = []color.Hex{"#000000", "#696969", "#808080", "#d3d3d3", "#ffffff"}

// Half a circle is 6 of Feel's 12 buckets, so the group moves by exactly 6 and wraps.
func TestComplementTurnsTheHueHalfACircle(t *testing.T) {
	for _, tc := range []struct {
		hex   color.Hex
		group int
	}{
		{"#ff0000", 7},  // red -> cyan
		{"#ffa500", 8},  // orange -> blue
		{"#00ffff", 12}, // cyan -> pink
		{"#0000ff", 3},  // blue -> yellow, wrapping past 12
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

// Chroma is the quantity held, so the pair is equally colorful even where it is not equally
// bright. The delta absorbs the two byte roundings on the way through.
func TestComplementKeepsChroma(t *testing.T) {
	for _, h := range []color.Hex{"#4682b4", "#bc8f8f", "#00ff00", "#ffff00"} {
		t.Run(string(h), func(t *testing.T) {
			require.InDelta(t, chromaOf(h), chromaOf(color.Complement(h)), 3)
		})
	}
}

// A color whose chroma the opposite hue can carry at its own lightness stays where it is. The
// delta is one byte rounding, not a move: these are the pair that round-trips exactly below.
func TestComplementKeepsLightnessWhenTheOppositeHueCanHoldTheChromaThere(t *testing.T) {
	for _, h := range []color.Hex{"#4682b4", "#bc8f8f"} {
		t.Run(string(h), func(t *testing.T) {
			require.InDelta(t, lightnessOf(h), lightnessOf(color.Complement(h)), 1)
		})
	}
}

// Yellow is as light as sRGB gets while still being saturated, and no violet is that bright.
// Lightness is what gives way, so the answer is a violet at the same chroma rather than the
// near-white that holding lightness would have produced.
func TestComplementOfYellowIsAVioletNotANearWhite(t *testing.T) {
	got := color.Complement("#ffff00")

	require.Equal(t, color.Hex("#8d6aff"), got)
	require.Equal(t, 9, groupOf(got)) // 3 + 6, the same half circle every other color gets
	require.InDelta(t, chromaOf("#ffff00"), chromaOf(got), 3)
	require.Less(t, lightnessOf(got), lightnessOf("#ffff00")-300)
}

// Chroma gives way only when no lightness at that hue carries it: sRGB has no cyan as saturated
// as its reds. Lightness still moves to wherever the most of it survives.
func TestComplementGivesUpChromaWhenNoLightnessCanHoldIt(t *testing.T) {
	got := color.Complement("#ff0000")

	require.Equal(t, color.Hex("#00e5ff"), got)
	require.Less(t, chromaOf(got), chromaOf("#ff0000"))
	require.Greater(t, chromaOf(got), 100) // still a color, not the near neutral it used to be
}

// The two that started the rule change. An HSL tool answers #a5ef10 and #def543 for these; ours
// are yellower because the rotation is a true half circle in OkLCh where HSL's is nearer 154
// degrees. What matters is that both are vivid - holding lightness answered #685f00 and #7b6c00.
func TestComplementOfADarkVioletIsAVividYellow(t *testing.T) {
	require.Equal(t, color.Hex("#fde900"), color.Complement("#5a10ef"))
	require.Equal(t, color.Hex("#ffe100"), color.Complement("#5a43f5"))
}

// Exact only where neither hop has to give anything up. A color saturated enough to lose chroma
// on the way out cannot get it back on the way home.
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

func TestComplementReturnsTheInputForAValueParseHexWouldReject(t *testing.T) {
	require.Equal(t, color.Hex("#fff"), color.Complement("#fff"))
	require.Equal(t, color.Hex(""), color.Complement(""))
}

// The sweep catches a NaN reaching the output, and asserts the property the previous rule broke:
// a color with a hue always answers with a color that has one. Under the old rule yellow came
// back a near white, which is Feel's neutral group.
func TestComplementOfAChromaticColorIsNeverANeutral(t *testing.T) {
	const digits = "0123456789abcdef"

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

				if groupOf(in) != 0 {
					require.NotZerof(t, groupOf(out), "complement of %s was %s", in, out)
				}
			}
		}
	}
}
