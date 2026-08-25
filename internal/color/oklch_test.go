package color_test

import (
	"testing"

	"github.com/salandered/wavelen/internal/color"
	"github.com/stretchr/testify/require"
)

const groupStep = 10_000_000

func groupOf(h color.Hex) int { return int(color.Feel(h)) / groupStep }

func TestSortKeyPutsNeutralsInTheirOwnGroup(t *testing.T) {
	for _, h := range []color.Hex{"#000000", "#696969", "#808080", "#d3d3d3", "#ffffff"} {
		t.Run(string(h), func(t *testing.T) {
			require.Equal(t, 0, groupOf(h))
		})
	}
}

// The cutoff has to clear true grays without swallowing the near-neutrals.
func TestSortKeyKeepsNearNeutralsInTheirHueGroup(t *testing.T) {
	for _, h := range []color.Hex{"#f5f5dc", "#fff8dc", "#ffe4c4", "#bc8f8f"} {
		t.Run(string(h), func(t *testing.T) {
			require.NotEqual(t, 0, groupOf(h))
		})
	}
}

func TestSortKeyWalksTheHuesInSpectrumOrder(t *testing.T) {
	for _, tc := range []struct {
		hex   color.Hex
		group int
	}{
		{"#ff0000", 1},  // the 20 degree origin is what opens group 1 with red
		{"#ffa500", 2},  // orange
		{"#ffff00", 3},  // yellow
		{"#7fff00", 4},  // chartreuse
		{"#00ff00", 5},  // green
		{"#00ffff", 6},  // cyan
		{"#0000ff", 9},  // blue
		{"#8a2be2", 10}, // blueviolet
		{"#ff00ff", 11}, // magenta
		{"#ff1493", 12}, // deeppink
	} {
		t.Run(string(tc.hex), func(t *testing.T) {
			require.Equal(t, tc.group, groupOf(tc.hex))
		})
	}
}

func TestSortKeyOrdersOneHueFamilyDarkToLight(t *testing.T) {
	ramp := []color.Hex{"#8b0000", "#b22222", "#dc143c", "#ff0000", "#f08080"}

	for i := 1; i < len(ramp); i++ {
		require.Less(t, color.Feel(ramp[i-1]), color.Feel(ramp[i]),
			"%s must sort before %s", ramp[i-1], ramp[i])
	}
}

// Every part has room below it, so none can carry into the one above.
func TestSortKeyPacksWithoutCarryingBetweenParts(t *testing.T) {
	white := color.Feel("#ffffff") // the largest a neutral can be
	darkestChromatic := color.Feel("#000001")

	require.Equal(t, 0, int(white)/groupStep)
	require.Less(t, white, darkestChromatic)
}

func TestSortKeyIsZeroForAValueParseHexWouldReject(t *testing.T) {
	require.Zero(t, color.Feel("#fff"))
	require.Zero(t, color.Feel(""))
}
