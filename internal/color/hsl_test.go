package color

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func topAndBottom(h Hex) (top, bottom int) {
	r, g, b := h.RByte(), h.GByte(), h.BByte()
	return max(r, g, b), min(r, g, b)
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
		harmony Harmony
		want    []Hex
	}{
		{Complement, []Hex{"#5ccdcd"}},
		{SplitComplement, []Hex{"#5ccd95", "#5c95cd"}},
		{Triad, []Hex{"#5ccd5c", "#5c5ccd"}},
		{Analogous, []Hex{"#cd5c95", "#cd955c"}},
		{Square, []Hex{"#95cd5c", "#5ccdcd", "#955ccd"}},
	} {
		t.Run(string(tc.harmony), func(t *testing.T) {
			require.Equal(t, tc.want, tc.harmony.Colors(SpaceHSL, "#cd5c5c"))
		})
	}
}

func TestHSLTriadOfYellowIsCyanAndMagenta(t *testing.T) {
	require.Equal(t,
		[]Hex{"#00ffff", "#ff00ff"},
		Triad.Colors(SpaceHSL, "#ffff00"))
}

// OkLCh holds chroma instead and lets lightness move
func TestHSLRotationHoldsTopAndBottomChannel(t *testing.T) {
	for _, base := range []Hex{"#cd5c5c", "#1f9d55", "#4682b4", "#ffff00", "#8a2be2"} {
		t.Run(string(base), func(t *testing.T) {
			top, bottom := topAndBottom(base)

			for _, name := range []Harmony{
				Complement, SplitComplement, Triad,
				Analogous, Square,
			} {
				for _, got := range name.Colors(SpaceHSL, base) {
					gotTop, gotBottom := topAndBottom(got)
					require.Equalf(t, top, gotTop, "%s of %s was %s", name, base, got)
					require.Equalf(t, bottom, gotBottom, "%s of %s was %s", name, base, got)
				}
			}
		})
	}
}

// #887777 is chroma .02114 and its complement is .01975
func TestHSLRotationStopsWhereItLandsUnderNeutralCutoff(t *testing.T) {
	there := Complement.Colors(SpaceHSL, "#887777")[0]

	require.Equal(t, Hex("#778888"), there)
	require.Equal(t, there, Complement.Colors(SpaceHSL, there)[0])
}

func TestSweepsAnswerSameInBothSpaces(t *testing.T) {
	for _, name := range []Harmony{Ramp, Tones} {
		t.Run(string(name), func(t *testing.T) {
			for _, base := range []Hex{"#cd5c5c", "#1f9d55", "#ffff00"} {
				require.Equal(t,
					name.Colors(SpaceOkLab, base),
					name.Colors(SpaceHSL, base))
			}
		})
	}
}

// HSV

func TestHexToHSVFollowsHexToHSLClosedForm(t *testing.T) {
	for _, h := range []Hex{
		"#ffffff", "#000000", "#808080", "#3cb371", "#ce5f14", "#d56784", "#000011",
	} {
		hslHue, saturation, lightness := hexToHSL(h)
		hsvHue, hsvSaturation, value := hexToHSV(h)

		require.InDeltaf(t, hslHue, hsvHue, 1e-12, "hue of %s", h)

		require.InDeltaf(t, lightness+saturation*min(lightness, 1-lightness), value, 1e-12,
			"value of %s", h)

		if value == 0 {
			require.Zerof(t, hsvSaturation, "saturation of %s", h)
			continue
		}
		require.InDeltaf(t, 2*(1-lightness/value), hsvSaturation, 1e-12, "saturation of %s", h)
	}
}

// #000011 divides to 1.0000000000000002, a float artifact rather than a color; hexToHSL clamps it.
func TestHexToHSLKeepsSaturationAtOrBelowOne(t *testing.T) {
	_, saturation, _ := hexToHSL("#000011")

	require.LessOrEqual(t, saturation, 1.0)
}
