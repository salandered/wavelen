package auth_test

import (
	"strings"
	"testing"

	"github.com/salandered/wavelen/internal/auth"
	"github.com/stretchr/testify/require"
)

const testPassword = "correct horse battery"

func TestPasswordMatchesTheHashItCameFrom(t *testing.T) {
	matches, err := auth.PasswordMatches(hashOf(t, testPassword), testPassword)

	require.NoError(t, err)
	require.True(t, matches)
}

func TestWrongPasswordIsNotAnError(t *testing.T) {
	matches, err := auth.PasswordMatches(hashOf(t, testPassword), "correct horse batterz")

	require.NoError(t, err)
	require.False(t, matches)
}

func TestUnusableHashNeverMatchesAndReportsWhy(t *testing.T) {
	hashes := map[string][]byte{
		"nil":            nil,
		"empty":          {},
		"not bcrypt":     []byte("not a bcrypt hash"),
		"truncated":      []byte("$2a$12$w197MSRbyWSMKybxo4ysO"),
		"unknown prefix": []byte("$9z$12$w197MSRbyWSMKybxo4ysO.QmD.JA3Ei0iDlMJE0ol0rQuHqyCmaRa"),
	}
	for name, hash := range hashes {
		t.Run(name, func(t *testing.T) {
			matches, err := auth.PasswordMatches(hash, testPassword)

			require.ErrorIs(t, err, auth.ErrUnusableHash)
			require.False(t, matches)
		})
	}
}

func TestHashPasswordRejectsLengthsOutsideTheBcryptRange(t *testing.T) {
	lengths := map[string]string{
		"empty":     "",
		"too short": strings.Repeat("a", auth.MinPasswordLen-1),
		"too long":  strings.Repeat("a", auth.MaxPasswordLen+1),
	}
	for name, password := range lengths {
		t.Run(name, func(t *testing.T) {
			_, err := auth.HashPassword(password)

			require.ErrorIs(t, err, auth.ErrInvalidPassword)
		})
	}
}

func TestSamePasswordHashesDifferentlyEveryTime(t *testing.T) {
	require.NotEqual(t, hashOf(t, testPassword), hashOf(t, testPassword))
}

// utils

func hashOf(t *testing.T, password string) []byte {
	t.Helper()

	hash, err := auth.HashPassword(password)
	require.NoError(t, err)
	return hash
}
