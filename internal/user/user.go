package user

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type ID int64

const (
	MinNameLen  = 1
	MaxNameLen  = 100
	MaxEmailLen = 254 // 254 comes from SMTP
)

var (
	ErrInvalidID    = errors.New("invalid user id")
	ErrInvalidEmail = errors.New("invalid email")
	ErrInvalidName  = errors.New("invalid name")
)

type User struct {
	ID           ID
	Email        string
	Name         string
	PasswordHash []byte // bcrypt hash
	CreatedAt    time.Time
}

func ParseID(s string) (ID, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%w: want a positive integer, got %q", ErrInvalidID, s)
	}
	return ID(n), nil
}

// NormalizeEmail trims and lowercases s.
// TODO: consider not lowercasing here, we store email as citext anyway.
func NormalizeEmail(s string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(s))
	if len(email) > MaxEmailLen {
		return "", fmt.Errorf("%w: must be at most %d bytes", ErrInvalidEmail, MaxEmailLen)
	}
	if !validEmail(email) {
		return "", fmt.Errorf("%w: got %q", ErrInvalidEmail, s)
	}
	return email, nil
}

// Validates '@'; non-empty local part; a dotted domain
func validEmail(email string) bool {
	local, domain, found := strings.Cut(email, "@")
	switch {
	case !found, local == "", domain == "":
		return false
	case strings.Contains(domain, "@"):
		return false
	case strings.HasPrefix(domain, "."), strings.HasSuffix(domain, "."):
		return false
	}
	return strings.Contains(domain, ".")
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
