//go:build integration

package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/salandered/wavelen/internal/collection"
	"github.com/salandered/wavelen/internal/color"
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

// Tx tests

func (s *StorageSuite) TestInTxCommitsWhenCallbackReturnsNil() {
	_, collectionID := s.createUserAndCollection("olya")

	// when
	err := s.storage.InTx(s.ctx(), func(tx storage.Storage) error {
		_, err := tx.AddColor(s.ctx(), collectionID, "#ff0000")
		return err
	})

	// then
	s.Require().NoError(err)
	n, err := s.storage.CountColors(s.ctx(), collectionID)
	s.Require().NoError(err)
	s.Require().Equal(1, n)
}

func (s *StorageSuite) TestInTxRollsbackAllWritesWhenCallbackFails() {
	_, collectionID := s.createUserAndCollection("olya")
	sentinel := errors.New("callback gave up")

	// when
	err := s.storage.InTx(s.ctx(), func(tx storage.Storage) error {
		if _, err := tx.AddColor(s.ctx(), collectionID, "#ff0000"); err != nil {
			return err
		}
		if _, err := tx.AddColor(s.ctx(), collectionID, "#00ff00"); err != nil {
			return err
		}
		return sentinel
	})

	// then
	s.Require().ErrorIs(err, sentinel)
	n, err := s.storage.CountColors(s.ctx(), collectionID)
	s.Require().NoError(err)
	s.Require().Zero(n)
}

func (s *StorageSuite) TestInTxRefusesNesting() {
	err := s.storage.InTx(s.ctx(), func(tx storage.Storage) error {
		return tx.InTx(s.ctx(), func(storage.Storage) error { return nil })
	})

	s.Require().ErrorIs(err, storage.ErrNestedTx)
}

// utils

func (s *StorageSuite) ctx() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	s.T().Cleanup(cancel)
	return ctx
}

// Not a valid bcrypt hash, nothing here verifies it.
var stubPasswordHash = []byte("stub")

const testCollectionName = "Main"

func newCollection(name string, isDefault bool) collection.CreateParams {
	return collection.CreateParams{
		Name:      name,
		Icon:      collection.DefIconSlug,
		Accent:    collection.DefIconAccent,
		IsDefault: isDefault,
	}
}

func (s *StorageSuite) createUser(nickname string) user.ID {
	userID, _ := s.createUserAndCollection(nickname)
	return userID
}

// An account and its def collection
func (s *StorageSuite) createUserAndCollection(nickname string) (user.ID, collection.ID) {
	u := user.User{Nickname: nickname, PasswordHash: stubPasswordHash}
	s.Require().NoError(s.storage.CreateUser(s.ctx(), &u))

	col, err := s.storage.CreateCollection(
		s.ctx(),
		u.ID,
		newCollection(testCollectionName, true),
	)
	s.Require().NoError(err)
	return u.ID, col.ID
}

func (s *StorageSuite) addColors(collectionID collection.ID, hexes ...color.Hex) {
	for _, hex := range hexes {
		// one statement per row, so now() differs and no two rows share a created_at
		_, err := s.storage.AddColor(s.ctx(), collectionID, hex)
		s.Require().NoError(err)
	}
}

func (s *StorageSuite) countUsers(userID user.ID) int {
	return s.count(`SELECT count(*) FROM users WHERE id = $1`, userID)
}

func (s *StorageSuite) countCollections(userID user.ID) int {
	return s.count(`SELECT count(*) FROM collections WHERE user_id = $1`, userID)
}

func (s *StorageSuite) countColorsIn(collectionID collection.ID) int {
	return s.count(`SELECT count(*) FROM collection_colors WHERE collection_id = $1`, collectionID)
}

func (s *StorageSuite) countTokens(userID user.ID) int {
	return s.count(`SELECT count(*) FROM tokens WHERE user_id = $1`, userID)
}

func (s *StorageSuite) count(query string, arg any) int {
	var n int
	s.Require().NoError(s.pool.QueryRow(s.ctx(), query, arg).Scan(&n))
	return n
}
