package color_test

import (
	"testing"

	"github.com/salandered/wavelen/internal/color"
	"github.com/stretchr/testify/require"
)

func complement(h color.Hex) color.Hex {
	return color.Complement.Colors(h)[0]
}

func triad(h color.Hex) (second, third color.Hex) {
	out := color.Triad.Colors(h)
	return out[0], out[1]
}

func bucketFrom(group, step int) int {
	return (group-1+step+12)%12 + 1
}

// Half a circle is 6 of the sort key's 12 hue buckets, so the group moves by exactly 6 and wraps.
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
			require.Equal(t, tc.group, groupOf(complement(tc.hex)))
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
			second, third := triad(tc.hex)
			require.Equal(t, tc.second, groupOf(second))
			require.Equal(t, tc.third, groupOf(third))
		})
	}
}

// 30 degrees is one bucket either side, the neighbours red sits between.
func TestAnalogousTurnsTheHueOneBucketEitherWay(t *testing.T) {
	out := color.Analogous.Colors("#ff0000")

	require.Len(t, out, 2)
	require.Equal(t, bucketFrom(groupOf("#ff0000"), -1), groupOf(out[0]))
	require.Equal(t, bucketFrom(groupOf("#ff0000"), 1), groupOf(out[1]))
}

// 150 and 210 straddle the complement's 180, one bucket short of it on either side.
func TestSplitComplementStraddlesTheComplement(t *testing.T) {
	out := color.SplitComplement.Colors("#ff0000")

	require.Len(t, out, 2)
	require.Equal(t, bucketFrom(groupOf(complement("#ff0000")), -1), groupOf(out[0]))
	require.Equal(t, bucketFrom(groupOf(complement("#ff0000")), 1), groupOf(out[1]))
}

// The middle corner of the square is the complement.
func TestSquareContainsTheComplement(t *testing.T) {
	out := color.Square.Colors("#ff0000")

	require.Len(t, out, 3)
	require.Equal(t, complement("#ff0000"), out[1])
}

// Chroma is the quantity held, so the pair is equally colorful even where it is not equally
// bright. The delta absorbs the two byte roundings on the way through.
func TestComplementKeepsChroma(t *testing.T) {
	for _, h := range []color.Hex{"#4682b4", "#bc8f8f", "#00ff00"} {
		t.Run(string(h), func(t *testing.T) {
			require.InDelta(t, chromaOf(h), chromaOf(complement(h)), 3)
		})
	}
}

// A color whose chroma the opposite hue can carry at its own lightness stays where it is. The
// delta is one byte rounding, not a move: these are the pair that round-trips exactly below.
func TestComplementKeepsLightnessWhenTheOppositeHueCanHoldTheChromaThere(t *testing.T) {
	for _, h := range []color.Hex{"#4682b4", "#bc8f8f"} {
		t.Run(string(h), func(t *testing.T) {
			require.InDelta(t, lightnessOf(h), lightnessOf(complement(h)), 1)
		})
	}
}

// Yellow is as light as sRGB gets while still being saturated, and no violet is that bright.
// Lightness is what gives way, so the answer is a violet at the same chroma rather than the
// near-white that holding lightness would have produced.
func TestComplementOfYellowIsAVioletNotANearWhite(t *testing.T) {
	got := complement("#ffff00")

	require.Equal(t, color.Hex("#8d6aff"), got)
	require.Equal(t, 9, groupOf(got)) // 3 + 6, the same half circle every other color gets
	require.InDelta(t, chromaOf("#ffff00"), chromaOf(got), 3)
	require.Less(t, lightnessOf(got), lightnessOf("#ffff00")-300)
}

// Chroma gives way only when no lightness at that hue carries it: sRGB has no cyan as saturated
// as its reds. Lightness still moves to wherever the most of it survives.
func TestComplementGivesUpChromaWhenNoLightnessCanHoldIt(t *testing.T) {
	got := complement("#ff0000")

	require.Equal(t, color.Hex("#00e5ff"), got)
	require.Less(t, chromaOf(got), chromaOf("#ff0000"))
	require.Greater(t, chromaOf(got), 100) // still a color, not the near neutral it used to be
}

// The two that started the rule change. An HSL tool answers #a5ef10 and #def543 for these; ours
// are yellower because the rotation is a true half circle in OkLCh where HSL's is nearer 154
// degrees. What matters is that both are vivid - holding lightness answered #685f00 and #7b6c00.
func TestComplementOfADarkVioletIsAVividYellow(t *testing.T) {
	require.Equal(t, color.Hex("#fde900"), complement("#5a10ef"))
	require.Equal(t, color.Hex("#ffe100"), complement("#5a43f5"))
}

// Exact only where neither hop has to give anything up. A color saturated enough to lose chroma
// on the way out cannot get it back on the way home.
func TestComplementRoundTripsWhenNeitherStepLeavesTheGamut(t *testing.T) {
	for _, h := range []color.Hex{"#4682b4", "#bc8f8f"} {
		t.Run(string(h), func(t *testing.T) {
			require.Equal(t, h, complement(complement(h)))
		})
	}
}

