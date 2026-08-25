//go:build integration

package storage_test

import (
	"cmp"
	"slices"

	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/storage"
)

func (s *StorageSuite) TestListCommonColorsDefaultsToNameAscending() {
	// when
	common, err := s.storage.ListCommonColors(s.ctx(), storage.ListCommonColorsParams{})

	// then
	s.Require().NoError(err)
	s.Require().Len(common, 100)

	names := make([]string, 0, len(common))
	for _, c := range common {
		names = append(names, c.Name)
	}
	s.Require().True(slices.IsSorted(names))

	s.Require().Contains(common, color.Common{Hex: "#000000", Name: "black"})
	s.Require().Contains(common, color.Common{Hex: "#ff0000", Name: "red"})
}

// The seed is applied by the migration. Check it survives ParseHex
func (s *StorageSuite) TestSeededPaletteIsAlreadyNormalized() {
	common, err := s.storage.ListCommonColors(s.ctx(), storage.ListCommonColorsParams{})
	s.Require().NoError(err)

	for _, c := range common {
		parsed, err := color.ParseHex(string(c.Hex))
		s.Require().NoError(err, c.Name)
		s.Require().Equal(c.Hex, parsed, c.Name) // hex is unchanged
	}
}

func (s *StorageSuite) TestListCommonColorsSortsByEitherColumnInEitherDirection() {
	for _, tc := range []struct {
		name   string
		params storage.ListCommonColorsParams
		sorted func([]color.Common) bool
	}{
		{
			name:   "hex ascending",
			params: storage.ListCommonColorsParams{Sort: storage.CatalogSortByHex},
			sorted: func(c []color.Common) bool { return slices.IsSortedFunc(c, byHexAsc) },
		},
		{
			name: "hex descending",
			params: storage.ListCommonColorsParams{
				Sort: storage.CatalogSortByHex, Order: storage.OrderDesc,
			},
			sorted: func(c []color.Common) bool {
				return slices.IsSortedFunc(c, reverse(byHexAsc))
			},
		},
		{
			name: "name descending",
			params: storage.ListCommonColorsParams{
				Sort: storage.CatalogSortByName, Order: storage.OrderDesc,
			},
			sorted: func(c []color.Common) bool {
				return slices.IsSortedFunc(c, reverse(byNameAsc))
			},
		},
	} {
		s.Run(tc.name, func() {
			common, err := s.storage.ListCommonColors(s.ctx(), tc.params)

			s.Require().NoError(err)
			s.Require().Len(common, 100)
			s.Require().True(tc.sorted(common))
		})
	}
}

func (s *StorageSuite) TestListCommonColorsRejectsAnUnknownSortInsteadOfInterpolatingIt() {
	_, err := s.storage.ListCommonColors(s.ctx(), storage.ListCommonColorsParams{
		Sort: "name; DROP TABLE common_colors",
	})

	s.Require().ErrorIs(err, storage.ErrInvalidCatalogSort)
}

func byHexAsc(a, b color.Common) int { return cmp.Compare(a.Hex, b.Hex) }

func byNameAsc(a, b color.Common) int { return cmp.Compare(a.Name, b.Name) }

func reverse(less func(a, b color.Common) int) func(a, b color.Common) int {
	return func(a, b color.Common) int { return -less(a, b) }
}
