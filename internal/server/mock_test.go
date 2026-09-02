package server_test

import (
	"context"
	"time"

	"github.com/salandered/wavelen/internal/auth"
	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
)

// A minimal mocked storage.Storage.

var _ storage.Storage = (*mockStorage)(nil)

type mockStorage struct {
	// what the methods answer
	assignID       user.ID
	createErr      error
	lockErr        error
	added          bool
	addErr         error
	colorCount     int
	countErr       error
	hasColor       bool
	hasErr         error
	deleteErr      error
	colors         []color.Color
	hasMore        bool
	colorsErr      error
	common         []color.Common
	commonErr      error
	pingErr        error
	userByMail     *user.User
	mailErr        error
	tokenUser      user.ID
	tokenErr       error
	insertErr      error
	deleteTokenErr error

	// what the handler passed down
	gotUser          *user.User
	gotEmail         string
	gotToken         *auth.Token
	gotTokenHash     []byte
	deletedTokenHash []byte
	gotUserID        user.ID
	gotHex           color.Hex
	gotParams        storage.ListColorsParams
	gotCatalogParams storage.ListCommonColorsParams
	pingCalls        int
}

// not UTC, a response carrying Z proves the handler normalized it.
var stubTime = time.Date(2026, 8, 23, 14, 0, 0, 0, time.FixedZone("+04:00", 4*60*60))

func newMockStorage() *mockStorage {
	// tokenUser - most of the color tests call /users/1/...
	return &mockStorage{assignID: 1, added: true, tokenUser: 1}
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

func (s *mockStorage) UserByEmail(_ context.Context, email string) (*user.User, error) {
	s.gotEmail = email
	return s.userByMail, s.mailErr
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

func (s *mockStorage) LockUser(_ context.Context, userID user.ID) error {
	s.gotUserID = userID
	return s.lockErr
}

func (s *mockStorage) CountColors(_ context.Context, userID user.ID) (int, error) {
	s.gotUserID = userID
	return s.colorCount, s.countErr
}

func (s *mockStorage) HasColor(_ context.Context, userID user.ID, hex color.Hex) (bool, error) {
	s.gotUserID, s.gotHex = userID, hex
	return s.hasColor, s.hasErr
}

func (s *mockStorage) AddColor(_ context.Context, userID user.ID, hex color.Hex) (bool, error) {
	s.gotUserID, s.gotHex = userID, hex
	return s.added, s.addErr
}

func (s *mockStorage) DeleteColor(_ context.Context, userID user.ID, hex color.Hex) error {
	s.gotUserID, s.gotHex = userID, hex
	return s.deleteErr
}

func (s *mockStorage) ListColors(
	_ context.Context, userID user.ID, p storage.ListColorsParams,
) (storage.ColorPage, error) {
	s.gotUserID, s.gotParams = userID, p
	return storage.ColorPage{Colors: s.colors, HasMore: s.hasMore}, s.colorsErr
}

func (s *mockStorage) ListCommonColors(
	_ context.Context, p storage.ListCommonColorsParams,
) ([]color.Common, error) {
	s.gotCatalogParams = p
	return s.common, s.commonErr
}

func (s *mockStorage) Ping(_ context.Context) error {
	s.pingCalls++
	return s.pingErr
}
