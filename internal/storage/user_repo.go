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
		INSERT INTO users (email, name, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`

	err := s.db.QueryRow(
		ctx, query, u.Email, u.Name, u.PasswordHash,
	).Scan(&u.ID, &u.CreatedAt)

	if err != nil {
		if pgErrCode(err) == uniqueViolation {
			return ErrDuplicateEmail
		}
		return fmt.Errorf("storage create user: %w", err)
	}
	return nil
}

func (s *Postgres) UserByEmail(ctx context.Context, email string) (*user.User, error) {
	const query = `
		SELECT id, email, name, password_hash, created_at
		FROM users
		WHERE email = $1`

	var u user.User

	err := s.db.QueryRow(ctx, query, email).
		Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("storage user by email: %w", err)
	}
	return &u, nil
}

func (s *Postgres) UserByID(ctx context.Context, id user.ID) (*user.User, error) {
	const query = `
		SELECT id, email, name, created_at
		FROM users
		WHERE id = $1`

	var u user.User

	err := s.db.QueryRow(ctx, query, id).Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("storage user by id: %w", err)
	}
	return &u, nil
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
