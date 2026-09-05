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
		INSERT INTO users (nickname, name, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`

	err := s.db.QueryRow(
		ctx, query, u.Nickname, u.Name, u.PasswordHash,
	).Scan(&u.ID, &u.CreatedAt)

	if err != nil {
		if pgErrCode(err) == uniqueViolation {
			return ErrDuplicateNickname
		}
		return fmt.Errorf("storage create user: %w", err)
	}
	return nil
}

// Fills in u.ID and u.CreatedAt. A taken nickname -> ErrDuplicateNickname.
func (s *Postgres) UserByNickname(ctx context.Context, nickname string) (*user.User, error) {
	const query = `
		SELECT id, nickname, name, password_hash, created_at
		FROM users
		WHERE nickname = $1`

	var u user.User

	err := s.db.QueryRow(ctx, query, nickname).
		Scan(&u.ID, &u.Nickname, &u.Name, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("storage user by nickname: %w", err)
	}
	return &u, nil
}

func (s *Postgres) UserByID(ctx context.Context, id user.ID) (*user.User, error) {
	const query = `
		SELECT id, nickname, name, created_at
		FROM users
		WHERE id = $1`

	var u user.User

	err := s.db.QueryRow(ctx, query, id).Scan(&u.ID, &u.Nickname, &u.Name, &u.CreatedAt)
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
