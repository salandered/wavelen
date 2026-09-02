package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/salandered/wavelen/internal/auth"
	"github.com/salandered/wavelen/internal/user"
)

func (s *Postgres) InsertToken(ctx context.Context, t *auth.Token) error {
	const query = `
		INSERT INTO tokens (hash, user_id, expiry)
		VALUES ($1, $2, $3)`

	if _, err := s.db.Exec(ctx, query, t.Hash, t.UserID, t.Expiry); err != nil {
		if pgErrCode(err) == foreignKeyViolation {
			return ErrUserNotFound
		}
		return fmt.Errorf("storage insert token: %w", err)
	}
	return nil
}

// The owner of an unexpired token. ErrTokenNotFound in case of unknown or expired.
// No join with users: a token row implies an existing user (cascading on FK).
// Only the user id is read, not the password hash.
func (s *Postgres) UserIDForTokenHash(ctx context.Context, hash []byte) (user.ID, error) {
	const query = `
		SELECT user_id
		FROM tokens
		WHERE hash = $1 AND expiry > now()`

	var id user.ID
	if err := s.db.QueryRow(ctx, query, hash).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrTokenNotFound
		}
		return 0, fmt.Errorf("storage user id for token hash: %w", err)
	}
	return id, nil
}

// Revokes one token. Not found is treated the same as success.
func (s *Postgres) DeleteToken(ctx context.Context, hash []byte) error {
	const query = `
		DELETE FROM tokens
		WHERE hash = $1`

	if _, err := s.db.Exec(ctx, query, hash); err != nil {
		return fmt.Errorf("storage delete token: %w", err)
	}
	return nil
}
