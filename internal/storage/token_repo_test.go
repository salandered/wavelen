//go:build integration

package storage_test

import (
	"time"

	"github.com/salandered/wavelen/internal/auth"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
)

func (s *StorageSuite) TestUserByNicknameReturnsTheStoredHash() {
	s.createUser("olya", "Olya")

	// when
	u, err := s.storage.UserByNickname(s.ctx(), "olya")

	// then
	s.Require().NoError(err)
	s.Require().Equal("Olya", u.Name)
	s.Require().Equal(stubPasswordHash, u.PasswordHash)
}

func (s *StorageSuite) TestUserByNicknameMatchesRegardlessOfCase() {
	// the column is citext, the lookup does not lowercase
	s.createUser("olya", "Olya")

	u, err := s.storage.UserByNickname(s.ctx(), "OLYA")

	s.Require().NoError(err)
	s.Require().Equal("Olya", u.Name)
}

func (s *StorageSuite) TestUserByNicknameReportsAnUnknownNickname() {
	_, err := s.storage.UserByNickname(s.ctx(), "nobody")

	s.Require().ErrorIs(err, storage.ErrUserNotFound)
}

func (s *StorageSuite) TestAnInsertedTokenResolvesToItsOwner() {
	id := s.createUser("olya", "Olya")
	tok := auth.NewToken(id, time.Hour)
	s.Require().NoError(s.storage.InsertToken(s.ctx(), tok))

	// when
	got, err := s.storage.UserIDForTokenHash(s.ctx(), tok.Hash)

	// then
	s.Require().NoError(err)
	s.Require().Equal(id, got)
}

func (s *StorageSuite) TestAnExpiredTokenIsIndistinguishableFromAnUnknownOne() {
	id := s.createUser("olya", "Olya")
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

func (s *StorageSuite) TestDeletedTokenStopsResolving() {
	id := s.createUser("olya", "Olya")
	kept := auth.NewToken(id, time.Hour)
	revoked := auth.NewToken(id, time.Hour)
	s.Require().NoError(s.storage.InsertToken(s.ctx(), kept))
	s.Require().NoError(s.storage.InsertToken(s.ctx(), revoked))

	// when
	s.Require().NoError(s.storage.DeleteToken(s.ctx(), revoked.Hash))

	// then
	_, err := s.storage.UserIDForTokenHash(s.ctx(), revoked.Hash)
	s.Require().ErrorIs(err, storage.ErrTokenNotFound)

	// the user's other sessions are ok
	got, err := s.storage.UserIDForTokenHash(s.ctx(), kept.Hash)
	s.Require().NoError(err)
	s.Require().Equal(id, got)
}

func (s *StorageSuite) TestDeletingAnUnknownTokenIsNotAnError() {
	err := s.storage.DeleteToken(s.ctx(), auth.HashToken("never minted"))

	s.Require().NoError(err)
}
