package shades

import (
	"fmt"
	"testing"

	"github.com/salandered/wavelen/internal/color"
	"github.com/stretchr/testify/require"
)

func TestGridIsThirteenFamiliesOfTen(t *testing.T) {
	require.Len(t, families, 13)
	for _, f := range families {
		require.Len(t, f.Colors, ShadeCount, f.Name)
	}
}

func TestHexesAreUniqueAndNormalized(t *testing.T) {
	seen := make(map[color.Hex]bool, 130)
	for _, f := range families {
		for _, c := range f.Colors {
			parsed, err := color.NewHex(string(c.Hex))
			require.NoError(t, err, string(c.Hex))
			require.Equal(t, c.Hex, parsed)

			require.False(t, seen[c.Hex], string(c.Hex))
			seen[c.Hex] = true
		}
	}
}

func TestNamesAreFamilyAndShadeIndex(t *testing.T) {
	for _, f := range families {
		for i, c := range f.Colors {
			require.Equal(t, fmt.Sprintf("%s-%d", f.Name, i), c.Name)
		}
	}
}

func TestFamiliesHandsOutACopy(t *testing.T) {
	got := Families()
	got[0].Name = "changed"
	got[0].Colors[0].Hex = "#000000"

	require.Equal(t, "gray", Families()[0].Name)
	require.Equal(t, color.Hex("#f8f9fa"), Families()[0].Colors[0].Hex)
}
