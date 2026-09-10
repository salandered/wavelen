package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/salandered/wavelen/internal/collection"
	"github.com/salandered/wavelen/internal/user"
)

// Returns a collection with a db-generated id and created_at.
// An unknown user yields ErrUserNotFound.
// A second default for the same user violates collections_one_default_per_user.
func (s *Postgres) CreateCollection(
	ctx context.Context, userID user.ID, p collection.CreateParams,
) (*collection.Collection, error) {
	const query = `
		INSERT INTO collections (user_id, name, icon_slug, icon_accent, is_default)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, icon_slug, icon_accent, is_default, created_at`

	var c collection.Collection
	err := s.db.QueryRow(ctx, query, userID, p.Name, p.Icon, p.Accent, p.IsDefault).
		Scan(&c.ID, &c.Name, &c.IconSlug, &c.IconAccent, &c.IsDefault, &c.CreatedAt)
	if err != nil {
		if pgErrCode(err) == foreignKeyViolation {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("storage create collection: %w", err)
	}
	return &c, nil
}

// Oldest first. Empty of an unknown user.
func (s *Postgres) ListCollections(
	ctx context.Context, userID user.ID,
) ([]collection.Collection, error) {
	const query = `
		SELECT id, name, icon_slug, icon_accent, is_default, created_at
		FROM collections
		WHERE user_id = $1
		ORDER BY created_at, id`

	rows, err := s.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("storage list collections: %w", err)
	}

	collections, err := pgx.CollectRows(rows, pgx.RowToStructByPos[collection.Collection])
	if err != nil {
		return nil, fmt.Errorf("storage list collections: %w", err)
	}
	return collections, nil
}

func (s *Postgres) CountCollections(ctx context.Context, userID user.ID) (int, error) {
	const query = `SELECT count(*) FROM collections WHERE user_id = $1`

	var n int
	if err := s.db.QueryRow(ctx, query, userID).Scan(&n); err != nil {
		return 0, fmt.Errorf("storage count collections: %w", err)
	}
	return n, nil
}

func (s *Postgres) CollectionByID(
	ctx context.Context, userID user.ID, id collection.ID,
) (*collection.Collection, error) {
	const query = `
		SELECT id, name, icon_slug, icon_accent, is_default, created_at
		FROM collections
		WHERE id = $1 AND user_id = $2`

	var c collection.Collection
	err := s.db.QueryRow(ctx, query, id, userID).
		Scan(&c.ID, &c.Name, &c.IconSlug, &c.IconAccent, &c.IsDefault, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("storage collection by id: %w", err)
	}
	return &c, nil
}

// Cascades to the collection's colors.
// Note that ErrNotFound covers both - no such collection or no such collection for this user;
// it is ok and good: if a user requested someone else's collection the result should be 404.
func (s *Postgres) DeleteCollection(
	ctx context.Context, userID user.ID, id collection.ID,
) error {
	const query = `DELETE FROM collections WHERE id = $1 AND user_id = $2`

	tag, err := s.db.Exec(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("storage delete collection: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Returns collection id or error
func (s *Postgres) ResolveCollection(
	ctx context.Context, userID user.ID, id collection.ID,
) (collection.ID, error) {
	return s.resolveCollection(ctx, userID, id, false)
}

// ResolveCollection plus a row lock
func (s *Postgres) LockCollection(
	ctx context.Context, userID user.ID, id collection.ID,
) (collection.ID, error) {
	return s.resolveCollection(ctx, userID, id, true)
}

func (s *Postgres) resolveCollection(
	ctx context.Context, userID user.ID, id collection.ID, lock bool,
) (collection.ID, error) {
	query := `SELECT id FROM collections WHERE id = $1 AND user_id = $2`
	if lock {
		query += ` FOR UPDATE`
	}
	var found collection.ID
	if err := s.db.QueryRow(ctx, query, id, userID).Scan(&found); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return collection.ID{}, ErrNotFound
		}
		return collection.ID{}, fmt.Errorf("storage resolve collection: %w", err)
	}
	return found, nil
}
