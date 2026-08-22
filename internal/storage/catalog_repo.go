package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/salandered/wavelen/internal/color"
)

func (s *Store) ListCommonColors(ctx context.Context) ([]color.Common, error) {
	const query = `
		SELECT hex, name
		FROM common_colors
		ORDER BY name`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("storage list common colors: %w", err)
	}

	common, err := pgx.CollectRows(rows, pgx.RowToStructByPos[color.Common])
	if err != nil {
		return nil, fmt.Errorf("storage list common colors: %w", err)
	}
	return common, nil
}
