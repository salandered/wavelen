package dbconfig_test

import (
	"net/url"
	"testing"

	"github.com/salandered/wavelen/internal/dbconfig"
	"github.com/stretchr/testify/require"
)

// Need to set explicitly so a dev's .env would not affect tests
// (`make test` would use exports POSTGRES_* from .env).
// DSN treats an empty value as unset.
func setPostgresEnv(t *testing.T, user, password, host, port, db, sslmode string) {
	t.Helper()
	t.Setenv("POSTGRES_USER", user)
	t.Setenv("POSTGRES_PASSWORD", password)
	t.Setenv("POSTGRES_HOST", host)
	t.Setenv("POSTGRES_PORT", port)
	t.Setenv("POSTGRES_DB", db)
	t.Setenv("POSTGRES_SSLMODE", sslmode)
}

func TestDSNFillsInDefaultsWhenOnlyCredentialsAreSet(t *testing.T) {
	setPostgresEnv(t, "wavelen", "secret", "", "", "", "")

	got, err := dbconfig.DSN("postgres")

	require.NoError(t, err)
	require.Equal(t, "postgres://wavelen:secret@localhost:5433/wavelen?sslmode=disable", got)
}

func TestDSNUsesEveryPostgresEnvVarWhenSet(t *testing.T) {
	setPostgresEnv(t, "admin", "hunter2", "db.example.com", "5439", "prod", "require")

	got, err := dbconfig.DSN("postgres")

	require.NoError(t, err)
	require.Equal(t, "postgres://admin:hunter2@db.example.com:5439/prod?sslmode=require", got)
}

func TestDSNUsesTheGivenScheme(t *testing.T) {
	setPostgresEnv(t, "wavelen", "secret", "", "", "", "")

	got, err := dbconfig.DSN("pgx5")

	require.NoError(t, err)
	require.Equal(t, "pgx5://wavelen:secret@localhost:5433/wavelen?sslmode=disable", got)
}

func TestDSNEscapesSpecialCharactersInCredentials(t *testing.T) {
	const password = `p@ss:w/rd?#%&=+ ' "`
	setPostgresEnv(t, "us:er", password, "", "", "", "")

	got, err := dbconfig.DSN("postgres")
	require.NoError(t, err)

	// got represents an unreadable escaped form: parse back to check
	parsed, err := url.Parse(got)
	require.NoError(t, err)
	require.Equal(t, "us:er", parsed.User.Username())
	gotPassword, set := parsed.User.Password()
	require.True(t, set)
	require.Equal(t, password, gotPassword)
	require.Equal(t, "localhost:5433", parsed.Host)
}

func TestDSNBracketsAnIPv6Host(t *testing.T) {
	setPostgresEnv(t, "wavelen", "secret", "::1", "5432", "", "")

	got, err := dbconfig.DSN("postgres")

	require.NoError(t, err)
	require.Equal(t, "postgres://wavelen:secret@[::1]:5432/wavelen?sslmode=disable", got)
}

func TestDSNFailsWithoutCreds(t *testing.T) {
	tests := map[string]struct{ user, password string }{
		"both missing":     {"", ""},
		"password missing": {"wavelen", ""},
		"user missing":     {"", "secret"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			setPostgresEnv(t, tt.user, tt.password, "", "", "", "")

			_, err := dbconfig.DSN("postgres")

			require.ErrorIs(t, err, dbconfig.ErrMissingCreds)
		})
	}
}
