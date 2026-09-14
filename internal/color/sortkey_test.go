package color

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func groupOf(h Hex) int  { return int(perceptualSortKey(h)) / groupStep }
func chromaOf(h Hex) int { return int(perceptualSortKey(h)) % lightnessStep }

func TestSortKeyPutsNeutralsInTheirOwnGroup(t *testing.T) {
	for _, h := range neutrals {
		t.Run(string(h), func(t *testing.T) {
			require.Equal(t, 0, groupOf(h))
		})
	}
}

func TestSortKeyKeepsNearNeutralsInTheirHueGroup(t *testing.T) {
	for _, h := range []Hex{"#f5f5dc", "#fff8dc", "#ffe4c4", "#bc8f8f"} {
		t.Run(string(h), func(t *testing.T) {
			require.NotEqual(t, 0, groupOf(h))
		})
	}
}

func TestSortKeyWalksHuesInSpectrumOrder(t *testing.T) {
	for _, tc := range []struct {
		hex   Hex
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
	ramp := []Hex{"#8b0000", "#b22222", "#dc143c", "#ff0000", "#f08080"}

	for i := 1; i < len(ramp); i++ {
		require.Less(t, Feel(ramp[i-1]), Feel(ramp[i]),
			"%s must sort before %s", ramp[i-1], ramp[i])
	}
}

func TestSortKeyPacksWithoutCarryingBetweenParts(t *testing.T) {
	white := Feel("#ffffff") // the largest a neutral can be
	darkestChromatic := Feel("#000001")

	require.Equal(t, 0, int(white)/groupStep)
	require.Less(t, white, darkestChromatic)
}

func TestSortKeyIsZeroForValueParseHexWouldReject(t *testing.T) {
	require.Zero(t, Feel("#fff"))
	require.Zero(t, Feel(""))
}
