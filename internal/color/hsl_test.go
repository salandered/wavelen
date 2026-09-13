package color_test

import (
	"strconv"
	"testing"

	"github.com/salandered/wavelen/internal/color"
	"github.com/stretchr/testify/require"
)

func channels(h color.Hex) [3]int {
	var out [3]int
	for i := range out {
		v, err := strconv.ParseUint(string(h[1+2*i:3+2*i]), 16, 8)
		if err != nil {
			panic(err)
		}
		out[i] = int(v)
	}
	return out
}

func topAndBottom(h color.Hex) (top, bottom int) {
	c := channels(h)
	return max(c[0], c[1], c[2]), min(c[0], c[1], c[2])
}

/*
The values Figma and colordesigner.io answer for the same base.

Two of them are off by one from those sites: #955ccd where Figma shows #945ccd, and #cd5c95 and
#cd955c where both sites show #cd5c94 and #cd945c. #cd5c5c is at saturation .5305, which lands the
middle channel of a 30 degree step on exactly 148.5. math.Round takes the half up to 0x95 and
those sites take it down to 0x94. Figma itself does both, floor at +30 and +270 and round up at
+90, so there is no one answer to match here.
*/
func TestHSLRotationMatchesColorWheelSites(t *testing.T) {
	for _, tc := range []struct {
		harmony color.Harmony
		want    []color.Hex
	}{
		{color.Complement, []color.Hex{"#5ccdcd"}},
		{color.SplitComplement, []color.Hex{"#5ccd95", "#5c95cd"}},
		{color.Triad, []color.Hex{"#5ccd5c", "#5c5ccd"}},
		{color.Analogous, []color.Hex{"#cd5c95", "#cd955c"}},
		{color.Square, []color.Hex{"#95cd5c", "#5ccdcd", "#955ccd"}},
	} {
		t.Run(string(tc.harmony), func(t *testing.T) {
			require.Equal(t, tc.want, tc.harmony.Colors(color.HSL, "#cd5c5c"))
		})
	}
}

func TestHSLTriadOfYellowIsCyanAndMagenta(t *testing.T) {
	require.Equal(t,
		[]color.Hex{"#00ffff", "#ff00ff"},
		color.Triad.Colors(color.HSL, "#ffff00"))
}

// OkLCh holds chroma instead and lets lightness move
func TestHSLRotationHoldsTopAndBottomChannel(t *testing.T) {
	for _, base := range []color.Hex{"#cd5c5c", "#1f9d55", "#4682b4", "#ffff00", "#8a2be2"} {
		t.Run(string(base), func(t *testing.T) {
			top, bottom := topAndBottom(base)

			for _, name := range []color.Harmony{
				color.Complement, color.SplitComplement, color.Triad,
				color.Analogous, color.Square,
			} {
				for _, got := range name.Colors(color.HSL, base) {
					gotTop, gotBottom := topAndBottom(got)
					require.Equalf(t, top, gotTop, "%s of %s was %s", name, base, got)
					require.Equalf(t, bottom, gotBottom, "%s of %s was %s", name, base, got)
				}
			}
		})
	}
}

func TestHSLComplementRoundTripsWhereBothEndsHaveHue(t *testing.T) {
	const digits = "0123456789abcdef"

	for r := range 16 {
		for g := range 16 {
			for b := range 16 {
				in := color.Hex([]byte{
					'#', digits[r], digits[r], digits[g], digits[g], digits[b], digits[b],
				})

				there := color.Complement.Colors(color.HSL, in)[0]
				if groupOf(in) == 0 || groupOf(there) == 0 {
					continue
				}

				back := color.Complement.Colors(color.HSL, there)[0]
				require.Equalf(t, in, back, "%s -> %s -> %s", in, there, back)
			}
		}
	}
}

// #887777 is chroma .02114 and its complement is .01975
func TestHSLRotationStopsWhereItLandsUnderNeutralCutoff(t *testing.T) {
	there := color.Complement.Colors(color.HSL, "#887777")[0]

	require.Equal(t, color.Hex("#778888"), there)
	require.Equal(t, there, color.Complement.Colors(color.HSL, there)[0])
}

func TestSweepsAnswerSameInBothSpaces(t *testing.T) {
	for _, name := range []color.Harmony{color.Ramp, color.Tones} {
		t.Run(string(name), func(t *testing.T) {
			for _, base := range []color.Hex{"#cd5c5c", "#1f9d55", "#ffff00"} {
				require.Equal(t,
					name.Colors(color.OKLab, base),
					name.Colors(color.HSL, base))
			}
		})
	}
}

func TestParseSpaceAcceptsBothWheels(t *testing.T) {
	for _, want := range []color.Space{color.OKLab, color.HSL} {
		got, err := color.ParseSpace(string(want))

		require.NoError(t, err)
		require.Equal(t, want, got)
	}
}

func TestParseSpaceRejectsUnknownEmptyAndWrongCase(t *testing.T) {
	for _, s := range []string{"", "hsv", "OKLab", " hsl", "hsl "} {
		got, err := color.ParseSpace(s)

		require.Empty(t, got)
		require.ErrorIs(t, err, color.ErrUnknownSpace)
	}
}
