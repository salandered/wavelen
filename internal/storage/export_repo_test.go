//go:build integration

package storage_test

import (
	"github.com/salandered/wavelen/internal/collection"
	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
)

func (s *StorageSuite) TestExportCollectionsForUnknownUserIsEmptyNoError() {
	got, err := s.storage.ExportCollections(s.ctx(), 999)

	s.Require().NoError(err)
	s.Require().Empty(got)
}

func (s *StorageSuite) TestExportCollectionsReturnsOneCollectionWithOneColor() {
	userID, defaultID := s.createUserAndCollection("olya", "Olya")
	s.addColors(defaultID, "#ff0000")

	got, err := s.storage.ExportCollections(s.ctx(), userID)

	s.Require().NoError(err)
	s.Require().Len(got, 1)
	s.Require().Equal(defaultID, got[0].Clt.ID)
	s.Require().Equal([]color.Hex{"#ff0000"}, hexesOf(got[0].Colors))
}

// unreachable in prod: every user has at least one def collection
func (s *StorageSuite) TestExportCollectionsForUserWithoutCollectionsIsEmptyNoError() {
	u := user.User{Nickname: "olya", Name: "Olya", PasswordHash: stubPasswordHash}
	s.Require().NoError(s.storage.CreateUser(s.ctx(), &u))

	got, err := s.storage.ExportCollections(s.ctx(), u.ID)

	s.Require().NoError(err)
	s.Require().NotNil(got)
	s.Require().Empty(got)
}

func (s *StorageSuite) TestExportCollectionsOrdersCollectionsOldestFirst() {
	userID, defaultID := s.createUserAndCollection("olya", "Olya")
	sunset, err := s.storage.CreateCollection(s.ctx(), userID, newCollection("Sunset", false))
	s.Require().NoError(err)
	ocean, err := s.storage.CreateCollection(s.ctx(), userID, newCollection("Ocean", false))
	s.Require().NoError(err)

	// when
	got, err := s.storage.ExportCollections(s.ctx(), userID)

	// then
	s.Require().NoError(err)
	s.Require().Equal([]collection.ID{defaultID, sunset.ID, ocean.ID}, cltIDsOf(got))
}

func (s *StorageSuite) TestExportCollectionsGroupsColorsByCollectionOldestFirst() {
	userID, defaultID := s.createUserAndCollection("olya", "Olya")
	sunset, err := s.storage.CreateCollection(s.ctx(), userID, newCollection("Sunset", false))
	s.Require().NoError(err)
	s.addColors(defaultID, "#ff0000", "#00ff00")
	s.addColors(sunset.ID, "#0000ff")

	// when
	got, err := s.storage.ExportCollections(s.ctx(), userID)

	// then
	s.Require().NoError(err)
	s.Require().Len(got, 2)

	s.Require().Equal(defaultID, got[0].Clt.ID)
	s.Require().Equal(testCollectionName, got[0].Clt.Name)
	s.Require().True(got[0].Clt.IsDefault)
	s.Require().Equal([]color.Hex{"#ff0000", "#00ff00"}, hexesOf(got[0].Colors))

	s.Require().Equal(sunset.ID, got[1].Clt.ID)
	s.Require().Equal([]color.Hex{"#0000ff"}, hexesOf(got[1].Colors))
}

func (s *StorageSuite) TestExportCollectionsKeepsCollectionWithNoColors() {
	userID, defaultID := s.createUserAndCollection("olya", "Olya")
	_, err := s.storage.CreateCollection(s.ctx(), userID, newCollection("Empty", false))
	s.Require().NoError(err)
	s.addColors(defaultID, "#ff0000")

	got, err := s.storage.ExportCollections(s.ctx(), userID)

	s.Require().NoError(err)
	s.Require().Len(got, 2)
	s.Require().Equal("Empty", got[1].Clt.Name)
	s.Require().NotNil(got[1].Colors) // empty list
	s.Require().Empty(got[1].Colors)
}

func (s *StorageSuite) TestExportCollectionsSkipsAnotherUserRows() {
	olyaID, olyaCollection := s.createUserAndCollection("olya", "Olya")
	_, graceCollection := s.createUserAndCollection("grace", "Grace")
	s.addColors(olyaCollection, "#ff0000")
	s.addColors(graceCollection, "#00ff00")

	got, err := s.storage.ExportCollections(s.ctx(), olyaID)

	s.Require().NoError(err)
	s.Require().Len(got, 1)
	s.Require().Equal(olyaCollection, got[0].Clt.ID)
	s.Require().Equal([]color.Hex{"#ff0000"}, hexesOf(got[0].Colors))
}

// Utils

func cltIDsOf(items []storage.CltWithColors) []collection.ID {
	ids := make([]collection.ID, 0, len(items))
	for _, v := range items {
		ids = append(ids, v.Clt.ID)
	}
	return ids
}
