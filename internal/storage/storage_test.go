//go:build integration

package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/storagetest"
	"github.com/salandered/wavelen/internal/user"
	"github.com/stretchr/testify/suite"
)

func TestStorageSuite(t *testing.T) {
	suite.Run(t, new(StorageSuite))
}

type StorageSuite struct {
	suite.Suite
	pool    *pgxpool.Pool
	storage *storage.Postgres
}

func (s *StorageSuite) SetupSuite() {
	s.pool = storagetest.Start(s.T())
	s.storage = storage.New(s.pool)
}

func (s *StorageSuite) SetupTest() {
	storagetest.Truncate(s.T(), s.pool)
}

// utils

func (s *StorageSuite) ctx() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	s.T().Cleanup(cancel)
	return ctx
}

// utils to mock db data

// Not a valid bcrypt hash, nothing here verifies it.
var stubPasswordHash = []byte("stub")

func (s *StorageSuite) createUser(email, name string) user.ID {
	u := user.User{Email: email, Name: name, PasswordHash: stubPasswordHash}
	s.Require().NoError(s.storage.CreateUser(s.ctx(), &u))
	return u.ID
}

// Tx tests

func (s *StorageSuite) TestInTxCommitsWhenCallbackReturnsNil() {
	userID := s.createUser("olya@example.com", "Olya")

	// when
	err := s.storage.InTx(s.ctx(), func(tx storage.Storage) error {
		_, err := tx.AddColor(s.ctx(), userID, "#ff0000")
		return err
	})

	// then
	s.Require().NoError(err)
	n, err := s.storage.CountColors(s.ctx(), userID)
	s.Require().NoError(err)
	s.Require().Equal(1, n)
}

func (s *StorageSuite) TestInTxRollsbackAllWritesWhenCallbackFails() {
	userID := s.createUser("olya@example.com", "Olya")
	sentinel := errors.New("callback gave up")

	// when
	err := s.storage.InTx(s.ctx(), func(tx storage.Storage) error {
		if _, err := tx.AddColor(s.ctx(), userID, "#ff0000"); err != nil {
			return err
		}
		if _, err := tx.AddColor(s.ctx(), userID, "#00ff00"); err != nil {
			return err
		}
		return sentinel
	})

	// then
	s.Require().ErrorIs(err, sentinel)
	n, err := s.storage.CountColors(s.ctx(), userID)
	s.Require().NoError(err)
	s.Require().Zero(n)
}

func (s *StorageSuite) TestInTxRefusesNesting() {
	err := s.storage.InTx(s.ctx(), func(tx storage.Storage) error {
		return tx.InTx(s.ctx(), func(storage.Storage) error { return nil })
	})

	s.Require().ErrorIs(err, storage.ErrNestedTx)
}
