package user

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

type ID int64

const (
	MinNameLen     = 1
	MaxNameLen     = 100
	MinNicknameLen = 3  // no single-character land grab
	MaxNicknameLen = 30 // ASCII only, so bytes and characters are the same count
)

var (
	ErrInvalidNickname = errors.New("invalid nickname")
	ErrInvalidName     = errors.New("invalid name")
)

type User struct {
	ID           ID
	Nickname     string // the login identifier, unique
	Name         string // free-form, for display
	PasswordHash []byte // bcrypt hash
	CreatedAt    time.Time
}

// NormalizeNickname trims and lowercases s.
// TODO: consider not lowercasing here, we store email as citext anyway.
func NormalizeNickname(s string) (string, error) {
	nick := strings.ToLower(strings.TrimSpace(s))
	if n := len(nick); n < MinNicknameLen || n > MaxNicknameLen {
		return "", fmt.Errorf(
			"%w: length must be in [%d, %d], got %d",
			ErrInvalidNickname, MinNicknameLen, MaxNicknameLen, n)
	}
	if !validNickname(nick) {
		return "", fmt.Errorf("%w: got %q", ErrInvalidNickname, s)
	}
	return nick, nil
}

// Lowercase ASCII letters, digits, '_' and '-'; the first and last character is alphanumeric.
func validNickname(nick string) bool {
	for i := 0; i < len(nick); i++ {
		c := nick[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		case (c == '_' || c == '-') && i > 0 && i < len(nick)-1:
		default:
			return false
		}
	}
	return true
}

// NormalizeName trims s.
func NormalizeName(s string) (string, error) {
	name := strings.TrimSpace(s)
	if n := utf8.RuneCountInString(name); n < MinNameLen || n > MaxNameLen {
		return "", fmt.Errorf(
			"%w: length must be in [%d, %d], got %d", ErrInvalidName, MinNameLen, MaxNameLen, n)
	}
	return name, nil
}
