package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/salandered/wavelen/internal/collection"
	"github.com/salandered/wavelen/internal/color"
)

// Returns whether the color already was in the collection.
// An unknown collection yields ErrNotFound
func (s *Postgres) AddColor(
	ctx context.Context, cltID collection.ID, hex color.Hex,
) (bool, error) {
	const query = `
		INSERT INTO collection_colors (collection_id, hex, color_key)
		VALUES ($1, $2, $3)
		ON CONFLICT (collection_id, hex) DO NOTHING`

	tag, err := s.db.Exec(ctx, query, cltID, hex, color.Feel(hex))
	if err != nil {
		if pgErrCode(err) == foreignKeyViolation {
			return false, ErrNotFound
		}
		return false, fmt.Errorf("storage add color: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (s *Postgres) CountColors(ctx context.Context, collectionID collection.ID) (int, error) {
	const query = `SELECT count(*) FROM collection_colors WHERE collection_id = $1`

	var n int
	if err := s.db.QueryRow(ctx, query, collectionID).Scan(&n); err != nil {
		return 0, fmt.Errorf("storage count colors: %w", err)
	}
	return n, nil
}

func (s *Postgres) HasColor(
	ctx context.Context, cltID collection.ID, hex color.Hex,
) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM collection_colors WHERE collection_id = $1 AND hex = $2
		)`

	var has bool
	if err := s.db.QueryRow(ctx, query, cltID, hex).Scan(&has); err != nil {
		return false, fmt.Errorf("storage has color: %w", err)
	}
	return has, nil
}

// ErrNotFound if no such row
func (s *Postgres) DeleteColor(
	ctx context.Context, cltID collection.ID, hex color.Hex,
) error {
	const query = `DELETE FROM collection_colors WHERE collection_id = $1 AND hex = $2`

	tag, err := s.db.Exec(ctx, query, cltID, hex)
	if err != nil {
		return fmt.Errorf("storage delete color: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// One page after the cursor, ordered by the column p names with hex as the tiebreak
func (s *Postgres) ListColors(
	ctx context.Context, cltID collection.ID, p ListColorsParams,
) (ColorPage, error) {
	p = p.normalized()

	query, args, err := p.listQuery(cltID)
	if err != nil {
		return ColorPage{}, fmt.Errorf("storage list colors: %w", err)
	}

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return ColorPage{}, fmt.Errorf("storage list colors: %w", err)
	}

	// TODO: add tags to color.Color struct
	colors, err := pgx.CollectRows(rows, pgx.RowToStructByPos[color.Color])
	if err != nil {
		return ColorPage{}, fmt.Errorf("storage list colors: %w", err)
	}

	page := ColorPage{Colors: colors}
	if len(colors) > p.Limit {
		page.Colors, page.HasMore = colors[:p.Limit], true
	}
	return page, nil
}

// Builds the query with all the arguments
func (p ListColorsParams) listQuery(cltID collection.ID) (string, []any, error) {
	const template = `
		SELECT hex, created_at
		FROM collection_colors
		WHERE collection_id = $1 %s
		ORDER BY %s
		LIMIT $2`

	orderBy, predicate, cursorArgs, err := p.keyset()
	if err != nil {
		return "", nil, err
	}

	args := []any{cltID, p.Limit + 1} // one extra row is what HasMore reads
	filter := ""                      // no cursor, no filter: the first page starts at the top
	if p.After != nil {
		filter = "AND " + predicate
		args = append(args, cursorArgs...)
	}
	return fmt.Sprintf(template, filter, orderBy), args, nil
}
