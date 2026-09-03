package user_test

import (
	"strings"
	"testing"

	"github.com/salandered/wavelen/internal/user"
	"github.com/stretchr/testify/require"
)

func TestNormalizeEmailTrimsAndLowercases(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"user@example.com", "user@example.com"},
		{"  User@Example.COM  ", "user@example.com"},
		{"a.b+tag@sub.example.co.uk", "a.b+tag@sub.example.co.uk"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := user.NormalizeEmail(tt.in)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeEmailRejectsMalformedInput(t *testing.T) {
	tests := map[string]string{
		"empty":         "",
		"no at":         "userexample.com",
		"no local part": "@example.com",
		"no domain":     "user@",
		"two ats":       "user@ex@ample.com",
		"dotless":       "user@example",
		"leading dot":   "user@.example.com",
		"trailing dot":  "user@example.com.",
		"too long":      strings.Repeat("a", 250) + "@example.com",
	}
	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := user.NormalizeEmail(in)
			require.ErrorIs(t, err, user.ErrInvalidEmail)
		})
	}
}

func TestNormalizeNameTrimsSurroundingWhitespace(t *testing.T) {
	got, err := user.NormalizeName("  Olya Lovelace \n")
	require.NoError(t, err)
	require.Equal(t, "Olya Lovelace", got)
}

func TestNormalizeNameRejectsEmptyAndOverlong(t *testing.T) {
	for name, in := range map[string]string{
		"empty":          "",
		"whitespace":     "   ",
		"over max runes": strings.Repeat("a", user.MaxNameLen+1),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := user.NormalizeName(in)
			require.ErrorIs(t, err, user.ErrInvalidName)
		})
	}
}

func TestNormalizeNameCountsRunesNotBytes(t *testing.T) {
	// 100 two-byte runes is 200 bytes but exactly MaxNameLen runes
	got, err := user.NormalizeName(strings.Repeat("é", user.MaxNameLen))
	require.NoError(t, err)
	require.Len(t, []rune(got), user.MaxNameLen)
}
