//go:build integration

package colorsvc_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/salandered/wavelen/internal/collection"
	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/colorsvc"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/storagetest"
	"github.com/salandered/wavelen/internal/user"
	"github.com/stretchr/testify/suite"
)

var unknownCollection = collection.ID(
	uuid.MustParse("00000000-0000-7000-8000-00000000dead"))

func TestQuotaSuite(t *testing.T) {
	suite.Run(t, new(QuotaSuite))
}

type QuotaSuite struct {
	suite.Suite
	pool  *pgxpool.Pool
	store *storage.Postgres
}

func (s *QuotaSuite) SetupSuite() {
	s.pool = storagetest.Start(s.T())
	s.store = storage.New(s.pool)
}

func (s *QuotaSuite) SetupTest() {
	storagetest.Truncate(s.T(), s.pool)
}

func (s *QuotaSuite) TestConcurrentAddColorsRespectsQuota() {
	const (
		quota    = 20
		attempts = 80
	)
	ctx := s.ctx(30 * time.Second)
	userID, collectionID := s.createUser()
	colorsvc_ := colorsvc.New(s.store, quota)

	type outcome struct {
		created bool
		err     error
	}

	outcomes := make([]outcome, attempts)

	var wg sync.WaitGroup
	for i := range attempts {
		wg.Go(func() {
			created, err := colorsvc_.AddColor(ctx, userID, collectionID, color.Hex(fmt.Sprintf("#%06x", i)))
			outcomes[i] = outcome{created: created, err: err}
		})
	}
	wg.Wait()

	var created, full int
	for i, out := range outcomes {
		switch {
		case out.err == nil && out.created:
			created++
		case errors.Is(out.err, colorsvc.ErrQuotaFull):
			full++
		default:
			s.Require().Failf("unexpected outcome",
				"attempt %d: created=%v, err=%v", i, out.created, out.err)
		}
	}
	s.Require().Equal(quota, created)
	s.Require().Equal(attempts-quota, full)

	n, err := s.store.CountColors(ctx, collectionID)
	s.Require().NoError(err)
	s.Require().Equal(quota, n)
}

func (s *QuotaSuite) TestResavingColorAtQuotaStaysIdempotent() {
	const quota = 3
	ctx := s.ctx(10 * time.Second)
	userID, collectionID := s.createUser()
	colorsvc_ := colorsvc.New(s.store, quota)

	// add three colors
	for _, hex := range []color.Hex{"#000001", "#000002", "#000003"} {
		created, err := colorsvc_.AddColor(ctx, userID, collectionID, hex)
		s.Require().NoError(err)
		s.Require().True(created)
	}

	// a new color is refused
	_, err := colorsvc_.AddColor(ctx, userID, collectionID, "#000004")
	s.Require().ErrorIs(err, colorsvc.ErrQuotaFull)

	// adding new which is already saved is ok
	created, err := colorsvc_.AddColor(ctx, userID, collectionID, "#000001")
	s.Require().NoError(err)
	s.Require().False(created)

	n, err := s.store.CountColors(ctx, collectionID)
	s.Require().NoError(err)
	s.Require().Equal(quota, n)
}

func (s *QuotaSuite) TestAddColorToACollectionNobodyOwns() {
	ctx := s.ctx(10 * time.Second)

	created, err := colorsvc.New(s.store, 50).
		AddColor(ctx, 999, unknownCollection, "#ff0000")

	s.Require().ErrorIs(err, storage.ErrNotFound)
	s.Require().False(created)
}

func (s *QuotaSuite) ctx(d time.Duration) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	s.T().Cleanup(cancel)
	return ctx
}

// same what usersvc.CreateUser writes
func (s *QuotaSuite) createUser() (user.ID, collection.ID) {
	ctx := s.ctx(10 * time.Second)

	u := user.User{Nickname: "olya", Name: "Olya", PasswordHash: []byte("stub")}
	s.Require().NoError(s.store.CreateUser(ctx, &u))

	col, err := s.store.CreateCollection(ctx, u.ID, "My colors", true)
	s.Require().NoError(err)
	return u.ID, col.ID
}
