package server_test

import (
	"context"
	"time"

	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
)

// A minimal mocked storage.Storage.

type mockStorage struct {
	// what the methods answer
	assignID  user.ID
	createErr error
	added     bool
	addErr    error
	colors    []color.Color
	hasMore   bool
	colorsErr error
	common    []color.Common
	commonErr error
	pingErr   error

	// what the handler passed down
	gotUser          *user.User
	gotUserID        user.ID
	gotHex           color.Hex
	gotParams        storage.ListColorsParams
	gotCatalogParams storage.ListCommonColorsParams
	pingCalls        int
}

// not UTC, a response carrying Z proves the handler normalized it.
var stubTime = time.Date(2026, 8, 23, 14, 0, 0, 0, time.FixedZone("+04:00", 4*60*60))

func newMockStorage() *mockStorage {
	return &mockStorage{assignID: 1, added: true}
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

func (s *mockStorage) AddColor(_ context.Context, userID user.ID, hex color.Hex) (bool, error) {
	s.gotUserID, s.gotHex = userID, hex
	return s.added, s.addErr
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
