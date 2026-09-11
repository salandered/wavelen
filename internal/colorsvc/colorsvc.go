package colorsvc

import (
	"context"
	"errors"

	"github.com/salandered/wavelen/internal/collection"
	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
)

var ErrQuotaFull = errors.New("color quota full")

type ColorSvc struct {
	storage storage.Storage
	quota   int
}

func New(store storage.Storage, quota int) *ColorSvc {
	return &ColorSvc{storage: store, quota: quota}
}

// Returns whether the color was added.
// A collection exceeding the quota -> ErrQuotaFull
func (c *ColorSvc) AddColor(
	ctx context.Context, userID user.ID, collectionID collection.ID, hex color.Hex,
) (bool, error) {
	var created bool

	err := c.storage.InTx(
		ctx,
		func(s storage.Storage) error {
			// proves the caller owns it and serializes concurrent adds,
			// so the count below cannot go stale
			owned, err := s.LockCollection(ctx, userID, collectionID)
			if err != nil {
				return err
			}

			n, err := s.CountColors(ctx, owned)
			if err != nil {
				return err
			}

			if n >= c.quota {
				has, err := s.HasColor(ctx, owned, hex)
				if err != nil {
					return err
				}
				if !has {
					return ErrQuotaFull
				}
				created = false
				return nil
			}

			created, err = s.AddColor(ctx, owned, hex)
			return err
		},
	)
	if err != nil {
		return false, err
	}
	return created, nil
}

// ErrNotFound if no color or no collection
func (c *ColorSvc) DeleteColor(
	ctx context.Context, userID user.ID, collectionID collection.ID, hex color.Hex,
) error {
	owned, err := c.storage.ResolveCollection(ctx, userID, collectionID)
	if err != nil {
		return err
	}
	return c.storage.DeleteColor(ctx, owned, hex)
}

// Deletes all colors from the collection.
func (c *ColorSvc) DeleteAllColors(
	ctx context.Context, userID user.ID, collectionID collection.ID,
) error {
	owned, err := c.storage.ResolveCollection(ctx, userID, collectionID)
	if err != nil {
		return err
	}
	return c.storage.DeleteAllColors(ctx, owned)
}

// ErrNotFound if no collection.
func (c *ColorSvc) ListColors(
	ctx context.Context, userID user.ID, collectionID collection.ID,
	p storage.ListColorsParams,
) (storage.ColorPage, error) {
	owned, err := c.storage.ResolveCollection(ctx, userID, collectionID)
	if err != nil {
		return storage.ColorPage{}, err
	}
	return c.storage.ListColors(ctx, owned, p)
}
