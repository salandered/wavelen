package usersvc

import (
	"context"

	"github.com/salandered/wavelen/internal/collection"
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

			_, err := s.CreateCollection(ctx, user.ID, collection.CreateParams{
				Name:      DefCollectionName,
				Icon:      collection.DefIconSlug,
				Accent:    collection.DefIconAccent,
				IsDefault: true,
			})
			return err
		},
	)
}

func (u *Users) UserByID(ctx context.Context, id user.ID) (*user.User, error) {
	return u.storage.UserByID(ctx, id)
}

// Deletes the account.
func (u *Users) DeleteUser(ctx context.Context, id user.ID) error {
	return u.storage.DeleteUser(ctx, id)
}

// Export user data: account info, collection and color data.
// Note that the two reads use READ COMMITTED (see docs/dbtx.md).
// TODO: check what if the user is deleted between the two reads.
func (u *Users) ExportUser(
	ctx context.Context, id user.ID,
) (*user.User, []storage.CltWithColors, error) {
	var (
		usr         *user.User
		collections []storage.CltWithColors
	)

	err := u.storage.InTx(ctx, func(s storage.Storage) error {
		var err error
		if usr, err = s.UserByID(ctx, id); err != nil {
			return err
		}
		collections, err = s.ExportCollections(ctx, id)
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	return usr, collections, nil
}
