package color

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDescribeRGB(t *testing.T) {
	for _, tc := range []struct {
		hex  Hex
		want RGB
	}{
		{"#000000", RGB{R: 0, G: 0, B: 0}},
		{"#ffffff", RGB{R: 255, G: 255, B: 255}},
		{"#3cb371", RGB{R: 60, G: 179, B: 113}},
		{"#ce5f14", RGB{R: 206, G: 95, B: 20}},
	} {
		t.Run(string(tc.hex), func(t *testing.T) {
			require.Equal(t, tc.want, Describe(tc.hex).RGB)
		})
	}
}

// HSL and HSV read off htmlcolorcodes, colorhexa and the macOS picker.
func TestDescribeMatchesReferenceTools(t *testing.T) {
	for _, tc := range []struct {
		hex   Hex
		hsl   HSL
		hsv   HSV
		oklch OkLabCh
	}{
		{"#ffffff",
			HSL{Hue: 0, Saturation: 0, Lightness: 100},
			HSV{Hue: 0, Saturation: 0, Value: 100},
			OkLabCh{Lightness: 100, Chroma: 0, Hue: 0}},
		{"#000000",
			HSL{Hue: 0, Saturation: 0, Lightness: 0},
			HSV{Hue: 0, Saturation: 0, Value: 0},
			OkLabCh{Lightness: 0, Chroma: 0, Hue: 0}},
		{"#808080",
			HSL{Hue: 0, Saturation: 0, Lightness: 50},
			HSV{Hue: 0, Saturation: 0, Value: 50},
			OkLabCh{Lightness: 60, Chroma: 0, Hue: 0}},
		{"#3cb371",
			HSL{Hue: 147, Saturation: 50, Lightness: 47},
			HSV{Hue: 147, Saturation: 66, Value: 70},
			OkLabCh{Lightness: 68.4, Chroma: 0.144, Hue: 155}},
		{"#d56784",
			HSL{Hue: 344, Saturation: 57, Lightness: 62},
			HSV{Hue: 344, Saturation: 52, Value: 84},
			OkLabCh{Lightness: 65.2, Chroma: 0.141, Hue: 5}},
		{"#ce5f14",
			HSL{Hue: 24, Saturation: 82, Lightness: 44},
			HSV{Hue: 24, Saturation: 90, Value: 81},
			OkLabCh{Lightness: 61.3, Chroma: 0.16, Hue: 48}},
		{"#e7ff00",
			HSL{Hue: 66, Saturation: 100, Lightness: 50},
			HSV{Hue: 66, Saturation: 100, Value: 100},
			OkLabCh{Lightness: 94.9, Chroma: 0.218, Hue: 116}},
		{"#ff0000",
			HSL{Hue: 0, Saturation: 100, Lightness: 50},
			HSV{Hue: 0, Saturation: 100, Value: 100},
			OkLabCh{Lightness: 62.8, Chroma: 0.258, Hue: 29}},
	} {
		t.Run(string(tc.hex), func(t *testing.T) {
			got := Describe(tc.hex)

			require.Equal(t, tc.hsl, got.HSL)
			require.Equal(t, tc.hsv, got.HSV)
			require.Equal(t, tc.oklch, got.OkLCh)
		})
	}
}

func TestDescribeNeutralHasNoHueInAnyRepresentation(t *testing.T) {
	for _, hex := range neutrals {
		t.Run(string(hex), func(t *testing.T) {
			got := Describe(hex)

			require.Zero(t, got.HSL.Hue)
			require.Zero(t, got.HSL.Saturation)
			require.Zero(t, got.HSV.Hue)
			require.Zero(t, got.HSV.Saturation)
			require.Zero(t, got.OkLCh.Hue)
		})
	}
}

func TestDescribeReturnsInputForValueParseHexWouldReject(t *testing.T) {
	require.Equal(t, Info{Hex: "#fff"}, Describe("#fff"))
	require.Equal(t, Info{}, Describe(""))
}
