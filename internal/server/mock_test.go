package server_test

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/salandered/wavelen/internal/auth"
	"github.com/salandered/wavelen/internal/collection"
	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
)

// A minimal mocked storage.Storage.

var _ storage.Storage = (*mockStorage)(nil)

type mockStorage struct {
	//// what the methods answer
	// color
	colors         []color.Color
	colorsErr      error
	colorAdded     bool
	addColorErr    error
	colorCount     int
	colorCountErr  error
	hasColor       bool
	hasColorErr    error
	deleteColorErr error
	hasMore        bool
	// collection
	assignCltID     collection.ID
	resolveErr      error
	collections     []collection.Collection
	collectionsErr  error
	collectionCount int
	collCountErr    error
	byID            *collection.Collection
	collIsDefault   bool
	deleteCollErr   error
	// user
	assignID    user.ID
	lockUserErr error
	createErr   error
	userByNick  *user.User
	nickErr     error
	userByID    *user.User
	idErr       error
	// token
	tokenUser      user.ID
	tokenErr       error
	insertErr      error
	deleteTokenErr error
	// misc
	pingErr error

	//// what the handler registered
	gotUser          *user.User
	gotNickname      string
	gotToken         *auth.Token
	gotTokenHash     []byte
	deletedTokenHash []byte
	gotUserID        user.ID
	gotCollectionID  collection.ID
	gotCollName      string
	gotHex           color.Hex
	gotParams        storage.ListColorsParams
	pingCalls        int
}

// not UTC, a response carrying Z proves the handler normalized it.
var stubTime = time.Date(2026, 8, 23, 14, 0, 0, 0, time.FixedZone("+04:00", 4*60*60))

var stubCollectionID = collection.ID(
	uuid.MustParse("01999999-7777-7777-8888-999999999999"))

var otherCollectionID = collection.ID(
	uuid.MustParse("0199aaaa-7777-7777-8888-aaaaaaaaaaaa"))

const stubCollectionName = "Main"

func newMockStorage() *mockStorage {
	// tokenUser - the color tests get their user id from the token
	return &mockStorage{
		assignID:    1,
		assignCltID: stubCollectionID,
		colorAdded:  true,
		tokenUser:   1,
	}
}

// Clears the mock storage.
// In place, the running server holds this pointer.
func (s *mockStorage) reset() {
	*s = *newMockStorage()
}

func (s *mockStorage) CreateUser(_ context.Context, u *user.User) error {
	passed := *u
	s.gotUser = &passed

	if s.createErr != nil {
		return s.createErr
	}
	u.ID = s.assignID
	u.CreatedAt = stubTime
	return nil
}

func (s *mockStorage) UserByNickname(_ context.Context, nickname string) (*user.User, error) {
	s.gotNickname = nickname
	return s.userByNick, s.nickErr
}

func (s *mockStorage) UserByID(_ context.Context, id user.ID) (*user.User, error) {
	s.gotUserID = id
	return s.userByID, s.idErr
}

func (s *mockStorage) LockUser(_ context.Context, id user.ID) error {
	s.gotUserID = id
	return s.lockUserErr
}

func (s *mockStorage) InsertToken(_ context.Context, t *auth.Token) error {
	passed := *t
	s.gotToken = &passed
	return s.insertErr
}

func (s *mockStorage) UserIDForTokenHash(_ context.Context, hash []byte) (user.ID, error) {
	s.gotTokenHash = hash
	return s.tokenUser, s.tokenErr
}

func (s *mockStorage) DeleteToken(_ context.Context, hash []byte) error {
	s.deletedTokenHash = hash
	return s.deleteTokenErr
}

func (s *mockStorage) InTx(_ context.Context, fn func(storage.Storage) error) error {
	return fn(s)
}

// Collections

func (s *mockStorage) CreateCollection(
	_ context.Context, userID user.ID, name string, isDefault bool,
) (*collection.Collection, error) {
	s.gotUserID, s.gotCollName = userID, name
	return &collection.Collection{
		ID:        s.assignCltID,
		Name:      name,
		IsDefault: isDefault,
		CreatedAt: stubTime,
	}, nil
}

func (s *mockStorage) ListCollections(
	_ context.Context, userID user.ID,
) ([]collection.Collection, error) {
	s.gotUserID = userID
	return s.collections, s.collectionsErr
}

func (s *mockStorage) CountCollections(_ context.Context, userID user.ID) (int, error) {
	s.gotUserID = userID
	return s.collectionCount, s.collCountErr
}

func (s *mockStorage) CollectionByID(
	_ context.Context, userID user.ID, id collection.ID,
) (*collection.Collection, error) {
	s.gotUserID, s.gotCollectionID = userID, id
	if s.resolveErr != nil {
		return nil, s.resolveErr
	}
	if s.byID != nil {
		return s.byID, nil
	}
	return &collection.Collection{
		ID:        id,
		Name:      stubCollectionName,
		IsDefault: s.collIsDefault,
		CreatedAt: stubTime,
	}, nil
}

func (s *mockStorage) DeleteCollection(
	_ context.Context, userID user.ID, id collection.ID,
) error {
	s.gotUserID, s.gotCollectionID = userID, id
	return s.deleteCollErr
}

func (s *mockStorage) ResolveCollection(
	_ context.Context, userID user.ID, id collection.ID,
) (collection.ID, error) {
	s.gotUserID, s.gotCollectionID = userID, id
	if s.resolveErr != nil {
		return collection.ID{}, s.resolveErr
	}
	return id, nil
}

func (s *mockStorage) LockCollection(
	ctx context.Context, userID user.ID, id collection.ID,
) (collection.ID, error) {
	return s.ResolveCollection(ctx, userID, id)
}

// Colors

func (s *mockStorage) CountColors(
	_ context.Context, cltID collection.ID,
) (int, error) {
	s.gotCollectionID = cltID
	return s.colorCount, s.colorCountErr
}

func (s *mockStorage) HasColor(
	_ context.Context, cltID collection.ID, hex color.Hex,
) (bool, error) {
	s.gotCollectionID, s.gotHex = cltID, hex
	return s.hasColor, s.hasColorErr
}

func (s *mockStorage) AddColor(
	_ context.Context, cltID collection.ID, hex color.Hex,
) (bool, error) {
	s.gotCollectionID, s.gotHex = cltID, hex
	return s.colorAdded, s.addColorErr
}

func (s *mockStorage) DeleteColor(
	_ context.Context, cltID collection.ID, hex color.Hex,
) error {
	s.gotCollectionID, s.gotHex = cltID, hex
	return s.deleteColorErr
}

func (s *mockStorage) ListColors(
	_ context.Context, cltID collection.ID, p storage.ListColorsParams,
) (storage.ColorPage, error) {
	s.gotCollectionID, s.gotParams = cltID, p
	return storage.ColorPage{Colors: s.colors, HasMore: s.hasMore}, s.colorsErr
}

func (s *mockStorage) Ping(_ context.Context) error {
	s.pingCalls++
	return s.pingErr
}
