package server_test

import (
	"context"
	"time"

	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/user"
)

// A canned storage.Storage with no behaviour of its own. Each method records what the
// handler passed down and returns whatever the test set.
//
// Storage semantics - dedup, email uniqueness, ordering, per-user scoping - are proven
// in internal/storage against real Postgres. Reimplementing them here would only test
// the double.
type stubStorage struct {
	// what the methods answer
	assignID  user.ID
	createErr error
	added     bool
	addErr    error
	colors    []color.Color
	colorsErr error
	common    []color.Common
	commonErr error

	// what the handler passed down
	gotUser   *user.User
	gotUserID user.ID
	gotHex    color.Hex
}

// Deliberately not UTC, so a response carrying Z proves the handler normalized it.
var stubTime = time.Date(2026, 8, 23, 14, 0, 0, 0, time.FixedZone("+04:00", 4*60*60))

func newStubStorage() *stubStorage {
	return &stubStorage{assignID: 1, added: true}
}

func (s *stubStorage) CreateUser(_ context.Context, u *user.User) error {
	passed := *u
	s.gotUser = &passed

	if s.createErr != nil {
		return s.createErr
	}
	u.ID = s.assignID
	u.CreatedAt = stubTime
	return nil
}

func (s *stubStorage) AddColor(_ context.Context, userID user.ID, hex color.Hex) (bool, error) {
	s.gotUserID, s.gotHex = userID, hex
	return s.added, s.addErr
}

func (s *stubStorage) ListColors(_ context.Context, userID user.ID) ([]color.Color, error) {
	s.gotUserID = userID
	return s.colors, s.colorsErr
}

func (s *stubStorage) ListCommonColors(_ context.Context) ([]color.Common, error) {
	return s.common, s.commonErr
}
