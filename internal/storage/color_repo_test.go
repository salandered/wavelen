//go:build integration

package storage_test

import (
	"uuid"

	"github.com/salandered/wavelen/internal/collection"
	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/storage"
)

var unknownCollection = collection.ID(
	uuid.MustParse("00000000-0000-7000-8000-00000000dead"))

func (s *StorageSuite) TestAddColorReportsCreatedOnTheFirstInsert() {
	_, collectionID := s.createUserAndCollection("olya", "Olya")

	// when
	created, err := s.storage.AddColor(s.ctx(), collectionID, "#ff0000")

	// then
	s.Require().NoError(err)
	s.Require().True(created)
}

func (s *StorageSuite) TestAddColorReportsNotCreatedOnARepeat() {
	_, collectionID := s.createUserAndCollection("olya", "Olya")
	_, err := s.storage.AddColor(s.ctx(), collectionID, "#ff0000")
	s.Require().NoError(err)

	// when
	created, err := s.storage.AddColor(s.ctx(), collectionID, "#ff0000")

	// then
	s.Require().NoError(err)
	s.Require().False(created)

	page, err := s.storage.ListColors(s.ctx(), collectionID, storage.ListColorsParams{})
	s.Require().NoError(err)
	s.Require().Len(page.Colors, 1) // one is saved
}

func (s *StorageSuite) TestAddColorForAnUnknownCollection() {
	// when
	created, err := s.storage.AddColor(s.ctx(), unknownCollection, "#ff0000")

	// then
	s.Require().ErrorIs(err, storage.ErrNotFound)
	s.Require().False(created)
}

func (s *StorageSuite) TestCountColorsIsZeroForAnUnknownCollection() {
	n, err := s.storage.CountColors(s.ctx(), unknownCollection)

	s.Require().NoError(err)
	s.Require().Zero(n)
}

func (s *StorageSuite) TestCountColors() {
	_, graceCollection := s.createUserAndCollection("grace", "Grace")
	s.addColors(graceCollection, "#ff0000", "#00ff00", "#e0d253")

	// when
	n, err := s.storage.CountColors(s.ctx(), graceCollection)

	// then
	s.Require().NoError(err)
	s.Require().Equal(3, n)
}

func (s *StorageSuite) TestHasColorOk() {
	_, collectionID := s.createUserAndCollection("olya", "Olya")
	s.addColors(collectionID, "#ff0000")

	// when
	has, err := s.storage.HasColor(s.ctx(), collectionID, "#ff0000")

	// then
	s.Require().NoError(err)
	s.Require().True(has)
}

func (s *StorageSuite) TestHasColorNotOk() {
	_, collectionID := s.createUserAndCollection("olya", "Olya")
	s.addColors(collectionID, "#ff0000")

	// when
	has, err := s.storage.HasColor(s.ctx(), collectionID, "#00ff00")

	// then
	s.Require().NoError(err)
	s.Require().False(has)
}

func (s *StorageSuite) TestHasColorIsFalseForAnUnknownCollection() {
	has, err := s.storage.HasColor(s.ctx(), unknownCollection, "#ff0000")

	s.Require().NoError(err)
	s.Require().False(has)
}

func (s *StorageSuite) TestDeleteColorOk() {
	_, collectionID := s.createUserAndCollection("olya", "Olya")
	s.addColors(collectionID, "#ff0000", "#00ff00")

	// when
	err := s.storage.DeleteColor(s.ctx(), collectionID, "#ff0000")

	// then
	s.Require().NoError(err)

	page, err := s.storage.ListColors(s.ctx(), collectionID, storage.ListColorsParams{})
	s.Require().NoError(err)
	s.Require().Equal([]color.Hex{"#00ff00"}, hexesOf(page.Colors))
}

func (s *StorageSuite) TestDeleteColorTheCollectionDoesNotHave() {
	_, collectionID := s.createUserAndCollection("olya", "Olya")
	s.addColors(collectionID, "#ff0000")

	// when
	err := s.storage.DeleteColor(s.ctx(), collectionID, "#00ff00")

	// then
	s.Require().ErrorIs(err, storage.ErrNotFound)
}

