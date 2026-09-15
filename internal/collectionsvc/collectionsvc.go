package collectionsvc

import (
	"context"
	"errors"

	"github.com/salandered/wavelen/internal/clt"
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
	ctx context.Context, userID user.ID, p clt.CreateParams,
) (*clt.Collection, error) {
	p.IsDefault = false // the default one is written at signup only

	var created *clt.Collection

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

			created, err = s.CreateCollection(ctx, userID, p)
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
) ([]clt.Collection, error) {
	return c.storage.ListCollections(ctx, userID)
}

func (c *CollectionSvc) CollectionByID(
	ctx context.Context, userID user.ID, id clt.ID,
) (*clt.Collection, error) {
	return c.storage.CollectionByID(ctx, userID, id)
}

// one row to write, no lock and no transaction.
func (c *CollectionSvc) UpdateCollection(
	ctx context.Context, userID user.ID, id clt.ID, p clt.UpdateParams,
) (*clt.Collection, error) {
	return c.storage.UpdateCollection(ctx, userID, id, p)
}

func (c *CollectionSvc) DeleteCollection(
	ctx context.Context, userID user.ID, id clt.ID,
) error {
	// no storage.InTx: is_default can't be changed
	col, err := c.storage.CollectionByID(ctx, userID, id)
	if err != nil {
		return err
	}
	if col.IsDefault {
		return ErrDeleteDefault
	}
	return c.storage.DeleteCollection(ctx, userID, id)
}
