//go:build integration

package storage_test

import (
	"time"

	"github.com/salandered/wavelen/internal/auth"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
)

func (s *StorageSuite) TestCreateUserFillsInIDAndCreatedAt() {
	u := user.User{Nickname: "olya", Name: "Olya Lovelace", PasswordHash: stubPasswordHash}

	// when
	err := s.storage.CreateUser(s.ctx(), &u)

	// then
	s.Require().NoError(err)
	s.Require().NotZero(u.ID)
	s.Require().WithinDuration(time.Now(), u.CreatedAt, time.Minute)
}

func (s *StorageSuite) TestCreateUserAssignsDistinctIDs() {
	olya := s.createUser("olya", "Olya")
	grace := s.createUser("grace", "Grace")

	s.Require().NotEqual(olya, grace)
}

func (s *StorageSuite) TestCreateUserRejectsTakenNickname() {
	s.createUser("olya", "Olya")

	// when
	u := user.User{Nickname: "olya", Name: "Olya Again", PasswordHash: stubPasswordHash}
	err := s.storage.CreateUser(s.ctx(), &u)

	// then
	s.Require().ErrorIs(err, storage.ErrDuplicateNickname)
}

func (s *StorageSuite) TestCreateUserNicknameUniquenessIgnoresCase() {
	s.createUser("olya", "Olya")

	// when
	u := user.User{Nickname: "OLYA", Name: "Olya Again", PasswordHash: stubPasswordHash}
	err := s.storage.CreateUser(s.ctx(), &u)

	// then
	// (the nickname column is citext)
	s.Require().ErrorIs(err, storage.ErrDuplicateNickname)
}

func (s *StorageSuite) TestUserByIDReturnsAccountWithoutPasswordHash() {
	id := s.createUser("olya", "Olya Lovelace")

	// when
	u, err := s.storage.UserByID(s.ctx(), id)

	// then
	s.Require().NoError(err)
	s.Require().Equal(id, u.ID)
	s.Require().Equal("olya", u.Nickname)
	s.Require().Equal("Olya Lovelace", u.Name)
	s.Require().WithinDuration(time.Now(), u.CreatedAt, time.Minute)
	// the query does not select the column
	s.Require().Empty(u.PasswordHash)
}

func (s *StorageSuite) TestUserByIDUnknownIDReturnsNotFound() {
	_, err := s.storage.UserByID(s.ctx(), 999999)

	s.Require().ErrorIs(err, storage.ErrUserNotFound)
}

// Proving the cascade schema rules
func (s *StorageSuite) TestDeleteUserDeletesCollectionsColorsTokens() {
	userID, defCollection := s.createUserAndCollection("olya", "Olya")

	second, err := s.storage.CreateCollection(s.ctx(), userID, newCollection("Work", false))
	s.Require().NoError(err)

	s.addColors(defCollection, "#112233", "#445566")
	s.addColors(second.ID, "#778899")

	for range 2 {
		s.Require().NoError(s.storage.InsertToken(s.ctx(), auth.NewToken(userID, time.Hour)))
	}

	// when
	s.Require().NoError(s.storage.DeleteUser(s.ctx(), userID))

	// then
	s.Require().Zero(s.countUsers(userID))
	s.Require().Zero(s.countCollections(userID))
	s.Require().Zero(s.countColorsIn(defCollection))
	s.Require().Zero(s.countColorsIn(second.ID))
	s.Require().Zero(s.countTokens(userID))
}

func (s *StorageSuite) TestDeleteUserLeavesAnotherAccount() {
	olya, olyaCollection := s.createUserAndCollection("olya", "Olya")
	grace, graceCollection := s.createUserAndCollection("grace", "Grace")

	s.addColors(olyaCollection, "#112233")
	s.addColors(graceCollection, "#445566")
	s.Require().NoError(s.storage.InsertToken(s.ctx(), auth.NewToken(grace, time.Hour)))

	// when
	s.Require().NoError(s.storage.DeleteUser(s.ctx(), olya))

	// then
	s.Require().Equal(1, s.countUsers(grace))
	s.Require().Equal(1, s.countCollections(grace))
	s.Require().Equal(1, s.countColorsIn(graceCollection))
	s.Require().Equal(1, s.countTokens(grace))
}

func (s *StorageSuite) TestDeleteUserReleasesTheNickname() {
	id := s.createUser("olya", "Olya")
	s.Require().NoError(s.storage.DeleteUser(s.ctx(), id))

	// when
	again := user.User{Nickname: "olya", Name: "Someone Else", PasswordHash: stubPasswordHash}
	err := s.storage.CreateUser(s.ctx(), &again)

	// then
	s.Require().NoError(err)
	s.Require().NotEqual(id, again.ID)
}

func (s *StorageSuite) TestDeleteUserUnknownIDReturnsNotFound() {
	err := s.storage.DeleteUser(s.ctx(), 999999)

	s.Require().ErrorIs(err, storage.ErrUserNotFound)
}
