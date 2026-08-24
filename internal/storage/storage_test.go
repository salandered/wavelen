//go:build integration

package storage_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// throwaway container info
const (
	testDBImage = "postgres:18-alpine"
	testDBName  = "wavelen"
	testDBUser  = "testuser"
	testDBPass  = "testpass"
)

func TestStorageSuite(t *testing.T) {
	suite.Run(t, new(StorageSuite))
}

type StorageSuite struct {
	suite.Suite
	pool    *pgxpool.Pool
	storage *storage.Store
}

func (s *StorageSuite) SetupSuite() {
	pool, err := pgxpool.New(s.ctx(), s.runContainer())
	s.Require().NoError(err)
	s.Require().NoError(pool.Ping(s.ctx()))

	s.pool = pool
	s.storage = storage.New(pool)
	s.applyMigrations()
}

// Starts a throwaway Postgres on a random host port and returns its DSN.
func (s *StorageSuite) runContainer() string {
	ctx := context.Background() // outlives s.ctx(), the container is torn down by Cleanup

	ctr, err := tcpostgres.Run(ctx, testDBImage,
		tcpostgres.WithDatabase(testDBName),
		tcpostgres.WithUsername(testDBUser),
		tcpostgres.WithPassword(testDBPass),
		tcpostgres.BasicWaitStrategies(),
	)
	testcontainers.CleanupContainer(s.T(), ctr) // registered before the error check
	s.Require().NoError(err)

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	s.Require().NoError(err)
	return dsn
}

func (s *StorageSuite) TearDownSuite() {
	s.pool.Close()
}

func (s *StorageSuite) SetupTest() {
	// truncate all except for common_colors.
	// RESTART IDENTITY - resets things like users.id bigserial
	_, err := s.pool.Exec(s.ctx(), `TRUNCATE users, user_colors RESTART IDENTITY CASCADE`)
	s.Require().NoError(err)
}

// Rebuilds the schema from migrations/*.up.sql.
func (s *StorageSuite) applyMigrations() {
	ctx := s.ctx()

	_, err := s.pool.Exec(ctx,
		`DROP TABLE IF EXISTS user_colors, common_colors, users, schema_migrations CASCADE`)
	s.Require().NoError(err)

	files, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.up.sql"))
	s.Require().NoError(err)
	s.Require().NotEmpty(files)
	slices.Sort(files) // version order is the apply order

	for _, f := range files {
		raw, err := os.ReadFile(f)
		s.Require().NoError(err, f)

		// no args, so pgx takes the simple protocol and a file may hold several statements
		_, err = s.pool.Exec(ctx, string(raw))
		s.Require().NoError(err, f)
	}
}

func (s *StorageSuite) ctx() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	s.T().Cleanup(cancel)
	return ctx
}

// utils to mock db data

func (s *StorageSuite) createUser(email, name string) user.ID {
	u := user.User{Email: email, Name: name}
	s.Require().NoError(s.storage.CreateUser(s.ctx(), &u))
	return u.ID
}
