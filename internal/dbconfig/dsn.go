// Package dbconfig builds the database DSN from envs.
// cmd/api and cmd/migrate should use it.
package dbconfig

import (
	"errors"
	"net"
	"net/url"
	"os"
)

const (
	defaultHost    = "localhost"
	defaultPort    = "5433"
	defaultName    = "wavelen"
	defaultSSLMode = "disable"
)

// MigrateScheme is the DSN scheme golang-migrate maps to its pgx/v5 driver.
const MigrateScheme = "pgx5"

var ErrMissingCreds = errors.New("POSTGRES_USER and POSTGRES_PASSWORD are required")

// DSN reads POSTGRES_* and returns a conn string.
// 'scheme' selects the driver: "postgres" for a pgx pool, "pgx5" for golang-migrate.
func DSN(scheme string) (string, error) {
	user, password := os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD")
	if user == "" || password == "" {
		return "", ErrMissingCreds
	}

	// The general form is:
	// [scheme:][//[userinfo@]host][/]path[?query][#fragment]
	// Note: fmt.Sprintf fails if the password contains characters like '%', '/'; url.URL escapes every part
	dsn := url.URL{
		Scheme: scheme,
		User:   url.UserPassword(user, password),
		Host: net.JoinHostPort(
			stringFromEnv("POSTGRES_HOST", defaultHost),
			stringFromEnv("POSTGRES_PORT", defaultPort),
		),
		Path: "/" + stringFromEnv("POSTGRES_DB", defaultName),
		RawQuery: url.Values{
			"sslmode": {
				// explicit default, see https://pkg.go.dev/github.com/jackc/pgx/v5/pgconn#hdr-Connecting_Securely
				stringFromEnv("POSTGRES_SSLMODE", defaultSSLMode),
			},
		}.Encode(),
	}
	return dsn.String(), nil
}

func stringFromEnv(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}
