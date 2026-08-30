package colorsvc

import (
	"context"
	"errors"

	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
)

var ErrQuotaFull = errors.New("color quota full")

type Colors struct {
	storage storage.Storage
	quota   int
}

func New(store storage.Storage, quota int) *Colors {
	return &Colors{storage: store, quota: quota}
}

// Reports whether the color was added.
// A user exceeding the quota -> ErrQuotaFull
func (c *Colors) AddColor(ctx context.Context, userID user.ID, hex color.Hex) (bool, error) {
	var created bool

	err := c.storage.InTx(
		ctx,
		func(s storage.Storage) error {
			// serializes this user's concurrent adds, so the count below cannot go stale
			if err := s.LockUser(ctx, userID); err != nil {
				return err
			}

			n, err := s.CountColors(ctx, userID)
			if err != nil {
				return err
			}

			if n >= c.quota {
				has, err := s.HasColor(ctx, userID, hex)
				if err != nil {
					return err
				}
				if !has {
					return ErrQuotaFull
				}
				created = false
				return nil
			}

			created, err = s.AddColor(ctx, userID, hex)
			return err
		},
	)
	if err != nil {
		return false, err
	}
	return created, nil
}

// ErrNotFound if retried.
func (c *Colors) DeleteColor(ctx context.Context, userID user.ID, hex color.Hex) error {
	return c.storage.DeleteColor(ctx, userID, hex)
}

func (c *Colors) ListColors(
	ctx context.Context, userID user.ID, p storage.ListColorsParams,
) (storage.ColorPage, error) {
	return c.storage.ListColors(ctx, userID, p)
}
