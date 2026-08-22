//go:build integration

package storage_test

import (
	"time"

	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
)

func (s *StorageSuite) TestCreateUserFillsInIDAndCreatedAt() {
	u := user.User{Email: "ada@example.com", Name: "Ada Lovelace"}

	// when
	err := s.storage.CreateUser(s.ctx(), &u)

	// then
	s.Require().NoError(err)
	s.Require().NotZero(u.ID)
	s.Require().WithinDuration(time.Now(), u.CreatedAt, time.Minute)
}

func (s *StorageSuite) TestCreateUserAssignsDistinctIDs() {
	ada := s.createUser("ada@example.com", "Ada")
	grace := s.createUser("grace@example.com", "Grace")

	s.Require().NotEqual(ada, grace)
}

func (s *StorageSuite) TestCreateUserRejectsATakenEmail() {
	s.createUser("ada@example.com", "Ada")

	// when
	u := user.User{Email: "ada@example.com", Name: "Ada Again"}
	err := s.storage.CreateUser(s.ctx(), &u)

	// then
	s.Require().ErrorIs(err, storage.ErrDuplicateEmail)
}

func (s *StorageSuite) TestCreateUserEmailUniquenessIgnoresCase() {
	s.createUser("ada@example.com", "Ada")

	// when
	u := user.User{Email: "ADA@EXAMPLE.COM", Name: "Ada Again"}
	err := s.storage.CreateUser(s.ctx(), &u)

	// then
	// (the email column is citext)
	s.Require().ErrorIs(err, storage.ErrDuplicateEmail)
}
