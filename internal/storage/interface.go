package storage

import (
	"context"

	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/user"
)

type UserRepo interface {
	// Fills in u.ID and u.CreatedAt. A taken email yields ErrDuplicateEmail.
	CreateUser(ctx context.Context, u *user.User) error
}

type ColorRepo interface {
	AddColor(ctx context.Context, userID user.ID, hex color.Hex) (bool, error)
	ListColors(ctx context.Context, userID user.ID) ([]color.Color, error)
}

type CatalogRepo interface {
	// The shared palette, by name.
	ListCommonColors(ctx context.Context) ([]color.Common, error)
}

type HealthRepo interface {
	Ping(ctx context.Context) error
}

type Storage interface {
	UserRepo
	ColorRepo
	CatalogRepo
	HealthRepo
}
