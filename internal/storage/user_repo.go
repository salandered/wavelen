package storage

import (
	"context"
	"fmt"

	"github.com/salandered/wavelen/internal/user"
)

func (s *Store) CreateUser(ctx context.Context, u *user.User) error {
	const query = `
		INSERT INTO users (email, name)
		VALUES ($1, $2)
		RETURNING id, created_at`

	err := s.pool.QueryRow(ctx, query, u.Email, u.Name).Scan(&u.ID, &u.CreatedAt)
	if err != nil {
		if pgErrCode(err) == uniqueViolation {
			return ErrDuplicateEmail
		}
		return fmt.Errorf("storage create user: %w", err)
	}
	return nil
}
