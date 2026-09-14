package color

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// red opens group 1, so one bucket back wraps to 12
func TestAnalogousTurnsHueOneBucketEitherWay(t *testing.T) {
	out := Analogous.Colors(SpaceOkLab, "#ff0000")

	require.Len(t, out, 2)
	require.Equal(t, 12, groupOf(out[0]))
	require.Equal(t, 2, groupOf(out[1]))
}

// the complement of red is group 7
func TestSplitComplementStraddlesComplement(t *testing.T) {
	out := SplitComplement.Colors(SpaceOkLab, "#ff0000")

	require.Len(t, out, 2)
	require.Equal(t, 6, groupOf(out[0]))
	require.Equal(t, 8, groupOf(out[1]))
}

// Gray has no chroma to clamp, every step is a gray
func TestRampOfNeutralIsGrays(t *testing.T) {
	out := Ramp.Colors(SpaceOkLab, "#808080")

	require.Len(t, out, 7)
	for _, got := range out {
		require.Equal(t, got[1:3], got[3:5])
		require.Equal(t, got[3:5], got[5:7])
	}
}

func TestRampOfValueParseHexWouldRejectIsInputRepeated(t *testing.T) {
	out := Ramp.Colors(SpaceOkLab, "#fff")

	require.Len(t, out, 7)
	for _, got := range out {
		require.Equal(t, Hex("#fff"), got)
	}
}

func TestTonesRunFromGrayToFullColor(t *testing.T) {
	for _, h := range []Hex{"#7b2ff7", "#ff6b35", "#1e90ff", "#2e8b57", "#f5deb3"} {
		t.Run(string(h), func(t *testing.T) {
			out := Tones.Colors(SpaceOkLab, h)

			require.Len(t, out, 7)
			require.Zero(t, groupOf(out[0]))
			require.Equal(t, groupOf(h), groupOf(out[len(out)-1]))
			require.Greater(t, chromaOf(out[len(out)-1]), chromaOf(h)-3)
		})
	}
}

func TestTonesRiseInChromaOverSevenSteps(t *testing.T) {
	for _, h := range []Hex{"#7b2ff7", "#ff6b35", "#1e90ff", "#2e8b57", "#f5deb3"} {
		t.Run(string(h), func(t *testing.T) {
			out := Tones.Colors(SpaceOkLab, h)

			for i := 1; i < len(out); i++ {
				require.NotEqual(t, out[i-1], out[i])
				require.Greater(t, chromaOf(out[i]), chromaOf(out[i-1]))
			}
		})
	}
}

func TestTonesOfNeutralIsInputRepeated(t *testing.T) {
	for _, h := range append(neutrals, "#fff", "") {
		t.Run(string(h), func(t *testing.T) {
			out := Tones.Colors(SpaceOkLab, h)

			require.Len(t, out, 7)
			for _, got := range out {
				require.Equal(t, h, got)
			}
		})
	}
}
