package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/salandered/wavelen/internal/user"
)

func (s *Postgres) CreateUser(ctx context.Context, u *user.User) error {
	const query = `
		INSERT INTO users (email, name)
		VALUES ($1, $2)
		RETURNING id, created_at`

	err := s.db.QueryRow(ctx, query, u.Email, u.Name).Scan(&u.ID, &u.CreatedAt)
	if err != nil {
		if pgErrCode(err) == uniqueViolation {
			return ErrDuplicateEmail
		}
		return fmt.Errorf("storage create user: %w", err)
	}
	return nil
}

// Row lock on the user.
// Makes sense inside a transaction with some logic.
// An unknown user yields ErrUserNotFound.
func (s *Postgres) LockUser(ctx context.Context, userID user.ID) error {
	const query = `SELECT 1 FROM users WHERE id = $1 FOR UPDATE`

	var one int
	if err := s.db.QueryRow(ctx, query, userID).Scan(&one); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return fmt.Errorf("storage lock user: %w", err)
	}
	return nil
}
