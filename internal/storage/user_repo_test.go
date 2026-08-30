//go:build integration

package storage_test

import (
	"time"

	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
)

func (s *StorageSuite) TestCreateUserFillsInIDAndCreatedAt() {
	u := user.User{Email: "olya@example.com", Name: "Olya Lovelace"}

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
	u := user.User{Email: "olya@example.com", Name: "Olya Again"}
	err := s.storage.CreateUser(s.ctx(), &u)

	// then
	s.Require().ErrorIs(err, storage.ErrDuplicateEmail)
}

func (s *StorageSuite) TestCreateUserEmailUniquenessIgnoresCase() {
	s.createUser("olya@example.com", "Olya")

	// when
	u := user.User{Email: "OLYA@EXAMPLE.COM", Name: "Olya Again"}
	err := s.storage.CreateUser(s.ctx(), &u)

	// then
	// (the email column is citext)
	s.Require().ErrorIs(err, storage.ErrDuplicateEmail)
}
