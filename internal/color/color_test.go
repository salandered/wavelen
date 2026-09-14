package color

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var neutrals = []Hex{"#000000", "#696969", "#808080", "#d3d3d3", "#ffffff"}

func TestHarmonyOfNeutralIsNeutral(t *testing.T) {
	for _, name := range HarmonyNames() {
		if name == Ramp {
			continue // ramp answers a neutral with other neutrals
		}
		for _, space := range []Space{SpaceOkLab, SpaceHSL} {
			t.Run(string(name)+"/"+string(space), func(t *testing.T) {
				for _, h := range neutrals {
					for _, got := range name.Colors(space, h) {
						require.Equal(t, h, got)
					}
				}
			})
		}
	}
}

func TestUnknownHarmonyHasNoColors(t *testing.T) {
	require.Nil(t, Harmony("tetrad").Colors(SpaceOkLab, "#ff0000"))
}

func TestParseHarmonyAcceptsEveryType(t *testing.T) {
	for _, name := range HarmonyNames() {
		got, err := ParseHarmony(string(name))

		require.NoError(t, err)
		require.Equal(t, name, got)
	}
}

func TestParseHarmonyRejectsUnknown(t *testing.T) {
	got, err := ParseHarmony("tetrad")

	require.Empty(t, got)
	require.ErrorIs(t, err, ErrUnknownHarmony)
}

func TestParseHarmonyRejectsWrongCaseAndPadding(t *testing.T) {
	for _, s := range []string{"", "Triad", " triad", "triad "} {
		_, err := ParseHarmony(s)

		require.ErrorIs(t, err, ErrUnknownHarmony)
	}
}

func TestHarmonyNamesAreUniqueAndApplyEach(t *testing.T) {
	names := HarmonyNames()

	require.Len(t, names, 7)
	seen := make(map[Harmony]bool, len(names))
	for _, name := range names {
		require.False(t, seen[name], name)
		seen[name] = true
		require.NotEmpty(t, name.Colors(SpaceOkLab, "#ff0000"))
	}
}

func TestParseSpaceAcceptsBothWheels(t *testing.T) {
	for _, want := range []Space{SpaceOkLab, SpaceHSL} {
		got, err := ParseSpace(string(want))

		require.NoError(t, err)
		require.Equal(t, want, got)
	}
}

func TestParseSpaceRejectsUnknownEmptyAndWrongCase(t *testing.T) {
	for _, s := range []string{"", "hsv", "OKLab", " hsl", "hsl "} {
		got, err := ParseSpace(s)

		require.Empty(t, got)
		require.ErrorIs(t, err, ErrUnknownSpace)
	}
}
