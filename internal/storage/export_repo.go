package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/salandered/wavelen/internal/collection"
	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/user"
)

type CltWithColors struct {
	Clt    collection.Collection
	Colors []color.Color // empty (not nil) for a collection with no colors
}

// Every user's collection with its colors.
// Collections are ordered by date.
// Colors are ordered by date, hex.
// An unknown user returns zero slice (no error, same as ListCollections).
func (s *Postgres) ExportCollections(
	ctx context.Context, userID user.ID,
) ([]CltWithColors, error) {
	// Note: Reading the data without pagination: the result is bounded by USER_COLLECTION_QUOTA x USER_COLOR_QUOTA.
	// LEFT JOIN - a collection with no colors keeps its row with null hex.
	// ORDER BY c.created_at, c.id - the rows of one collection will be together
	const query = `
		SELECT c.id, c.name, c.icon_slug, c.icon_accent, c.is_default, c.created_at,
		       cc.hex, cc.created_at
		FROM collections c
		LEFT JOIN collection_colors cc ON cc.collection_id = c.id
		WHERE c.user_id = $1
		ORDER BY c.created_at, c.id, cc.created_at, cc.hex`

	rows, err := s.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("storage export collections: %w", err)
	}
	defer rows.Close()

	out := []CltWithColors{}
	for rows.Next() {
		var clt collection.Collection
		var hex *string
		var colorCreatedAt *time.Time

		// Scan into a new clt + the two nullable color fields.
		err := rows.Scan(
			&clt.ID, &clt.Name, &clt.IconSlug, &clt.IconAccent, &clt.IsDefault, &clt.CreatedAt,
			&hex, &colorCreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("storage export collections: %w", err)
		}

		// The rows of one collection are together, like this:
		// c.id  c.created_at  cc.hex   cc.created_at
		// A     t1            #111111  u1
		// A     t1            #222222  u2
		// B     t2            NULL     NULL
		// C     t3            #333333  u3
		//
		// So:
		// - Compare the row's clt id and the id of the clt in out.
		// - If not equal (or this is a first row)
		//   -> the prev clt ended (the group of adjacent rows belonging to one clt).
		//   => append a new clt with an empty color slice.
		if len(out) == 0 || out[len(out)-1].Clt.ID != clt.ID {
			out = append(out, CltWithColors{Clt: clt, Colors: []color.Color{}})
		}
		if hex == nil {
			continue // the collection holds no colors
		}

		// - If equal => add colors to the curr clt.
		current := &out[len(out)-1]
		current.Colors = append(current.Colors, color.Color{
			Hex:       color.Hex(*hex),
			CreatedAt: *colorCreatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage export collections: %w", err)
	}
	return out, nil
}
