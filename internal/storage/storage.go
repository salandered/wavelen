package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrUserNotFound   = fmt.Errorf("%w: user", ErrNotFound)
	ErrDuplicateEmail = errors.New("duplicate email")
)

// Postgres related

// SQLSTATE codes, see https://www.postgresql.org/docs/current/errcodes-appendix.html
const (
	uniqueViolation     = "23505"
	foreignKeyViolation = "23503"
)

type Store struct {
	pool *pgxpool.Pool
}

var _ Storage = (*Store)(nil) // compile-time interface assertion

// New wraps an already connected pool. The caller owns its lifetime.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// pgErrCode returns "" when err did not come from the server.
func pgErrCode(err error) string {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		return pgErr.Code
	}
	return ""
}
