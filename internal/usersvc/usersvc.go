package usersvc

import (
	"context"

	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
)

const DefCollectionName = "Main"

type Users struct {
	storage storage.Storage
}

func New(store storage.Storage) *Users {
	return &Users{storage: store}
}

// Creates the account and its default collection.
func (u *Users) CreateUser(ctx context.Context, user *user.User) error {
	return u.storage.InTx(
		ctx,
		func(s storage.Storage) error {
			if err := s.CreateUser(ctx, user); err != nil {
				return err
			}

			_, err := s.CreateCollection(ctx, user.ID, DefCollectionName, true)
			return err
		},
	)
}

func (u *Users) UserByID(ctx context.Context, id user.ID) (*user.User, error) {
	return u.storage.UserByID(ctx, id)
}
