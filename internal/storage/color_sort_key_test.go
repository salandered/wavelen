//go:build integration

package storage_test

import (
	"github.com/salandered/wavelen/internal/color"
)

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
