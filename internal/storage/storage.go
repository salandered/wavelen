package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrUserNotFound   = fmt.Errorf("%w: user", ErrNotFound)
	ErrTokenNotFound  = fmt.Errorf("%w: token", ErrNotFound)
	ErrDuplicateEmail = errors.New("duplicate email")

	ErrNestedTx = errors.New("nested transaction")
)

// SQLSTATE codes, see https://www.postgresql.org/docs/current/errcodes-appendix.html
const (
	uniqueViolation     = "23505"
	foreignKeyViolation = "23503"
)

// pgErrCode returns "" when err did not come from the server.
func pgErrCode(err error) string {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		return pgErr.Code
	}
	return ""
}

// Unit of work transactional boundaries
// See dev/dbtx.md

// Satisfied by [*pgxpool.Pool] and [pgx.Tx].
type querier interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// satisfies [Storage]
type Postgres struct {
	db querier // the pool, or the transaction Tx
}

// compile-time assertions
var (
	_ Storage = (*Postgres)(nil)
	_ querier = (*pgxpool.Pool)(nil)
	_ querier = pgx.Tx(nil)
)

func New(pool *pgxpool.Pool) *Postgres {
	return &Postgres{db: pool}
}

func (s *Postgres) InTx(ctx context.Context, fn func(Storage) error) error {
	// Note: [pgx.Tx.Begin] opens a savepoint, nesting would work.
	// We prevent it regardless.
	if _, ok := s.db.(*pgxpool.Pool); !ok {
		return ErrNestedTx
	}
	return pgx.BeginFunc(
		ctx,
		s.db,
		func(tx pgx.Tx) error {
			return fn(&Postgres{db: tx})
		},
	)
}

// health repo method
func (s *Postgres) Ping(ctx context.Context) error {
	// used to be, but db [querier] don't have a Ping
	// return s.pool.Ping(ctx)
	_, err := s.db.Exec(ctx, `SELECT 1`)
	return err
}
