package storage

import (
	"context"

	"github.com/google/uuid"
	"github.com/salandered/wavelen/internal/auth"
	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/user"
)

type CollectionID uuid.UUID

func (id CollectionID) String() string { return uuid.UUID(id).String() }

type UserRepo interface {
	CreateUser(ctx context.Context, u *user.User) error
	UserByNickname(ctx context.Context, nickname string) (*user.User, error)
	UserByID(ctx context.Context, id user.ID) (*user.User, error)
}

type TokenRepo interface {
	InsertToken(ctx context.Context, t *auth.Token) error
	UserIDForTokenHash(ctx context.Context, hash []byte) (user.ID, error)
	DeleteToken(ctx context.Context, hash []byte) error
}

type CollectionRepo interface {
	CreateCollection(
		ctx context.Context, userID user.ID, name string, isDefault bool,
	) (CollectionID, error)
	LockDefaultCollection(ctx context.Context, userID user.ID) (CollectionID, error)
}

type ColorRepo interface {
	AddColor(ctx context.Context, collectionID CollectionID, hex color.Hex) (bool, error)
	ListColors(ctx context.Context, userID user.ID, p ListColorsParams) (ColorPage, error)
	CountColors(ctx context.Context, collectionID CollectionID) (int, error)
	HasColor(ctx context.Context, collectionID CollectionID, hex color.Hex) (bool, error)
	DeleteColor(ctx context.Context, userID user.ID, hex color.Hex) error
}

type HealthRepo interface {
	Ping(ctx context.Context) error
}

type Storage interface {
	UserRepo
	TokenRepo
	CollectionRepo
	ColorRepo
	HealthRepo

	// Runs fn against a Storage bound to one transaction.
	// Commits when fn returns nil; rolls back if fn returns error.
	InTx(ctx context.Context, fn func(Storage) error) error
}
