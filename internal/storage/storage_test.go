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
)

const dsnEnv = "WAVELEN_TEST_DB_DSN"

func TestStorageSuite(t *testing.T) {
	dsn := os.Getenv(dsnEnv)
	if dsn == "" {
		t.Skipf("%s is not set", dsnEnv)
	}
	suite.Run(t, &StorageSuite{dsn: dsn})
}

type StorageSuite struct {
	suite.Suite
	dsn     string
	pool    *pgxpool.Pool
	storage *storage.Store
}

func (s *StorageSuite) SetupSuite() {
	pool, err := pgxpool.New(s.ctx(), s.dsn)
	s.Require().NoError(err)
	s.Require().NoError(pool.Ping(s.ctx()))

	s.pool = pool
	s.storage = storage.New(pool)
	s.applyMigrations()
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
