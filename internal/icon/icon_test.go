package icon

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSlugsAreSortedAndUnique(t *testing.T) {
	all := slugs[:]

	require.True(t, slices.IsSorted(all), "Should be sorted: ParseSlug binary searches the set")
	require.Len(t, slices.Compact(slices.Clone(all)), len(all))
}

func TestDefaultIsInTheSet(t *testing.T) {
	require.Contains(t, slugs[:], Default)
}

func TestParseSlugAcceptsEveryEntry(t *testing.T) {
	for _, slug := range slugs {
		parsed, err := ParseSlug(string(slug))
		require.NoError(t, err)
		require.Equal(t, slug, parsed)
	}
}

func TestParseSlugRejectsUnknownEntry(t *testing.T) {
	for _, in := range []string{"", "squ", "square-dashed", "i-square", "<svg>", "  square", "SQUARE"} {
		t.Run(in, func(t *testing.T) {
			_, err := ParseSlug(in)
			require.ErrorIs(t, err, ErrUnknownIcon)
		})
	}
}
