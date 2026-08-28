// Applies the embedded SQL migrations to the db using POSTGRES_* env vars.
// Rerunning is a no-op.
package main

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/golang-migrate/migrate/v4"

	// registers the driver for the migrateScheme URL scheme
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/salandered/wavelen"
	"github.com/salandered/wavelen/internal/dbconfig"
	"github.com/salandered/wavelen/internal/version"

	logging "github.com/salandered/slogenv"
)

// sourceName labels the source driver in golang-migrate errors
const sourceName = "iofs"

func main() {
	// same log setup as cmd/api/main.go, but no AttrsFunc
	logCloser, err := setupLogging()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer func() { _ = logCloser.Close() }()

	if err := run(); err != nil {
		slog.Error("migrate failed", "error", err)
		os.Exit(1)
	}
}

func setupLogging() (io.Closer, error) {
	cfg, err := logging.ConfigFromEnv()
	if err != nil {
		return nil, err
	}
	return logging.Setup(cfg, nil)
}

func run() error {
	slog.Info("wavelen migrate starting", "version", version.Get())

	src, err := iofs.New(wavelen.MigrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("reading embedded migrations: %w", err)
	}

	dsn, err := dbconfig.DSN(dbconfig.MigrateScheme)
	if err != nil {
		return err
	}

	m, err := migrate.NewWithSourceInstance(sourceName, src, dsn)
	if err != nil {
		return fmt.Errorf("opening the database: %w", err)
	}
	defer closeMigrate(m)

	from, err := currentVersion(m)
	if err != nil {
		return err
	}

	switch err := m.Up(); {
	case errors.Is(err, migrate.ErrNoChange):
		slog.Info("schema already up to date", "version", from)
		return nil
	case err != nil:
		return fmt.Errorf("applying migrations: %w", err)
	}

	to, err := currentVersion(m)
	if err != nil {
		return err
	}
	slog.Info("migrations applied", "from", from, "to", to)
	return nil
}

// currentVersion reports the applied schema version, 0 for an empty db.
// A dirty version returns error: an earlier run failed and we need a human.
func currentVersion(m *migrate.Migrate) (uint, error) {
	v, dirty, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("reading schema version: %w", err)
	}
	if dirty {
		return 0, fmt.Errorf("schema version %d is dirty, a previous migration failed. Needs repair", v)
	}
	return v, nil
}

func closeMigrate(m *migrate.Migrate) {
	if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
		slog.Warn("closing migrate", "source", srcErr, "database", dbErr)
	}
}
