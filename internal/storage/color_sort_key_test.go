//go:build integration

package storage_test

import (
	"slices"

	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/storage"
)

// The seed has color_key as literals.
// Should alert if the formula moves and the migration does not
func (s *StorageSuite) TestSeededPaletteColorKeysMatchTheFormula() {
	rows, err := s.pool.Query(s.ctx(), `SELECT hex, color_key FROM common_colors`)
	s.Require().NoError(err)

	seen := 0
	for rows.Next() {
		var hex color.Hex
		var stored int32
		s.Require().NoError(rows.Scan(&hex, &stored))
		s.Require().Equal(color.Feel(hex), stored, string(hex))
		seen++
	}
	s.Require().NoError(rows.Err())
	s.Require().Equal(100, seen)
}

func (s *StorageSuite) TestAddColorStoresTheKeyItSortsBy() {
	userID := s.createUser("olya", "Olya")
	s.addColors(userID, "#ff0000", "#123456")

	rows, err := s.pool.Query(s.ctx(),
		`SELECT hex, color_key FROM user_colors WHERE user_id = $1`, userID)
	s.Require().NoError(err)

	seen := 0
	for rows.Next() {
		var hex color.Hex
		var stored int32
		s.Require().NoError(rows.Scan(&hex, &stored))
		s.Require().Equal(color.Feel(hex), stored, string(hex))
		seen++
	}
	s.Require().NoError(rows.Err())
	s.Require().Equal(2, seen)
}

func (s *StorageSuite) TestPaletteInColorOrderRunsNeutralsThenHueGroups() {
	common, err := s.storage.ListCommonColors(s.ctx(),
		storage.ListCommonColorsParams{Sort: storage.CatalogSortByColor})
	s.Require().NoError(err)
	s.Require().Len(common, 100)

	groups := make([]int, 0, len(common))
	for _, c := range common {
		groups = append(groups, int(color.Feel(c.Hex))/10_000_000)
	}

	s.Require().True(slices.IsSorted(groups), "a group must not reappear after a later one")
	s.Require().Equal(0, groups[0])
	s.Require().Equal("black", common[0].Name)
	s.Require().Equal("white", common[7].Name) // the neutrals end light
	s.Require().Equal(12, groups[len(groups)-1])
	s.Require().Equal("pink", common[len(common)-1].Name)
}
