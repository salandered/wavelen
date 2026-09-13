package color_test

import (
	"testing"

	"github.com/salandered/wavelen/internal/color"
	"github.com/stretchr/testify/require"
)

func complement(h color.Hex) color.Hex {
	return color.Complement.Colors(color.OKLab, h)[0]
}

func triad(h color.Hex) (second, third color.Hex) {
	out := color.Triad.Colors(color.OKLab, h)
	return out[0], out[1]
}

func bucketFrom(group, step int) int {
	return (group-1+step+12)%12 + 1
}

func TestComplementTurnsHueHalfCircle(t *testing.T) {
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

func TestTriadTurnsHueByThirds(t *testing.T) {
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

func TestAnalogousTurnsHueOneBucketEitherWay(t *testing.T) {
	out := color.Analogous.Colors(color.OKLab, "#ff0000")

	require.Len(t, out, 2)
	require.Equal(t, bucketFrom(groupOf("#ff0000"), -1), groupOf(out[0]))
	require.Equal(t, bucketFrom(groupOf("#ff0000"), 1), groupOf(out[1]))
}

func TestSplitComplementStraddlesComplement(t *testing.T) {
	out := color.SplitComplement.Colors(color.OKLab, "#ff0000")

	require.Len(t, out, 2)
	require.Equal(t, bucketFrom(groupOf(complement("#ff0000")), -1), groupOf(out[0]))
	require.Equal(t, bucketFrom(groupOf(complement("#ff0000")), 1), groupOf(out[1]))
}

func TestSquareContainsComplement(t *testing.T) {
	out := color.Square.Colors(color.OKLab, "#ff0000")

	require.Len(t, out, 3)
	require.Equal(t, complement("#ff0000"), out[1])
}

func TestComplementKeepsChroma(t *testing.T) {
	for _, h := range []color.Hex{"#4682b4", "#bc8f8f", "#00ff00"} {
		t.Run(string(h), func(t *testing.T) {
			require.InDelta(t, chromaOf(h), chromaOf(complement(h)), 3)
		})
	}
}

func TestComplementKeepsLightnessWhenOppositeHueHoldsChromaThere(t *testing.T) {
	for _, h := range []color.Hex{"#4682b4", "#bc8f8f"} {
		t.Run(string(h), func(t *testing.T) {
			require.InDelta(t, lightnessOf(h), lightnessOf(complement(h)), 1)
		})
	}
}

func TestComplementOfYellowIsVioletNotNearWhite(t *testing.T) {
	got := complement("#ffff00")

	require.Equal(t, color.Hex("#8d6aff"), got)
	require.Equal(t, 9, groupOf(got)) // 3 + 6, the same half circle every other color gets
	require.InDelta(t, chromaOf("#ffff00"), chromaOf(got), 3)
	require.Less(t, lightnessOf(got), lightnessOf("#ffff00")-300)
}

func TestComplementGivesUpChromaWhenNoLightnessCanHoldIt(t *testing.T) {
	got := complement("#ff0000")

	require.Equal(t, color.Hex("#00e5ff"), got)
	require.Less(t, chromaOf(got), chromaOf("#ff0000"))
	require.Greater(t, chromaOf(got), 100) // still a color, not a near neutral
}

// HSL tool answers #a5ef10 and #def543 for these. Ours are "more yellow" because the rotation is a
// true half circle in OkLCh where HSL's is nearer 154 degrees.
func TestComplementOfDarkVioletIsVividYellow(t *testing.T) {
	require.Equal(t, color.Hex("#fde900"), complement("#5a10ef"))
	require.Equal(t, color.Hex("#ffe100"), complement("#5a43f5"))
}

func TestComplementRoundTripsWhenNeitherStepLeavesGamut(t *testing.T) {
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

// Sweep also catches a NaN reaching the output.
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
			out := color.Ramp.Colors(color.OKLab, h)

			require.Len(t, out, 7)
			for i := 1; i < len(out); i++ {
				require.NotEqual(t, out[i-1], out[i])
				require.Greater(t, lightnessOf(out[i]), lightnessOf(out[i-1]))
			}
		})
	}
}

func TestRampBracketsInputLightness(t *testing.T) {
	for _, h := range []color.Hex{"#7b2ff7", "#ff6b35", "#1e90ff", "#2e8b57"} {
		t.Run(string(h), func(t *testing.T) {
			out := color.Ramp.Colors(color.OKLab, h)

			require.Less(t, lightnessOf(out[0]), lightnessOf(h))
			require.Greater(t, lightnessOf(out[len(out)-1]), lightnessOf(h))
		})
	}
}

// Chroma is clamped, the middle of the scale keeps the hue while the ends may run
// out of room for it.
func TestRampKeepsHueThroughMiddle(t *testing.T) {
	for _, h := range []color.Hex{"#7b2ff7", "#ff6b35", "#1e90ff", "#2e8b57", "#f5deb3"} {
		t.Run(string(h), func(t *testing.T) {
			out := color.Ramp.Colors(color.OKLab, h)
			require.Equal(t, groupOf(h), groupOf(out[len(out)/2]))
		})
	}
}

// Gray has no chroma to clamp, every step is a gray
func TestRampOfNeutralIsGrays(t *testing.T) {
	out := color.Ramp.Colors(color.OKLab, "#808080")

	require.Len(t, out, 7)
	for _, got := range out {
		require.Zero(t, groupOf(got))
		require.Equal(t, got[1:3], got[3:5])
		require.Equal(t, got[3:5], got[5:7])
	}
}

func TestRampOfValueParseHexWouldRejectIsInputRepeated(t *testing.T) {
	out := color.Ramp.Colors(color.OKLab, "#fff")

	require.Len(t, out, 7)
	for _, got := range out {
		require.Equal(t, color.Hex("#fff"), got)
	}
}

func TestTonesRunFromGrayToFullColor(t *testing.T) {
	for _, h := range []color.Hex{"#7b2ff7", "#ff6b35", "#1e90ff", "#2e8b57", "#f5deb3"} {
		t.Run(string(h), func(t *testing.T) {
			out := color.Tones.Colors(color.OKLab, h)

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
			out := color.Tones.Colors(color.OKLab, h)

			for i := 1; i < len(out); i++ {
				require.NotEqual(t, out[i-1], out[i])
				require.Greater(t, chromaOf(out[i]), chromaOf(out[i-1]))
			}
		})
	}
}

func TestTonesKeepLightness(t *testing.T) {
	for _, h := range []color.Hex{"#7b2ff7", "#ff6b35", "#1e90ff", "#2e8b57", "#f5deb3"} {
		t.Run(string(h), func(t *testing.T) {
			for _, got := range color.Tones.Colors(color.OKLab, h) {
				require.InDelta(t, lightnessOf(h), lightnessOf(got), 2)
			}
		})
	}
}

func TestTonesOfNeutralIsInputRepeated(t *testing.T) {
	for _, h := range append(neutrals, "#fff", "") {
		t.Run(string(h), func(t *testing.T) {
			out := color.Tones.Colors(color.OKLab, h)

			require.Len(t, out, 7)
			for _, got := range out {
				require.Equal(t, h, got)
			}
		})
	}
}
