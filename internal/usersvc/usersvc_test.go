//go:build integration

package usersvc_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/salandered/wavelen/internal/collection"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/storagetest"
	"github.com/salandered/wavelen/internal/user"
	"github.com/salandered/wavelen/internal/usersvc"
	"github.com/stretchr/testify/suite"
)

func TestSignupSuite(t *testing.T) {
	suite.Run(t, new(SignupSuite))
}

type SignupSuite struct {
	suite.Suite
	pool  *pgxpool.Pool
	store *storage.Postgres
}

func (s *SignupSuite) SetupSuite() {
	s.pool = storagetest.Start(s.T())
	s.store = storage.New(s.pool)
}

func (s *SignupSuite) SetupTest() {
	storagetest.Truncate(s.T(), s.pool)
}

func (s *SignupSuite) TestCreateUserAlsoCreatesOneDefaultCollection() {
	ctx := s.ctx()

	u := user.User{Nickname: "olya", PasswordHash: []byte("stub")}

	// when
	s.Require().NoError(usersvc.New(s.store).CreateUser(ctx, &u))

	// then
	s.Require().NotZero(u.ID)
	s.Require().Equal(1, s.countUsers(ctx))
	s.Require().Equal(1, s.countCollections(ctx))

	collectionID, name := s.defaultCollection(ctx, u.ID)
	s.Require().NotZero(collectionID)
	s.Require().Equal(usersvc.DefCollectionName, name)
}

func (s *SignupSuite) TestTakenNicknameThenNoNewUserNoDefCollection() {
	ctx := s.ctx()
	svc := usersvc.New(s.store)

	first := user.User{Nickname: "olya", PasswordHash: []byte("stub")}
	s.Require().NoError(svc.CreateUser(ctx, &first))

	// when
	second := user.User{Nickname: "olya", PasswordHash: []byte("stub")}
	err := svc.CreateUser(ctx, &second)

	// then
	s.Require().ErrorIs(err, storage.ErrDuplicateNickname)
	s.Require().Equal(1, s.countUsers(ctx))
	s.Require().Equal(1, s.countCollections(ctx))
}

func (s *SignupSuite) TestEveryAccountGetsItsOwnCollection() {
	ctx := s.ctx()
	svc := usersvc.New(s.store)

	olya := user.User{Nickname: "olya", PasswordHash: []byte("stub")}
	s.Require().NoError(svc.CreateUser(ctx, &olya))
	grace := user.User{Nickname: "grace", PasswordHash: []byte("stub")}
	s.Require().NoError(svc.CreateUser(ctx, &grace))

	olyaCollection, _ := s.defaultCollection(ctx, olya.ID)
	graceCollection, _ := s.defaultCollection(ctx, grace.ID)

	s.Require().NotEqual(olyaCollection, graceCollection)
}

// utils

func (s *SignupSuite) ctx() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	s.T().Cleanup(cancel)
	return ctx
}

// Reads the default collection
func (s *SignupSuite) defaultCollection(
	ctx context.Context, userID user.ID,
) (collection.ID, string) {
	var id collection.ID
	var name string

	s.Require().NoError(s.pool.QueryRow(ctx,
		`SELECT id, name FROM collections WHERE user_id = $1 AND is_default`, userID,
	).Scan(&id, &name))
	return id, name
}

func (s *SignupSuite) countUsers(ctx context.Context) int {
	return s.count(ctx, "SELECT count(*) FROM users")
}

func (s *SignupSuite) countCollections(ctx context.Context) int {
	return s.count(ctx, "SELECT count(*) FROM collections")
}

func (s *SignupSuite) count(ctx context.Context, query string) int {
	var n int
	s.Require().NoError(s.pool.QueryRow(ctx, query).Scan(&n))
	return n
}
