//go:build integration

package storage_test

import (
	"time"

	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
)

func (s *StorageSuite) TestCreateUserFillsInIDAndCreatedAt() {
	u := user.User{Email: "olya@example.com", Name: "Olya Lovelace", PasswordHash: stubPasswordHash}

	// when
	err := s.storage.CreateUser(s.ctx(), &u)

	// then
	s.Require().NoError(err)
	s.Require().NotZero(u.ID)
	s.Require().WithinDuration(time.Now(), u.CreatedAt, time.Minute)
}

func (s *StorageSuite) TestCreateUserAssignsDistinctIDs() {
	olya := s.createUser("olya@example.com", "Olya")
	grace := s.createUser("grace@example.com", "Grace")

	s.Require().NotEqual(olya, grace)
}

func (s *StorageSuite) TestCreateUserRejectsATakenEmail() {
	s.createUser("olya@example.com", "Olya")

	// when
	u := user.User{Email: "olya@example.com", Name: "Olya Again", PasswordHash: stubPasswordHash}
	err := s.storage.CreateUser(s.ctx(), &u)

	// then
	s.Require().ErrorIs(err, storage.ErrDuplicateEmail)
}

func (s *StorageSuite) TestCreateUserEmailUniquenessIgnoresCase() {
	s.createUser("olya@example.com", "Olya")

	// when
	u := user.User{Email: "OLYA@EXAMPLE.COM", Name: "Olya Again", PasswordHash: stubPasswordHash}
	err := s.storage.CreateUser(s.ctx(), &u)

	// then
	// (the email column is citext)
	s.Require().ErrorIs(err, storage.ErrDuplicateEmail)
}

func (s *StorageSuite) TestUserByIDReturnsAccountWithoutPasswordHash() {
	id := s.createUser("olya@example.com", "Olya Lovelace")

	// when
	u, err := s.storage.UserByID(s.ctx(), id)

	// then
	s.Require().NoError(err)
	s.Require().Equal(id, u.ID)
	s.Require().Equal("olya@example.com", u.Email)
	s.Require().Equal("Olya Lovelace", u.Name)
	s.Require().WithinDuration(time.Now(), u.CreatedAt, time.Minute)
	// the query does not select the column
	s.Require().Empty(u.PasswordHash)
}

func (s *StorageSuite) TestUserByIDUnknownIDReturnsNotFound() {
	_, err := s.storage.UserByID(s.ctx(), 999999)

	s.Require().ErrorIs(err, storage.ErrUserNotFound)
}
