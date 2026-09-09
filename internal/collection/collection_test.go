package collection_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/salandered/wavelen/internal/collection"
	"github.com/stretchr/testify/require"
)

const canonicalID = "01999999-7777-7777-8888-999999999999"

func TestParseIDAcceptsCanonicalForm(t *testing.T) {
	id, err := collection.ParseID(canonicalID)

	require.NoError(t, err)
	require.Equal(t, canonicalID, id.String())
}

func TestParseIDCaseInsensitive(t *testing.T) {
	upper, err := collection.ParseID(strings.ToUpper(canonicalID))
	require.NoError(t, err)

	lower, err := collection.ParseID(canonicalID)
	require.NoError(t, err)

	require.Equal(t, lower, upper)
}

func TestParseIDRejectsInvalidUUID(t *testing.T) {
	tests := map[string]string{
		"empty":            "",
		"word":             "main",
		"digits":           "42",
		"too short":        "01999999-7777-7777-8888-9999999999",
		"trailing garbage": canonicalID + "x",
		"path traversal":   "../users",

		// kinda ok but not canon
		"braced":            "{" + canonicalID + "}",
		"urn":               "urn:uuid:" + canonicalID,
		"no dashes":         "01999999777777778888999999999999",
		"surrounding space": " " + canonicalID + " ",
	}
	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := collection.ParseID(in)

			require.ErrorIs(t, err, collection.ErrInvalidID)
		})
	}
}

func TestParseIDTakesNilUUID(t *testing.T) {
	id, err := collection.ParseID(uuid.Nil.String())

	require.NoError(t, err)
	require.Equal(t, collection.ID{}, id)
}

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
