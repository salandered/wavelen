//go:build integration

package storage_test

import (
	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
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

	page, err := s.storage.ListColors(s.ctx(), userID, storage.ListColorsParams{})
	s.Require().NoError(err)
	s.Require().Len(page.Colors, 1) // one is saved
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
	s.addColors(userID, "#ff0000", "#00ff00", "#0000ff")

	// when
	// The zero value ListColorsParams (default).

	page, err := s.storage.ListColors(s.ctx(), userID, storage.ListColorsParams{})

	// then
	s.Require().NoError(err)
	s.Require().False(page.HasMore)
	s.Require().Equal(
		[]color.Hex{"#0000ff", "#00ff00", "#ff0000"},
		hexesOf(page.Colors))

	saved := page.Colors
	for i := range len(saved) - 1 {
		s.Require().False(saved[i].CreatedAt.Before(saved[i+1].CreatedAt))
	}
}

// The MVP does not tell an unknown user apart from one who saved nothing.
func (s *StorageSuite) TestListColorsForAnUnknownUserIsEmpty() {
	page, err := s.storage.ListColors(s.ctx(), 999, storage.ListColorsParams{})

	s.Require().NoError(err)
	s.Require().Empty(page.Colors)
	s.Require().False(page.HasMore)
}

// Inserts one row per statement, so the timestamps would differ
var sortCases = []struct {
	sort  storage.ColorSort
	order storage.SortOrder
	want  []color.Hex
}{
	{storage.SortByCreatedAt, storage.OrderDesc, []color.Hex{"#123456", "#0000ff", "#00ff00", "#ff0000"}},
	{storage.SortByCreatedAt, storage.OrderAsc, []color.Hex{"#ff0000", "#00ff00", "#0000ff", "#123456"}},
	{storage.SortByHex, storage.OrderAsc, []color.Hex{"#0000ff", "#00ff00", "#123456", "#ff0000"}},
	{storage.SortByHex, storage.OrderDesc, []color.Hex{"#ff0000", "#123456", "#00ff00", "#0000ff"}},
	// red, green, a dark desaturated blue, blue - hue groups 1, 5, 8, 9
	{storage.SortByColor, storage.OrderAsc, []color.Hex{"#ff0000", "#00ff00", "#123456", "#0000ff"}},
	{storage.SortByColor, storage.OrderDesc, []color.Hex{"#0000ff", "#123456", "#00ff00", "#ff0000"}},
}

func (s *StorageSuite) TestListColorsOrdersByTheRequestedSortAndOrder() {
	userID := s.createUser("ada@example.com", "Ada")
	s.addColors(userID, "#ff0000", "#00ff00", "#0000ff", "#123456")

	for _, c := range sortCases {
		s.Run(string(c.sort)+" "+string(c.order), func() {
			page, err := s.storage.ListColors(s.ctx(), userID,
				storage.ListColorsParams{Sort: c.sort, Order: c.order})

			s.Require().NoError(err)
			s.Require().False(page.HasMore)
			s.Require().Equal(c.want, hexesOf(page.Colors))
		})
	}
}

func (s *StorageSuite) TestListColorsPagingVisitsEveryRowExactlyOnceInEveryOrder() {
	userID := s.createUser("ada@example.com", "Ada")
	s.addColors(userID, "#ff0000", "#00ff00", "#0000ff", "#123456")

	for _, c := range sortCases {
		s.Run(string(c.sort)+" "+string(c.order), func() {
			// four rows and a limit of two, so the last page is an exact multiple
			seen := s.pageThrough(userID,
				storage.ListColorsParams{Sort: c.sort, Order: c.order, Limit: 2})

			s.Require().Equal(c.want, seen)
		})
	}
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

// Utils

func (s *StorageSuite) addColors(userID user.ID, hexes ...color.Hex) {
	for _, hex := range hexes {
		// one statement per row, so now() differs and no two rows share a created_at
		_, err := s.storage.AddColor(s.ctx(), userID, hex)
		s.Require().NoError(err)
	}
}

// Walks the listing with p.Limit per page and returns every hex it saw, ordered.
func (s *StorageSuite) pageThrough(userID user.ID, p storage.ListColorsParams) []color.Hex {
	var seen []color.Hex
	for range 100 { // a HasMore that never clears must fail the test, not hang it
		page, err := s.storage.ListColors(s.ctx(), userID, p)
		s.Require().NoError(err)

		seen = append(seen, hexesOf(page.Colors)...)
		if !page.HasMore {
			return seen
		}
		p.After = cursorOf(page.Colors[len(page.Colors)-1])
	}
	s.Require().Fail("paging did not terminate")
	return seen
}

func cursorOf(last color.Color) *storage.ColorCursor {
	return &storage.ColorCursor{CreatedAt: last.CreatedAt, Hex: last.Hex}
}

func hexesOf(colors []color.Color) []color.Hex {
	hexes := make([]color.Hex, 0, len(colors))
	for _, c := range colors {
		hexes = append(hexes, c.Hex)
	}
	return hexes
}
