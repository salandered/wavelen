package server_test

import (
	"context"
	"time"
	"uuid"

	"github.com/salandered/wavelen/internal/auth"
	"github.com/salandered/wavelen/internal/clt"
	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
)

// A minimal mocked storage.Storage.

var _ storage.Storage = (*mockStorage)(nil)

type mockStorage struct {
	//// what the methods answer
	// color
	colors          []color.Color
	colorsErr       error
	colorAdded      bool
	addColorErr     error
	colorCount      int
	colorCountErr   error
	hasColor        bool
	hasColorErr     error
	deleteColorErr  error
	deleteColorsErr error
	hasMore         bool
	// collection
	assignCltID     clt.ID
	resolveErr      error
	collections     []clt.Collection
	collectionsErr  error
	collectionCount int
	cltCountErr     error
	byID            *clt.Collection
	cltIsDefault    bool
	deleteCltErr    error
	// user
	assignID    user.ID
	lockUserErr error
	createErr   error
	userByNick  *user.User
	nickErr     error
	userByID    *user.User
	idErr       error
	deleteErr   error
	exported    []storage.CltWithColors
	exportErr   error
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
	gotCollectionID  clt.ID
	gotCltParams     clt.CreateParams
	gotCltUpdate     clt.UpdateParams
	gotHex           color.Hex
	gotParams        storage.ListColorsParams
	pingCalls        int
	deleteUserCalls  int
	inTxCalls        int
}

// not UTC, a response carrying Z proves the handler normalized it.
var stubTime = time.Date(2026, 8, 23, 14, 0, 0, 0, time.FixedZone("+04:00", 4*60*60))

var stubCollectionID = clt.ID(
	uuid.MustParse("01999999-7777-7777-8888-999999999999"))

var otherCollectionID = clt.ID(
	uuid.MustParse("0199aaaa-7777-7777-8888-aaaaaaaaaaaa"))

const stubCollectionName = "Main"

// The account behind the test token.
func stubUser() *user.User {
	return &user.User{ID: 1, Nickname: "olya", CreatedAt: stubTime}
}

// The default collection every account is signed up with.
func stubCollection() clt.Collection {
	return clt.Collection{
		ID:         stubCollectionID,
		Name:       stubCollectionName,
		IconSlug:   clt.DefIconSlug,
		IconAccent: clt.DefIconAccent,
		IsDefault:  true,
		CreatedAt:  stubTime,
	}
}

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

func (s *mockStorage) DeleteUser(_ context.Context, id user.ID) error {
	s.gotUserID = id
	s.deleteUserCalls++
	return s.deleteErr
}

func (s *mockStorage) ExportCollections(
	_ context.Context, userID user.ID,
) ([]storage.CltWithColors, error) {
	s.gotUserID = userID
	return s.exported, s.exportErr
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
	s.inTxCalls++
	return fn(s)
}

// Collections

func (s *mockStorage) CreateCollection(
	_ context.Context, userID user.ID, p clt.CreateParams,
) (*clt.Collection, error) {
	s.gotUserID, s.gotCltParams = userID, p
	return &clt.Collection{
		ID:         s.assignCltID,
		Name:       p.Name,
		IconSlug:   p.Icon,
		IconAccent: p.Accent,
		IsDefault:  p.IsDefault,
		CreatedAt:  stubTime,
	}, nil
}

func (s *mockStorage) ListCollections(
	_ context.Context, userID user.ID,
) ([]clt.Collection, error) {
	s.gotUserID = userID
	return s.collections, s.collectionsErr
}

func (s *mockStorage) CountCollections(_ context.Context, userID user.ID) (int, error) {
	s.gotUserID = userID
	return s.collectionCount, s.cltCountErr
}

func (s *mockStorage) CollectionByID(
	_ context.Context, userID user.ID, id clt.ID,
) (*clt.Collection, error) {
	s.gotUserID, s.gotCollectionID = userID, id
	if s.resolveErr != nil {
		return nil, s.resolveErr
	}
	if s.byID != nil {
		return s.byID, nil
	}
	return &clt.Collection{
		ID:         id,
		Name:       stubCollectionName,
		IconSlug:   clt.DefIconSlug,
		IconAccent: clt.DefIconAccent,
		IsDefault:  s.cltIsDefault,
		CreatedAt:  stubTime,
	}, nil
}

// Merges p into stubCollection
func (s *mockStorage) UpdateCollection(
	_ context.Context, userID user.ID, id clt.ID, p clt.UpdateParams,
) (*clt.Collection, error) {
	s.gotUserID, s.gotCollectionID, s.gotCltUpdate = userID, id, p
	if s.resolveErr != nil {
		return nil, s.resolveErr
	}

	col := stubCollection()
	col.ID = id
	if p.Name != nil {
		col.Name = *p.Name
	}
	if p.Icon != nil {
		col.IconSlug = *p.Icon
	}
	if p.Accent != nil {
		col.IconAccent = *p.Accent
	}
	return &col, nil
}

func (s *mockStorage) DeleteCollection(
	_ context.Context, userID user.ID, id clt.ID,
) error {
	s.gotUserID, s.gotCollectionID = userID, id
	return s.deleteCltErr
}

func (s *mockStorage) ResolveCollection(
	_ context.Context, userID user.ID, id clt.ID,
) (clt.ID, error) {
	s.gotUserID, s.gotCollectionID = userID, id
	if s.resolveErr != nil {
		return clt.ID{}, s.resolveErr
	}
	return id, nil
}

func (s *mockStorage) LockCollection(
	ctx context.Context, userID user.ID, id clt.ID,
) (clt.ID, error) {
	return s.ResolveCollection(ctx, userID, id)
}

// Colors

func (s *mockStorage) CountColors(
	_ context.Context, cltID clt.ID,
) (int, error) {
	s.gotCollectionID = cltID
	return s.colorCount, s.colorCountErr
}

func (s *mockStorage) HasColor(
	_ context.Context, cltID clt.ID, hex color.Hex,
) (bool, error) {
	s.gotCollectionID, s.gotHex = cltID, hex
	return s.hasColor, s.hasColorErr
}

func (s *mockStorage) AddColor(
	_ context.Context, cltID clt.ID, hex color.Hex,
) (bool, error) {
	s.gotCollectionID, s.gotHex = cltID, hex
	return s.colorAdded, s.addColorErr
}

func (s *mockStorage) DeleteColor(
	_ context.Context, cltID clt.ID, hex color.Hex,
) error {
	s.gotCollectionID, s.gotHex = cltID, hex
	return s.deleteColorErr
}

func (s *mockStorage) DeleteAllColors(_ context.Context, cltID clt.ID) error {
	s.gotCollectionID = cltID
	return s.deleteColorsErr
}

func (s *mockStorage) ListColors(
	_ context.Context, cltID clt.ID, p storage.ListColorsParams,
) (storage.ColorPage, error) {
	s.gotCollectionID, s.gotParams = cltID, p
	return storage.ColorPage{Colors: s.colors, HasMore: s.hasMore}, s.colorsErr
}

func (s *mockStorage) Ping(_ context.Context) error {
	s.pingCalls++
	return s.pingErr
}
