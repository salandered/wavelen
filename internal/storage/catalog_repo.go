package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/salandered/wavelen/internal/color"
)

func (s *Postgres) ListCommonColors(
	ctx context.Context, p ListCommonColorsParams,
) ([]color.Common, error) {
	const template = `
		SELECT hex, name
		FROM common_colors
		ORDER BY %s`

	orderBy, err := p.normalized().orderBy()
	if err != nil {
		return nil, fmt.Errorf("storage list common colors: %w", err)
	}

	rows, err := s.db.Query(ctx, fmt.Sprintf(template, orderBy))
	if err != nil {
		return nil, fmt.Errorf("storage list common colors: %w", err)
	}

	common, err := pgx.CollectRows(rows, pgx.RowToStructByPos[color.Common])
	if err != nil {
		return nil, fmt.Errorf("storage list common colors: %w", err)
	}
	return common, nil
}
