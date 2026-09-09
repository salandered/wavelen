//go:build integration

package storage_test

import (
	"github.com/salandered/wavelen/internal/collection"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
)

// Create

func (s *StorageSuite) TestCreateCollectionReturnsGeneratedIDAndCreatedAt() {
	userID := s.createUser("olya", "Olya")

	col, err := s.storage.CreateCollection(s.ctx(), userID, "Sunset", false)

	s.Require().NoError(err)
	s.Require().NotEqual(collection.ID{}, col.ID)
	s.Require().Equal("Sunset", col.Name)
	s.Require().False(col.IsDefault)
	s.Require().False(col.CreatedAt.IsZero())
}

func (s *StorageSuite) TestCreateCollectionForUnknownUser() {
	_, err := s.storage.CreateCollection(s.ctx(), 999, "Sunset", false)

	s.Require().ErrorIs(err, storage.ErrUserNotFound)
}

// collections_one_default_per_user
func (s *StorageSuite) TestCreateSecondDefaultForOneUser() {
	userID, _ := s.createUserAndCollection("olya", "Olya")

	_, err := s.storage.CreateCollection(s.ctx(), userID, "Another", true)

	s.Require().Error(err)
}

func (s *StorageSuite) TestTwoUsersHaveDefaultClt() {
	_, olyaCollection := s.createUserAndCollection("olya", "Olya")
	_, graceCollection := s.createUserAndCollection("grace", "Grace")

	s.Require().NotEqual(olyaCollection, graceCollection)
}

// List and count

func (s *StorageSuite) TestListCollectionsReturnsOldestFirstStartingWithDefaultClt() {
	userID, defaultID := s.createUserAndCollection("olya", "Olya")
	sunset, err := s.storage.CreateCollection(s.ctx(), userID, "Sunset", false)
	s.Require().NoError(err)
	ocean, err := s.storage.CreateCollection(s.ctx(), userID, "Ocean", false)
	s.Require().NoError(err)

	// when
	got, err := s.storage.ListCollections(s.ctx(), userID)

	// then
	s.Require().NoError(err)
	s.Require().Equal([]collection.ID{defaultID, sunset.ID, ocean.ID}, idsOf(got))
	s.Require().True(got[0].IsDefault)
	s.Require().False(got[1].IsDefault)
}

func (s *StorageSuite) TestListCollectionsSkipsAnotherUserClts() {
	olyaID, olyaCollection := s.createUserAndCollection("olya", "Olya")
	_, graceCollection := s.createUserAndCollection("grace", "Grace")

	got, err := s.storage.ListCollections(s.ctx(), olyaID)

	s.Require().NoError(err)
	s.Require().Equal([]collection.ID{olyaCollection}, idsOf(got))
	s.Require().NotContains(idsOf(got), graceCollection)
}

func (s *StorageSuite) TestListCollectionsForUnknownUserIsEmpty() {
	got, err := s.storage.ListCollections(s.ctx(), 999)

	s.Require().NoError(err)
	s.Require().Empty(got)
}

func (s *StorageSuite) TestCountCollectionsCountsTheDefault() {
	userID, _ := s.createUserAndCollection("olya", "Olya")
	_, err := s.storage.CreateCollection(s.ctx(), userID, "Sunset", false)
	s.Require().NoError(err)

	n, err := s.storage.CountCollections(s.ctx(), userID)

	s.Require().NoError(err)
	s.Require().Equal(2, n)
}

func (s *StorageSuite) TestCountCollectionsZeroForUnknownUser() {
	n, err := s.storage.CountCollections(s.ctx(), 999)

	s.Require().NoError(err)
	s.Require().Zero(n)
}

// Read one

func (s *StorageSuite) TestCollectionByID() {
	userID, collectionID := s.createUserAndCollection("olya", "Olya")

	col, err := s.storage.CollectionByID(s.ctx(), userID, collectionID)

	s.Require().NoError(err)
	s.Require().Equal(collectionID, col.ID)
	s.Require().Equal(testCollectionName, col.Name)
	s.Require().True(col.IsDefault)
	s.Require().False(col.CreatedAt.IsZero())
}

