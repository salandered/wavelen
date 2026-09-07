package palette

import (
	"slices"
	"strings"
	"testing"

	"github.com/salandered/wavelen/internal/color"
	"github.com/stretchr/testify/require"
)

func TestPaletteFeelMatchesTheFormula(t *testing.T) {
	require.Len(t, entries, 100)
	for _, e := range entries {
		require.Equal(
			t,
			color.Feel(e.Hex),
			e.Feel,
			"the formula moved but entries weren't recalculated")
	}
}

func TestPaletteHexesAndNamesAreUnique(t *testing.T) {
	hexes := make(map[color.Hex]bool, len(entries))
	names := make(map[string]bool, len(entries))
	for _, e := range entries {
		require.False(t, hexes[e.Hex], string(e.Hex))
		require.False(t, names[e.Name], e.Name)
		hexes[e.Hex], names[e.Name] = true, true
	}
}

func TestPaletteHexesAreNormalized(t *testing.T) {
	for _, e := range entries {
		parsed, err := color.ParseHex(string(e.Hex))
		require.NoError(t, err, string(e.Hex))
		require.Equal(t, e.Hex, parsed)
	}
}

func TestListSortParamsZeroValueMeansNameAsc(t *testing.T) {
	common, err := List(SortParams{})
	require.NoError(t, err)
	require.Len(t, common, 100)

	names := make([]string, 0, len(common))
	for _, c := range common {
		names = append(names, c.Name)
	}
	require.True(t, slices.IsSorted(names))
}

func TestListSortsByEitherFieldInEitherDirection(t *testing.T) {
	for _, tc := range []struct {
		name   string
		params SortParams
		key    func(color.Common) string
	}{
		{"name asc", SortParams{Sort: SortByName}, func(c color.Common) string { return c.Name }},
		{"name desc", SortParams{Desc: true}, func(c color.Common) string { return c.Name }},
		{"hex asc", SortParams{Sort: SortByHex}, func(c color.Common) string { return string(c.Hex) }},
		{"hex desc", SortParams{Sort: SortByHex, Desc: true},
			func(c color.Common) string { return string(c.Hex) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			common, err := List(tc.params)
			require.NoError(t, err)
			require.Len(t, common, 100)

			keys := make([]string, 0, len(common))
			for _, c := range common {
				keys = append(keys, tc.key(c))
			}
			if tc.params.Desc {
				require.True(t, slices.IsSortedFunc(keys, func(a, b string) int {
					return strings.Compare(b, a)
				}))
				return
			}
			require.True(t, slices.IsSorted(keys))
		})
	}
}

func TestListReturnsNewSlice(t *testing.T) {
	first, err := List(SortParams{})
	require.NoError(t, err)
	first[0] = color.Common{Hex: "#ffffff", Name: "clobbered"}

	second, err := List(SortParams{})
	require.NoError(t, err)
	require.NotEqual(t, "clobbered", second[0].Name)
}

func TestListRejectsUnknownSort(t *testing.T) {
	_, err := List(SortParams{Sort: "created_at"})
	require.ErrorIs(t, err, ErrInvalidSort)
}

func TestParseSortAcceptsDocumentedValues(t *testing.T) {
	for _, raw := range []string{"name", "hex", "color"} {
		sort, err := ParseSort(raw)
		require.NoError(t, err)
		require.Equal(t, Sort(raw), sort)
	}
}

func TestParseSortRejectsUnknownValue(t *testing.T) {
	_, err := ParseSort("created_at")
	require.ErrorIs(t, err, ErrInvalidSort)
}
