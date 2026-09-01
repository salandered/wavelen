package auth_test

import (
	"testing"
	"time"

	"github.com/salandered/wavelen/internal/auth"
	"github.com/salandered/wavelen/internal/user"
	"github.com/stretchr/testify/require"
)

const (
	testToken      = "X3ASTT2CDAN66BACKSCI4SU7SA"
	testTokenOneUp = "X3ASTT2CDAN66BACKSCI4SU7SB"
)

func TestHashTokenIsDeterministic(t *testing.T) {
	require.Equal(t, auth.HashToken(testToken), auth.HashToken(testToken))
}

func TestHashTokenSeparatesTokensThatDifferByOneCharacter(t *testing.T) {
	require.NotEqual(t, auth.HashToken(testToken), auth.HashToken(testTokenOneUp))
}

func TestNewToken(t *testing.T) {
	token := auth.NewToken(user.ID(42), time.Hour) // with one hour ttl

	require.Equal(t, auth.HashToken(token.Plaintext), token.Hash)
	require.Equal(t, user.ID(42), token.UserID)
	require.WithinDuration(t, time.Now().Add(time.Hour), token.Expiry, time.Minute)
}

func TestNewTokenHashesDifferentPlaintextEveryTime(t *testing.T) {
	first := auth.NewToken(user.ID(42), time.Hour)
	second := auth.NewToken(user.ID(42), time.Hour)

	require.NotEqual(t, first.Plaintext, second.Plaintext)
	require.NotEqual(t, first.Hash, second.Hash)
}

func TestNegativeTTLCreatesExpiredToken(t *testing.T) {
	token := auth.NewToken(user.ID(42), -time.Minute)

	require.True(t, token.Expiry.Before(time.Now()))
}
