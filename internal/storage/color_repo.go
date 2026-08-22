package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/user"
)

// Returns false if such color already exists for this user.
// An unknown user yields ErrUserNotFound.
func (s *Store) AddColor(ctx context.Context, userID user.ID, hex color.Hex) (bool, error) {
	const query = `
		INSERT INTO user_colors (user_id, hex)
		VALUES ($1, $2)
		ON CONFLICT (user_id, hex) DO NOTHING`

	tag, err := s.pool.Exec(ctx, query, userID, hex)
	if err != nil {
		if pgErrCode(err) == foreignKeyViolation {
			return false, ErrUserNotFound
		}
		return false, fmt.Errorf("storage add color: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// Newest first. An unknown user is indistinguishable from one with no colors.
func (s *Store) ListColors(ctx context.Context, userID user.ID) ([]color.Color, error) {
	const query = `
		SELECT hex, created_at
		FROM user_colors
		WHERE user_id = $1
		ORDER BY created_at DESC, hex ASC`

	rows, err := s.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("storage list colors: %w", err)
	}

	// TODO: add tags to color.Color struct
	colors, err := pgx.CollectRows(rows, pgx.RowToStructByPos[color.Color])
	if err != nil {
		return nil, fmt.Errorf("storage list colors: %w", err)
	}
	return colors, nil
}
