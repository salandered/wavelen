package color

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContrastsOfBWForBlackAndWhite(t *testing.T) {
	onWhite, onBlack := contrastsOfBW("#ffffff")
	require.InDelta(t, 1, onWhite.Ratio, 1e-9)
	require.InDelta(t, 21, onBlack.Ratio, 1e-9)

	onWhite, onBlack = contrastsOfBW("#000000")
	require.InDelta(t, 21, onWhite.Ratio, 1e-9)
	require.InDelta(t, 1, onBlack.Ratio, 1e-9)
}

func TestContrastsOfBWMatchesRefColors(t *testing.T) {
	for _, tc := range []struct {
		hex          Hex
		white, black float64
		whiteLevel   Level
		blackLevel   Level
	}{
		{"#808080", 3.94, 5.31, LevelAALarge, LevelAA},
		{"#3cb371", 2.66, 7.87, LevelFail, LevelAAA},
		{"#d56784", 3.44, 6.10, LevelAALarge, LevelAA},
		{"#e7ff00", 1.12, 18.70, LevelFail, LevelAAA},
		{"#ff0000", 3.99, 5.25, LevelAALarge, LevelAA},
	} {
		t.Run(string(tc.hex), func(t *testing.T) {
			onWhite, onBlack := contrastsOfBW(tc.hex)

			require.Equal(t, tc.white, onWhite.Ratio)
			require.Equal(t, tc.black, onBlack.Ratio)
			require.Equal(t, tc.whiteLevel, onWhite.Level)
			require.Equal(t, tc.blackLevel, onBlack.Level)
		})
	}
}

func TestContrastsOfBWCloseToThresholdRationNotRounds(t *testing.T) {
	// #0099ff gives 2.999789 on white, 7.000493 on black
	onWhite, onBlack := contrastsOfBW("#0099ff")

	require.Equal(t, 2.99, onWhite.Ratio)
	require.Equal(t, LevelFail, onWhite.Level)

	require.Equal(t, 7.0, onBlack.Ratio)
	require.Equal(t, LevelAAA, onBlack.Level)
}
