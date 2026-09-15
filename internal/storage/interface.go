package storage

import (
	"context"

	"github.com/salandered/wavelen/internal/auth"
	"github.com/salandered/wavelen/internal/clt"
	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/user"
)

type UserRepo interface {
	CreateUser(ctx context.Context, u *user.User) error
	UserByNickname(ctx context.Context, nickname string) (*user.User, error)
	UserByID(ctx context.Context, id user.ID) (*user.User, error)
	DeleteUser(ctx context.Context, id user.ID) error
	LockUser(ctx context.Context, id user.ID) error
}

type TokenRepo interface {
	InsertToken(ctx context.Context, t *auth.Token) error
	UserIDForTokenHash(ctx context.Context, hash []byte) (user.ID, error)
	DeleteToken(ctx context.Context, hash []byte) error
}

type CollectionRepo interface {
	CreateCollection(ctx context.Context, userID user.ID, p clt.CreateParams) (*clt.Collection, error)
	ListCollections(ctx context.Context, userID user.ID) ([]clt.Collection, error)
	CountCollections(ctx context.Context, userID user.ID) (int, error)
	CollectionByID(ctx context.Context, userID user.ID, id clt.ID) (*clt.Collection, error)
	UpdateCollection(ctx context.Context, userID user.ID, id clt.ID, p clt.UpdateParams) (*clt.Collection, error)
	DeleteCollection(ctx context.Context, userID user.ID, id clt.ID) error

	ResolveCollection(ctx context.Context, userID user.ID, id clt.ID) (clt.ID, error)
	LockCollection(ctx context.Context, userID user.ID, id clt.ID) (clt.ID, error)
}

type ColorRepo interface {
	AddColor(ctx context.Context, cltID clt.ID, hex color.Hex) (bool, error)
	ListColors(ctx context.Context, cltID clt.ID, p ListColorsParams) (ColorPage, error)
	CountColors(ctx context.Context, cltID clt.ID) (int, error)
	HasColor(ctx context.Context, cltID clt.ID, hex color.Hex) (bool, error)
	DeleteColor(ctx context.Context, cltID clt.ID, hex color.Hex) error
	DeleteAllColors(ctx context.Context, cltID clt.ID) error
}

type ExportRepo interface {
	ExportCollections(ctx context.Context, userID user.ID) ([]CltWithColors, error)
}

type HealthRepo interface {
	Ping(ctx context.Context) error
}

type Storage interface {
	UserRepo
	TokenRepo
	CollectionRepo
	ColorRepo
	ExportRepo
	HealthRepo

	// Runs fn against a Storage bound to one transaction.
	// Commits when fn returns nil; rolls back if fn returns error.
	InTx(ctx context.Context, fn func(Storage) error) error
}
