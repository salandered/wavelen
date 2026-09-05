//go:build integration

package storage_test

import (
	"time"

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
