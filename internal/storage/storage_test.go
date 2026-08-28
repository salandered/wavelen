//go:build integration

package storage_test

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"

	// registers the driver for the migrateScheme URL scheme
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/salandered/wavelen"
	"github.com/salandered/wavelen/internal/dbconfig"
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

// same as in cmd/migrate
const sourceName = "iofs"

func TestStorageSuite(t *testing.T) {
	suite.Run(t, new(StorageSuite))
}

type StorageSuite struct {
	suite.Suite
	pool    *pgxpool.Pool
	storage *storage.Store
}

func (s *StorageSuite) SetupSuite() {
	dsn := s.runContainer()

	pool, err := pgxpool.New(s.ctx(), dsn)
	s.Require().NoError(err)
	s.Require().NoError(pool.Ping(s.ctx()))

	s.pool = pool
	s.storage = storage.New(pool)
	s.applyMigrations(dsn)
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
	// TODO: kinda fragile
	_, err := s.pool.Exec(s.ctx(), `TRUNCATE users, user_colors RESTART IDENTITY CASCADE`)
	s.Require().NoError(err)
}

// Builds the schema the same way as deploy (the embedded SQL via golang-migrate)
func (s *StorageSuite) applyMigrations(dsn string) {
	src, err := iofs.New(wavelen.MigrationsFS, "migrations")
	s.Require().NoError(err)

	// the container hands out a "postgres://" dsn, golang-migrate picks its driver by scheme
	u, err := url.Parse(dsn)
	s.Require().NoError(err)
	u.Scheme = dbconfig.MigrateScheme

	m, err := migrate.NewWithSourceInstance(sourceName, src, u.String())
	s.Require().NoError(err)
	defer func() {
		srcErr, dbErr := m.Close()
		s.Require().NoError(srcErr)
		s.Require().NoError(dbErr)
	}()

	s.Require().NoError(m.Up())
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
