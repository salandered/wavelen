//go:build integration

package storage_test

import (
	"time"

	"github.com/salandered/wavelen/internal/auth"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
)

func (s *StorageSuite) TestUserByEmailReturnsTheStoredHash() {
	s.createUser("olya@example.com", "Olya")

	// when
	u, err := s.storage.UserByEmail(s.ctx(), "olya@example.com")

	// then
	s.Require().NoError(err)
	s.Require().Equal("Olya", u.Name)
	s.Require().Equal(stubPasswordHash, u.PasswordHash)
}

func (s *StorageSuite) TestUserByEmailMatchesRegardlessOfCase() {
	// the column is citext, the lookup does not lowercase
	s.createUser("olya@example.com", "Olya")

	u, err := s.storage.UserByEmail(s.ctx(), "OLYA@EXAMPLE.COM")

	s.Require().NoError(err)
	s.Require().Equal("Olya", u.Name)
}

func (s *StorageSuite) TestUserByEmailReportsAnUnknownAddress() {
	_, err := s.storage.UserByEmail(s.ctx(), "nobody@example.com")

	s.Require().ErrorIs(err, storage.ErrUserNotFound)
}

func (s *StorageSuite) TestAnInsertedTokenResolvesToItsOwner() {
	id := s.createUser("olya@example.com", "Olya")
	tok := auth.NewToken(id, time.Hour)
	s.Require().NoError(s.storage.InsertToken(s.ctx(), tok))

	// when
	got, err := s.storage.UserIDForTokenHash(s.ctx(), tok.Hash)

	// then
	s.Require().NoError(err)
	s.Require().Equal(id, got)
}

func (s *StorageSuite) TestAnExpiredTokenIsIndistinguishableFromAnUnknownOne() {
	id := s.createUser("olya@example.com", "Olya")
	expired := auth.NewToken(id, -time.Minute)
	s.Require().NoError(s.storage.InsertToken(s.ctx(), expired))

	// when
	_, expiredErr := s.storage.UserIDForTokenHash(s.ctx(), expired.Hash)
	_, unknownErr := s.storage.UserIDForTokenHash(s.ctx(), auth.HashToken("never minted"))

	// then
	s.Require().ErrorIs(expiredErr, storage.ErrTokenNotFound)
	s.Require().ErrorIs(unknownErr, storage.ErrTokenNotFound)
}

func (s *StorageSuite) TestATokenForAnUnknownUserIsRejected() {
	tok := auth.NewToken(user.ID(999999), time.Hour)

	err := s.storage.InsertToken(s.ctx(), tok)

	s.Require().ErrorIs(err, storage.ErrUserNotFound)
}