func (s *StorageSuite) TestCollectionByIDAnotherUserOwns() {
	olyaID, _ := s.createUserAndCollection("olya", "Olya")
	_, graceCollection := s.createUserAndCollection("grace", "Grace")

	_, err := s.storage.CollectionByID(s.ctx(), olyaID, graceCollection)

	s.Require().ErrorIs(err, storage.ErrNotFound)
}

func (s *StorageSuite) TestCollectionByIDUnknownCollection() {
	userID := s.createUser("olya", "Olya")

	_, err := s.storage.CollectionByID(s.ctx(), userID, unknownCollection)

	s.Require().ErrorIs(err, storage.ErrNotFound)
}

// Resolve and lock

func (s *StorageSuite) TestResolveCollectionAnswersWithTheSameID() {
	userID, collectionID := s.createUserAndCollection("olya", "Olya")

	got, err := s.storage.ResolveCollection(s.ctx(), userID, collectionID)

	s.Require().NoError(err)
	s.Require().Equal(collectionID, got)
}

func (s *StorageSuite) TestResolveCollectionAnotherUserOwns() {
	olyaID, _ := s.createUserAndCollection("olya", "Olya")
	_, graceCollection := s.createUserAndCollection("grace", "Grace")

	_, err := s.storage.ResolveCollection(s.ctx(), olyaID, graceCollection)

	s.Require().ErrorIs(err, storage.ErrNotFound)
}

func (s *StorageSuite) TestLockCollectionAnswersWithTheSameID() {
	userID, collectionID := s.createUserAndCollection("olya", "Olya")

	got, err := s.storage.LockCollection(s.ctx(), userID, collectionID)

	s.Require().NoError(err)
	s.Require().Equal(collectionID, got)
}

func (s *StorageSuite) TestLockCollectionAnotherUserOwns() {
	olyaID, _ := s.createUserAndCollection("olya", "Olya")
	_, graceCollection := s.createUserAndCollection("grace", "Grace")

	_, err := s.storage.LockCollection(s.ctx(), olyaID, graceCollection)

	s.Require().ErrorIs(err, storage.ErrNotFound)
}

func (s *StorageSuite) TestLockCollectionUnknownCollection() {
	_, err := s.storage.LockCollection(s.ctx(), 999, unknownCollection)

	s.Require().ErrorIs(err, storage.ErrNotFound)
}

// Delete

func (s *StorageSuite) TestDeleteCollectionCascadesToItsColors() {
	userID, collectionID := s.createUserAndCollection("olya", "Olya")
	s.addColors(collectionID, "#ff0000", "#00ff00")

	// when
	err := s.storage.DeleteCollection(s.ctx(), userID, collectionID)

	// then
	s.Require().NoError(err)

	n, err := s.storage.CountColors(s.ctx(), collectionID)
	s.Require().NoError(err)
	s.Require().Zero(n)

	_, err = s.storage.CollectionByID(s.ctx(), userID, collectionID)
	s.Require().ErrorIs(err, storage.ErrNotFound)
}

func (s *StorageSuite) TestDeleteCollectionAnotherUserOwns() {
	olyaID, _ := s.createUserAndCollection("olya", "Olya")
	graceID, graceCollection := s.createUserAndCollection("grace", "Grace")

	// when
	err := s.storage.DeleteCollection(s.ctx(), olyaID, graceCollection)

	// then
	s.Require().ErrorIs(err, storage.ErrNotFound)
	s.Require().Equal(1, s.countCollections(graceID))
}

func (s *StorageSuite) TestDeleteCollectionUnknownCollection() {
	userID := s.createUser("olya", "Olya")

	err := s.storage.DeleteCollection(s.ctx(), userID, unknownCollection)

	s.Require().ErrorIs(err, storage.ErrNotFound)
}

// Utils

func (s *StorageSuite) countCollections(userID user.ID) int {
	var n int
	s.Require().NoError(s.pool.QueryRow(s.ctx(),
		`SELECT count(*) FROM collections WHERE user_id = $1`, userID).Scan(&n))
	return n
}

func idsOf(collections []collection.Collection) []collection.ID {
	ids := make([]collection.ID, 0, len(collections))
	for _, c := range collections {
		ids = append(ids, c.ID)
	}
	return ids
}
