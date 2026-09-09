//go:build integration

package collectionsvc_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/salandered/wavelen/internal/collection"
	"github.com/salandered/wavelen/internal/collectionsvc"
	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/storagetest"
	"github.com/salandered/wavelen/internal/user"
	"github.com/salandered/wavelen/internal/usersvc"
	"github.com/stretchr/testify/suite"
)

var unknownCollection = collection.ID(
	uuid.MustParse("00000000-0000-7000-8000-00000000dead"))

func TestCollectionSuite(t *testing.T) {
	suite.Run(t, new(CollectionSuite))
}

type CollectionSuite struct {
	suite.Suite
	pool  *pgxpool.Pool
	store *storage.Postgres
}

func (s *CollectionSuite) SetupSuite() {
	s.pool = storagetest.Start(s.T())
	s.store = storage.New(s.pool)
}

func (s *CollectionSuite) SetupTest() {
	storagetest.Truncate(s.T(), s.pool)
}

// Create

func (s *CollectionSuite) TestCreateCollectionIsNotDefault() {
	userID := s.createUser("olya")

	col, err := collectionsvc.New(s.store, 10).CreateCollection(s.ctx(), userID, "Sunset")

	s.Require().NoError(err)
	s.Require().False(col.IsDefault)
	s.Require().Equal("Sunset", col.Name)
}

func (s *CollectionSuite) TestCreateCollectionAllowsRepeatedName() {
	userID := s.createUser("olya")
	svc := collectionsvc.New(s.store, 10)

	first, err := svc.CreateCollection(s.ctx(), userID, "Sunset")
	s.Require().NoError(err)
	second, err := svc.CreateCollection(s.ctx(), userID, "Sunset")

	s.Require().NoError(err)
	s.Require().NotEqual(first.ID, second.ID)
}

func (s *CollectionSuite) TestCreateCollectionUnknownUser() {
	_, err := collectionsvc.New(s.store, 10).CreateCollection(s.ctx(), 999, "Sunset")

	s.Require().ErrorIs(err, storage.ErrUserNotFound)
}

// Quota

func (s *CollectionSuite) TestCreateCollectionExceedQuota() {
	const quota = 2
	// adds the default collection
	userID := s.createUser("olya")
	svc := collectionsvc.New(s.store, quota)

	// second
	_, err := svc.CreateCollection(s.ctx(), userID, "Sunset")
	s.Require().NoError(err)

	// third
	_, err = svc.CreateCollection(s.ctx(), userID, "Ocean")

	s.Require().ErrorIs(err, collectionsvc.ErrQuotaFull)
	s.Require().Equal(quota, s.countCollections(userID))
}

func (s *CollectionSuite) TestConcurrentCreatesRespectQuota() {
	const (
		quota    = 5
		attempts = 40
	)
	ctx := s.ctxFor(30 * time.Second)
	userID := s.createUser("olya")
	svc := collectionsvc.New(s.store, quota)

	type outcome struct {
		created bool
		err     error
	}
	outcomes := make([]outcome, attempts)

	// create collections
	var wg sync.WaitGroup
	for i := range attempts {
		wg.Go(func() {
			_, err := svc.CreateCollection(ctx, userID, fmt.Sprintf("Palette %d", i))
			outcomes[i] = outcome{created: err == nil, err: err}
		})
	}
	wg.Wait()

	// count outcome results
	var created, full int
	for i, out := range outcomes {
		switch {
		case out.err == nil && out.created:
			created++
		case errors.Is(out.err, collectionsvc.ErrQuotaFull):
			full++
		default:
			s.Require().Failf("unexpected outcome", "attempt %d: err=%v", i, out.err)
		}
	}

	// default coll takes one slot
	s.Require().Equal(quota-1, created)
	s.Require().Equal(attempts-(quota-1), full)
	s.Require().Equal(quota, s.countCollections(userID))
}

// Read

