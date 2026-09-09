package collection_test

import (
	"strings"
	"testing"

	"github.com/salandered/wavelen/internal/collection"
	"github.com/stretchr/testify/require"
)

func TestNormalizeNameTrimsSurroundingWhitespace(t *testing.T) {
	got, err := collection.NormalizeName("  Sunset palette \n")

	require.NoError(t, err)
	require.Equal(t, "Sunset palette", got)
}

func TestNormalizeNameKeepsCaseAndPunctuation(t *testing.T) {
	for _, in := range []string{"Main", "UPPER", "with-dash", "with_score", "a.b", "két szín"} {
		t.Run(in, func(t *testing.T) {
			got, err := collection.NormalizeName(in)

			require.NoError(t, err)
			require.Equal(t, in, got)
		})
	}
}

func TestNormalizeNameRejectsEmptyAndOverlong(t *testing.T) {
	for name, in := range map[string]string{
		"empty":          "",
		"whitespace":     "   ",
		"over max runes": strings.Repeat("a", collection.MaxNameLen+1),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := collection.NormalizeName(in)

			require.ErrorContains(t, err, "collection name")
		})
	}
}

func TestNormalizeNameCountsRunesNotBytes(t *testing.T) {
	// N two-byte runes is 2N bytes
	got, err := collection.NormalizeName(strings.Repeat("é", collection.MaxNameLen))

	require.NoError(t, err)
	require.Len(t, []rune(got), collection.MaxNameLen)
}
