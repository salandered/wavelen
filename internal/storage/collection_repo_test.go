//go:build integration

package storage_test

import (
	"github.com/salandered/wavelen/internal/storage"
)

func (s *StorageSuite) TestLockDefaultCollectionForAnUnknownUser() {
	_, err := s.storage.LockDefaultCollection(s.ctx(), 999)

	s.Require().ErrorIs(err, storage.ErrUserNotFound)
}