func (s *CollectionSuite) TestCollectionByIDAnotherUserOwns() {
	olyaID := s.createUser("olya")
	graceID := s.createUser("grace")
	svc := collectionsvc.New(s.store, 10)

	graceCollection, err := svc.CreateCollection(s.ctx(), graceID, "Sunset")
	s.Require().NoError(err)

	// when
	_, err = svc.CollectionByID(s.ctx(), olyaID, graceCollection.ID)

	// then
	s.Require().ErrorIs(err, storage.ErrNotFound)
}

func (s *CollectionSuite) TestListCollectionsReturnsOnlyTheCallers() {
	olyaID := s.createUser("olya")
	graceID := s.createUser("grace")
	svc := collectionsvc.New(s.store, 10)

	_, err := svc.CreateCollection(s.ctx(), graceID, "Grace only")
	s.Require().NoError(err)

	got, err := svc.ListCollections(s.ctx(), olyaID)

	s.Require().NoError(err)
	s.Require().Len(got, 1) // just the default
	s.Require().True(got[0].IsDefault)
}

// Delete

func (s *CollectionSuite) TestDeleteDefaultCollectionIsRefused() {
	userID := s.createUser("olya")
	svc := collectionsvc.New(s.store, 10)

	defaultID := s.defaultCollection(userID)

	// when
	err := svc.DeleteCollection(s.ctx(), userID, defaultID)

	// then
	s.Require().ErrorIs(err, collectionsvc.ErrDeleteDefault)
	s.Require().Equal(1, s.countCollections(userID))
}

func (s *CollectionSuite) TestDeleteCollectionRemovesItAndItsColors() {
	userID := s.createUser("olya")
	svc := collectionsvc.New(s.store, 10)

	col, err := svc.CreateCollection(s.ctx(), userID, "Sunset")
	s.Require().NoError(err)
	_, err = s.store.AddColor(s.ctx(), col.ID, color.Hex("#ff0000"))
	s.Require().NoError(err)

	// when
	err = svc.DeleteCollection(s.ctx(), userID, col.ID)

	// then
	s.Require().NoError(err)
	s.Require().Equal(1, s.countCollections(userID)) // the default survives

	n, err := s.store.CountColors(s.ctx(), col.ID)
	s.Require().NoError(err)
	s.Require().Zero(n)
}

func (s *CollectionSuite) TestDeleteCollectionAnotherUserOwnsShouldReturnNotFound() {
	olyaID := s.createUser("olya")
	graceID := s.createUser("grace")
	svc := collectionsvc.New(s.store, 10)

	graceDefault := s.defaultCollection(graceID)

	// when
	err := svc.DeleteCollection(s.ctx(), olyaID, graceDefault)

	// then
	s.Require().ErrorIs(err, storage.ErrNotFound)
	s.Require().NotErrorIs(err, collectionsvc.ErrDeleteDefault)
	s.Require().Equal(1, s.countCollections(graceID))
}

func (s *CollectionSuite) TestDeleteCollectionUnknownCollection() {
	userID := s.createUser("olya")

	err := collectionsvc.New(s.store, 10).
		DeleteCollection(s.ctx(), userID, unknownCollection)

	s.Require().ErrorIs(err, storage.ErrNotFound)
}

// utils

func (s *CollectionSuite) ctx() context.Context {
	return s.ctxFor(10 * time.Second)
}

func (s *CollectionSuite) ctxFor(d time.Duration) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	s.T().Cleanup(cancel)
	return ctx
}

// Create an account with the default collection.
func (s *CollectionSuite) createUser(nickname string) user.ID {
	u := user.User{Nickname: nickname, Name: nickname, PasswordHash: []byte("stub")}
	s.Require().NoError(usersvc.New(s.store).CreateUser(s.ctx(), &u))
	return u.ID
}

func (s *CollectionSuite) defaultCollection(userID user.ID) collection.ID {
	var id collection.ID
	s.Require().NoError(s.pool.QueryRow(s.ctx(),
		`SELECT id FROM collections WHERE user_id = $1 AND is_default`, userID).Scan(&id))
	return id
}

func (s *CollectionSuite) countCollections(userID user.ID) int {
	var n int
	s.Require().NoError(s.pool.QueryRow(s.ctx(),
		`SELECT count(*) FROM collections WHERE user_id = $1`, userID).Scan(&n))
	return n
}