func TestComplementReturnsInputForValueParseHexWouldReject(t *testing.T) {
	require.Equal(t, color.Hex("#fff"), complement("#fff"))
	require.Equal(t, color.Hex(""), complement(""))
}

// The sweep catches a NaN reaching the output, and asserts the property the previous rule broke:
// a color with a hue always answers with a color that has one. Under the old rule yellow came
// back a near white, which is the sort key's neutral group.
func TestComplementOfChromaticColorIsNeverNeutral(t *testing.T) {
	const digits = "0123456789abcdef"

	for r := range 16 {
		for g := range 16 {
			for b := range 16 {
				in := color.Hex([]byte{
					'#', digits[r], digits[r], digits[g], digits[g], digits[b], digits[b],
				})

				out := complement(in)
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

func TestRampRisesInLightnessOverSevenDistinctSteps(t *testing.T) {
	for _, h := range []color.Hex{"#7b2ff7", "#ff6b35", "#1e90ff", "#2e8b57", "#f5deb3"} {
		t.Run(string(h), func(t *testing.T) {
			out := color.Ramp.Colors(h)

			require.Len(t, out, 7)
			for i := 1; i < len(out); i++ {
				require.NotEqual(t, out[i-1], out[i])
				require.Greater(t, lightnessOf(out[i]), lightnessOf(out[i-1]))
			}
		})
	}
}

// The scale is around the input, not through it, so the input's own lightness sits inside the
// range without having to be one of the steps.
func TestRampBracketsTheInputLightness(t *testing.T) {
	for _, h := range []color.Hex{"#7b2ff7", "#ff6b35", "#1e90ff", "#2e8b57"} {
		t.Run(string(h), func(t *testing.T) {
			out := color.Ramp.Colors(h)

			require.Less(t, lightnessOf(out[0]), lightnessOf(h))
			require.Greater(t, lightnessOf(out[len(out)-1]), lightnessOf(h))
		})
	}
}

// Chroma is clamped, not held, so the middle of the scale keeps the hue while the ends may run
// out of room for it.
func TestRampKeepsTheHueThroughTheMiddle(t *testing.T) {
	for _, h := range []color.Hex{"#7b2ff7", "#ff6b35", "#1e90ff", "#2e8b57", "#f5deb3"} {
		t.Run(string(h), func(t *testing.T) {
			out := color.Ramp.Colors(h)
			require.Equal(t, groupOf(h), groupOf(out[len(out)/2]))
		})
	}
}

// A gray has no chroma to clamp, so every step is a gray rather than the input repeated.
func TestRampOfNeutralIsGrays(t *testing.T) {
	out := color.Ramp.Colors("#808080")

	require.Len(t, out, 7)
	for _, got := range out {
		require.Zero(t, groupOf(got))
		require.Equal(t, got[1:3], got[3:5])
		require.Equal(t, got[3:5], got[5:7])
	}
}

func TestRampOfValueParseHexWouldRejectIsInputRepeated(t *testing.T) {
	out := color.Ramp.Colors("#fff")

	require.Len(t, out, 7)
	for _, got := range out {
		require.Equal(t, color.Hex("#fff"), got)
	}
}

// Step 0 is a gray as light as the input, the last step is the most chroma the hue holds there.
func TestTonesRunFromGrayToTheFullColor(t *testing.T) {
	for _, h := range []color.Hex{"#7b2ff7", "#ff6b35", "#1e90ff", "#2e8b57", "#f5deb3"} {
		t.Run(string(h), func(t *testing.T) {
			out := color.Tones.Colors(h)

			require.Len(t, out, 7)
			require.Zero(t, groupOf(out[0]))
			require.Equal(t, groupOf(h), groupOf(out[len(out)-1]))
			require.Greater(t, chromaOf(out[len(out)-1]), chromaOf(h)-3)
		})
	}
}

func TestTonesRiseInChromaOverSevenSteps(t *testing.T) {
	for _, h := range []color.Hex{"#7b2ff7", "#ff6b35", "#1e90ff", "#2e8b57", "#f5deb3"} {
		t.Run(string(h), func(t *testing.T) {
			out := color.Tones.Colors(h)

			for i := 1; i < len(out); i++ {
				require.NotEqual(t, out[i-1], out[i])
				require.Greater(t, chromaOf(out[i]), chromaOf(out[i-1]))
			}
		})
	}
}

// The sweep is across the chroma axis only, so every step keeps the lightness it started at.
func TestTonesKeepTheLightness(t *testing.T) {
	for _, h := range []color.Hex{"#7b2ff7", "#ff6b35", "#1e90ff", "#2e8b57", "#f5deb3"} {
		t.Run(string(h), func(t *testing.T) {
			for _, got := range color.Tones.Colors(h) {
				require.InDelta(t, lightnessOf(h), lightnessOf(got), 2)
			}
		})
	}
}

// A gray has no hue to sweep, so it comes back repeated rather than in an invented one.
func TestTonesOfNeutralIsInputRepeated(t *testing.T) {
	for _, h := range append(neutrals, "#fff", "") {
		t.Run(string(h), func(t *testing.T) {
			out := color.Tones.Colors(h)

			require.Len(t, out, 7)
			for _, got := range out {
				require.Equal(t, h, got)
			}
		})
	}
}
