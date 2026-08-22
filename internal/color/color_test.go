package color_test

import (
	"testing"

	"github.com/salandered/wavelen/internal/color"
	"github.com/stretchr/testify/require"
)

func TestParseHexNormalizesToLowercaseWithLeadingHash(t *testing.T) {
	tests := []struct {
		in   string
		want color.Hex
	}{
		{"#ff0000", "#ff0000"},
		{"#FF0000", "#ff0000"},
		{"ff0000", "#ff0000"},
		{"FF0000", "#ff0000"},
		{"  #AbCdEf  ", "#abcdef"},
		{"\t000000\n", "#000000"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := color.ParseHex(tt.in)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseHexRejectsMalformedInput(t *testing.T) {
	tests := map[string]string{
		"empty":            "",
		"hash only":        "#",
		"three digits":     "#fff",
		"seven digits":     "#ff00000",
		"non hex letters":  "#ff00gg",
		"inner space":      "ff 000",
		"dash":             "#ff-000",
		"all letters":      "zzzzzz",
		"two hashes":       "##ff0000",
		"rgba":             "#ff0000ff",
		"named color":      "red",
		"leading hash gap": "# ff0000",
	}
	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := color.ParseHex(in)
			require.ErrorIs(t, err, color.ErrInvalidHex)
		})
	}
}
