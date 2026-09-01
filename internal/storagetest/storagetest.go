//go:build integration

package storagetest

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"

	// registers the driver for the dbconfig.MigrateScheme URL scheme
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/salandered/wavelen"
	"github.com/salandered/wavelen/internal/dbconfig"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// throwaway container info
const (
	dbImage = "postgres:18-alpine"
	dbName  = "wavelen"
	dbUser  = "testuser"
	dbPass  = "testpass"
)

// same as in cmd/migrate
const sourceName = "iofs"

const opTimeout = 10 * time.Second

// Starts a Postgres container and applies migrations
func Start(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := runContainer(t)

	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	require.NoError(t, pool.Ping(ctx))

	applyMigrations(t, dsn)
	return pool
}

// Truncate deletes what the test suites write (except for the seeded palette).
// RESTART IDENTITY resets things like users.id bigserial.
func Truncate(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	// CASCADE would reach tokens anyway
	_, err := pool.Exec(ctx, `TRUNCATE users, user_colors, tokens RESTART IDENTITY CASCADE`)
	require.NoError(t, err)
}

// Starts a throwaway Postgres on a random host port
func runContainer(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx, dbImage,
		tcpostgres.WithDatabase(dbName),
		tcpostgres.WithUsername(dbUser),
		tcpostgres.WithPassword(dbPass),
		tcpostgres.BasicWaitStrategies(),
	)
	testcontainers.CleanupContainer(t, ctr) // registered before the error check
	require.NoError(t, err)

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	return dsn
}

// Builds the schema the same way as deploy (the embedded SQL via golang-migrate)
func applyMigrations(t *testing.T, dsn string) {
	t.Helper()

	src, err := iofs.New(wavelen.MigrationsFS, "migrations")
	require.NoError(t, err)

	// the container returns a "postgres://" dsn, replace with a scheme golang-migrate needs
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	u.Scheme = dbconfig.MigrateScheme

	m, err := migrate.NewWithSourceInstance(sourceName, src, u.String())
	require.NoError(t, err)
	defer func() {
		srcErr, dbErr := m.Close()
		require.NoError(t, srcErr)
		require.NoError(t, dbErr)
	}()

	require.NoError(t, m.Up())
}
