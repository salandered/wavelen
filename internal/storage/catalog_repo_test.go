//go:build integration

package storage_test

import (
	"slices"

	"github.com/salandered/wavelen/internal/color"
)

func (s *StorageSuite) TestListCommonColorsReturnsOrderedByName() {
	// when
	common, err := s.storage.ListCommonColors(s.ctx())

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

// The seed is applied by the migration (hand written). Check that it survives ParseHex unchanged.
func (s *StorageSuite) TestSeededPaletteIsAlreadyNormalized() {
	common, err := s.storage.ListCommonColors(s.ctx())
	s.Require().NoError(err)

	for _, c := range common {
		parsed, err := color.ParseHex(string(c.Hex))
		s.Require().NoError(err, c.Name)
		s.Require().Equal(c.Hex, parsed, c.Name)
	}
}
