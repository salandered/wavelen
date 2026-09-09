package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/salandered/wavelen/internal/user"
)

// An unknown user yields ErrUserNotFound.
// A second default for the same user violates collections_one_default_per_user.
func (s *Postgres) CreateCollection(
	ctx context.Context, userID user.ID, name string, isDefault bool,
) (CollectionID, error) {
	const query = `
		INSERT INTO collections (user_id, name, is_default)
		VALUES ($1, $2, $3)
		RETURNING id`

	var id CollectionID
	err := s.db.QueryRow(ctx, query, userID, name, isDefault).Scan(&id)
	if err != nil {
		if pgErrCode(err) == foreignKeyViolation {
			return CollectionID{}, ErrUserNotFound
		}
		return CollectionID{}, fmt.Errorf("storage create collection: %w", err)
	}
	return id, nil
}

// Row lock on the user's default collection.
// Makes sense inside a transaction with some logic.
// A user with no def collection yields ErrUserNotFound;
// that should not happen - def collection creation is atomic alongside the user creation
func (s *Postgres) LockDefaultCollection(
	ctx context.Context, userID user.ID,
) (CollectionID, error) {
	const query = `
		SELECT id FROM collections 
		WHERE user_id = $1 AND is_default FOR UPDATE`

	var id CollectionID
	if err := s.db.QueryRow(ctx, query, userID).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CollectionID{}, ErrUserNotFound
		}
		return CollectionID{}, fmt.Errorf("storage lock default collection: %w", err)
	}
	return id, nil
}
