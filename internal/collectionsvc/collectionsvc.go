package collectionsvc

import (
	"context"
	"errors"

	"github.com/salandered/wavelen/internal/collection"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
)

var (
	ErrQuotaFull     = errors.New("collection quota full")
	ErrDeleteDefault = errors.New("the default collection cannot be deleted")
)

type CollectionSvc struct {
	storage storage.Storage
	quota   int
}

func New(store storage.Storage, quota int) *CollectionSvc {
	return &CollectionSvc{storage: store, quota: quota}
}

// A user already at the quota -> ErrQuotaFull.
func (c *CollectionSvc) CreateCollection(
	ctx context.Context, userID user.ID, name string,
) (*collection.Collection, error) {
	var created *collection.Collection

	err := c.storage.InTx(
		ctx,
		func(s storage.Storage) error {
			// serializes this user's concurrent creates, the count below cannot go stale
			if err := s.LockUser(ctx, userID); err != nil {
				return err
			}

			n, err := s.CountCollections(ctx, userID)
			if err != nil {
				return err
			}
			if n >= c.quota {
				return ErrQuotaFull
			}

			created, err = s.CreateCollection(ctx, userID, name, false)
			return err
		},
	)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (c *CollectionSvc) ListCollections(
	ctx context.Context, userID user.ID,
) ([]collection.Collection, error) {
	return c.storage.ListCollections(ctx, userID)
}

func (c *CollectionSvc) CollectionByID(
	ctx context.Context, userID user.ID, id collection.ID,
) (*collection.Collection, error) {
	return c.storage.CollectionByID(ctx, userID, id)
}

func (c *CollectionSvc) DeleteCollection(
	ctx context.Context, userID user.ID, id collection.ID,
) error {
	// Not using storage.InTx becase the default collection cannot be changed currently
	col, err := c.storage.CollectionByID(ctx, userID, id)
	if err != nil {
		return err
	}
	if col.IsDefault {
		return ErrDeleteDefault
	}
	return c.storage.DeleteCollection(ctx, userID, id)
}
