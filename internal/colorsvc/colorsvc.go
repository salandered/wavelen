package colorsvc

import (
	"context"
	"errors"

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
func (c *ColorSvc) AddColor(ctx context.Context, userID user.ID, hex color.Hex) (bool, error) {
	var created bool

	err := c.storage.InTx(
		ctx,
		func(s storage.Storage) error {
			// resolves the collection and serializes this user's concurrent adds to it,
			// so the count below cannot go stale
			collectionID, err := s.LockDefaultCollection(ctx, userID)
			if err != nil {
				return err
			}

			n, err := s.CountColors(ctx, collectionID)
			if err != nil {
				return err
			}

			if n >= c.quota {
				has, err := s.HasColor(ctx, collectionID, hex)
				if err != nil {
					return err
				}
				if !has {
					return ErrQuotaFull
				}
				created = false
				return nil
			}

			created, err = s.AddColor(ctx, collectionID, hex)
			return err
		},
	)
	if err != nil {
		return false, err
	}
	return created, nil
}

// ErrNotFound if retried.
func (c *ColorSvc) DeleteColor(ctx context.Context, userID user.ID, hex color.Hex) error {
	return c.storage.DeleteColor(ctx, userID, hex)
}

func (c *ColorSvc) ListColors(
	ctx context.Context, userID user.ID, p storage.ListColorsParams,
) (storage.ColorPage, error) {
	return c.storage.ListColors(ctx, userID, p)
}
