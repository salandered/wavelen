package color_test

import (
	"testing"

	"github.com/salandered/wavelen/internal/color"
	"github.com/stretchr/testify/require"
)

var neutrals = []color.Hex{"#000000", "#696969", "#808080", "#d3d3d3", "#ffffff"}

// Test only what ParseHex adds, not strvalid.
func TestParseHexNormalizes(t *testing.T) {
	got, err := color.ParseHex("  FF0000 ")

	require.NoError(t, err)
	require.Equal(t, color.Hex("#ff0000"), got)
}

func TestParseHexRejectsMalformed(t *testing.T) {
	got, err := color.ParseHex("#fff")

	require.Error(t, err)
	require.Empty(t, got)
}

func TestHarmonyOfNeutralIsTheNeutral(t *testing.T) {
	for _, name := range color.HarmonyNames() {
		if name == color.Ramp {
			continue // the one harmony that answers a neutral with other neutrals
		}
		t.Run(string(name), func(t *testing.T) {
			for _, h := range neutrals {
				for _, got := range name.Colors(h) {
					require.Equal(t, h, got)
				}
			}
		})
	}
}

func TestUnknownHarmonyHasNoColors(t *testing.T) {
	require.Nil(t, color.Harmony("tetrad").Colors("#ff0000"))
}

func TestParseHarmonyAcceptsEveryType(t *testing.T) {
	for _, name := range color.HarmonyNames() {
		got, err := color.ParseHarmony(string(name))

		require.NoError(t, err)
		require.Equal(t, name, got)
	}
}

func TestParseHarmonyRejectsUnknown(t *testing.T) {
	got, err := color.ParseHarmony("tetrad")

	require.Empty(t, got)
	require.ErrorIs(t, err, color.ErrUnknownHarmony)
}

func TestParseHarmonyRejectsWrongCaseAndPadding(t *testing.T) {
	for _, s := range []string{"", "Triad", " triad", "triad "} {
		_, err := color.ParseHarmony(s)

		require.ErrorIs(t, err, color.ErrUnknownHarmony)
	}
}

func TestHarmonyNamesAreUniqueAndApplyEach(t *testing.T) {
	names := color.HarmonyNames()

	require.Len(t, names, 7)
	seen := make(map[color.Harmony]bool, len(names))
	for _, name := range names {
		require.False(t, seen[name], name)
		seen[name] = true
		require.NotEmpty(t, name.Colors("#ff0000"))
	}
}
