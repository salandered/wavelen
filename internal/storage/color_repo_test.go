//go:build integration

package storage_test

import (
	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/storage"
)

func (s *StorageSuite) TestAddColorReportsCreatedOnTheFirstInsert() {
	userID := s.createUser("ada@example.com", "Ada")

	// when
	created, err := s.storage.AddColor(s.ctx(), userID, "#ff0000")

	// then
	s.Require().NoError(err)
	s.Require().True(created)
}

func (s *StorageSuite) TestAddColorReportsNotCreatedOnARepeat() {
	userID := s.createUser("ada@example.com", "Ada")
	_, err := s.storage.AddColor(s.ctx(), userID, "#ff0000")
	s.Require().NoError(err)

	// when
	created, err := s.storage.AddColor(s.ctx(), userID, "#ff0000")

	// then
	s.Require().NoError(err)
	s.Require().False(created)

	saved, err := s.storage.ListColors(s.ctx(), userID)
	s.Require().NoError(err)
	s.Require().Len(saved, 1) // one is saved
}

func (s *StorageSuite) TestAddColorForAnUnknownUser() {
	// when
	created, err := s.storage.AddColor(s.ctx(), 999, "#ff0000")

	// then
	s.Require().ErrorIs(err, storage.ErrUserNotFound)
	s.Require().False(created)
}

func (s *StorageSuite) TestAddColorConstraintRejectsInvalidHex() {
	userID := s.createUser("ada@example.com", "Ada")

	for _, hex := range []color.Hex{"#FF0000", "ff0000", "#fff", ""} {
		s.Run(string(hex), func() {
			_, err := s.storage.AddColor(s.ctx(), userID, hex)
			s.Require().Error(err)
		})
	}
}

func (s *StorageSuite) TestListColorsReturnsNewestFirst() {
	userID := s.createUser("ada@example.com", "Ada")
	for _, hex := range []color.Hex{"#ff0000", "#00ff00", "#0000ff"} {
		_, err := s.storage.AddColor(s.ctx(), userID, hex)
		s.Require().NoError(err)
	}

	// when
	saved, err := s.storage.ListColors(s.ctx(), userID)

	// then
	s.Require().NoError(err)
	s.Require().Len(saved, 3)
	s.Require().Equal(color.Hex("#0000ff"), saved[0].Hex)
	s.Require().Equal(color.Hex("#00ff00"), saved[1].Hex)
	s.Require().Equal(color.Hex("#ff0000"), saved[2].Hex)

	for i := range len(saved) - 1 {
		s.Require().False(saved[i].CreatedAt.Before(saved[i+1].CreatedAt))
	}
}

// The MVP does not tell an unknown user apart from one who saved nothing.
func (s *StorageSuite) TestListColorsForAnUnknownUserIsEmpty() {
	saved, err := s.storage.ListColors(s.ctx(), 999)

	s.Require().NoError(err)
	s.Require().Empty(saved)
}

func (s *StorageSuite) TestDeletingAUserCascadesToTheirColors() {
	userID := s.createUser("ada@example.com", "Ada")
	_, err := s.storage.AddColor(s.ctx(), userID, "#ff0000")
	s.Require().NoError(err)

	// when
	_, err = s.pool.Exec(s.ctx(), `DELETE FROM users WHERE id = $1`, userID)
	s.Require().NoError(err)

	// then
	var remaining int
	err = s.pool.QueryRow(s.ctx(),
		`SELECT count(*) FROM user_colors WHERE user_id = $1`, userID).Scan(&remaining)
	s.Require().NoError(err)
	s.Require().Zero(remaining)
}
