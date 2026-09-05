package user_test

import (
	"strings"
	"testing"

	"github.com/salandered/wavelen/internal/user"
	"github.com/stretchr/testify/require"
)

func TestNormalizeNicknameTrimsAndLowercases(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"olya", "olya"},
		{"  Olya  ", "olya"},
		{"OLYA_99", "olya_99"},
		{"a-b_c", "a-b_c"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := user.NormalizeNickname(tt.in)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeNicknameRejectsMalformedInput(t *testing.T) {
	tests := map[string]string{
		"empty":              "",
		"too short":          "ol",
		"too long":           strings.Repeat("a", user.MaxNicknameLen+1),
		"leading dash":       "-olya",
		"trailing dash":      "olya-",
		"leading score":      "_olya",
		"trailing score":     "olya_",
		"inner space":        "olya lovelace",
		"dot":                "olya.lovelace",
		"at sign":            "olya@example.com",
		"cyrillic lookalike": "оlya",
	}
	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := user.NormalizeNickname(in)
			require.ErrorContains(t, err, "invalid nickname")
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
			require.ErrorContains(t, err, "invalid name")
		})
	}
}

func TestNormalizeNameCountsRunesNotBytes(t *testing.T) {
	// N two-byte runes is 2N bytes
	got, err := user.NormalizeName(strings.Repeat("é", user.MaxNameLen))
	require.NoError(t, err)
	require.Len(t, []rune(got), user.MaxNameLen)
}