func (s *StorageSuite) TestDeleteColorFromAnUnknownCollection() {
	err := s.storage.DeleteColor(s.ctx(), unknownCollection, "#ff0000")

	s.Require().ErrorIs(err, storage.ErrNotFound)
}

func (s *StorageSuite) TestAddColorConstraintRejectsInvalidHex() {
	_, collectionID := s.createUserAndCollection("olya", "Olya")

	for _, hex := range []color.Hex{"#FF0000", "ff0000", "#fff", ""} {
		s.Run(string(hex), func() {
			_, err := s.storage.AddColor(s.ctx(), collectionID, hex)
			s.Require().Error(err)
		})
	}
}

func (s *StorageSuite) TestListColorsReturnsNewestFirst() {
	_, collectionID := s.createUserAndCollection("olya", "Olya")
	s.addColors(collectionID, "#ff0000", "#00ff00", "#0000ff")

	// when
	// The zero value ListColorsParams (default).
	page, err := s.storage.ListColors(s.ctx(), collectionID, storage.ListColorsParams{})

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

func (s *StorageSuite) TestListColorsForUnknownCollectionIsEmpty() {
	page, err := s.storage.ListColors(s.ctx(), unknownCollection, storage.ListColorsParams{})

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
	_, collectionID := s.createUserAndCollection("olya", "Olya")
	s.addColors(collectionID, "#ff0000", "#00ff00", "#0000ff", "#123456")

	for _, c := range sortCases {
		s.Run(string(c.sort)+" "+string(c.order), func() {
			page, err := s.storage.ListColors(s.ctx(), collectionID,
				storage.ListColorsParams{Sort: c.sort, Order: c.order})

			s.Require().NoError(err)
			s.Require().False(page.HasMore)
			s.Require().Equal(c.want, hexesOf(page.Colors))
		})
	}
}

func (s *StorageSuite) TestListColorsPagingVisitsEveryRowExactlyOnceInEveryOrder() {
	_, collectionID := s.createUserAndCollection("olya", "Olya")
	s.addColors(collectionID, "#ff0000", "#00ff00", "#0000ff", "#123456")

	for _, c := range sortCases {
		s.Run(string(c.sort)+" "+string(c.order), func() {
			// four rows and a limit of two, so the last page is an exact multiple
			seen := s.pageThrough(collectionID,
				storage.ListColorsParams{Sort: c.sort, Order: c.order, Limit: 2})

			s.Require().Equal(c.want, seen)
		})
	}
}

// user takes the collection, then the collection takes its colors
func (s *StorageSuite) TestDeletingAUserCascadesToTheirColors() {
	userID, collectionID := s.createUserAndCollection("olya", "Olya")
	_, err := s.storage.AddColor(s.ctx(), collectionID, "#ff0000")
	s.Require().NoError(err)

	// when
	_, err = s.pool.Exec(s.ctx(), `DELETE FROM users WHERE id = $1`, userID)
	s.Require().NoError(err)

	// then
	var colors, collections int
	err = s.pool.QueryRow(s.ctx(),
		`SELECT count(*) FROM collection_colors WHERE collection_id = $1`,
		collectionID).Scan(&colors)
	s.Require().NoError(err)
	s.Require().Zero(colors)

	err = s.pool.QueryRow(s.ctx(),
		`SELECT count(*) FROM collections WHERE user_id = $1`, userID).Scan(&collections)
	s.Require().NoError(err)
	s.Require().Zero(collections)
}

// Utils

func (s *StorageSuite) addColors(collectionID collection.ID, hexes ...color.Hex) {
	for _, hex := range hexes {
		// one statement per row, so now() differs and no two rows share a created_at
		_, err := s.storage.AddColor(s.ctx(), collectionID, hex)
		s.Require().NoError(err)
	}
}

// Traverse the listing with p.Limit per page and returns every hex, ordered.
func (s *StorageSuite) pageThrough(
	collectionID collection.ID, p storage.ListColorsParams,
) []color.Hex {
	var seen []color.Hex
	for range 100 { // a HasMore that never clears must fail the test, not hang it
		page, err := s.storage.ListColors(s.ctx(), collectionID, p)
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
