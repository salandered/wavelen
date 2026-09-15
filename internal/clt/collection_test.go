package clt_test

import (
	"strings"
	"testing"

	"github.com/salandered/wavelen/internal/clt"
	"github.com/stretchr/testify/require"
)

func TestNormalizeNameTrimsSurroundingWhitespace(t *testing.T) {
	got, err := clt.NormalizeName("  Sunset palette \n")

	require.NoError(t, err)
	require.Equal(t, "Sunset palette", got)
}

func TestNormalizeNameKeepsCaseAndPunctuation(t *testing.T) {
	for _, in := range []string{"Main", "UPPER", "with-dash", "with_score", "a.b", "két szín"} {
		t.Run(in, func(t *testing.T) {
			got, err := clt.NormalizeName(in)

			require.NoError(t, err)
			require.Equal(t, in, got)
		})
	}
}

func TestNormalizeNameRejectsEmptyAndOverlong(t *testing.T) {
	for name, in := range map[string]string{
		"empty":          "",
		"whitespace":     "   ",
		"over max runes": strings.Repeat("a", clt.MaxNameLen+1),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := clt.NormalizeName(in)

			require.ErrorContains(t, err, "collection name")
		})
	}
}

func TestNormalizeNameCountsRunesNotBytes(t *testing.T) {
	// N two-byte runes is 2N bytes
	got, err := clt.NormalizeName(strings.Repeat("é", clt.MaxNameLen))

	require.NoError(t, err)
	require.Len(t, []rune(got), clt.MaxNameLen)
}
