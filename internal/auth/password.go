package auth

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const (
	MinPasswordLen = 8
	// max that bcrypt supports
	MaxPasswordLen = 72
)

// Note: Recommended not to change this.
// Technically it is ok - stored hashes have the cost included.
// But the time cost will be different for old and new passwords. Also the dummyHash should be in sync.
const bcryptCost = 12

var ErrInvalidPassword = errors.New("invalid password")
var ErrUnusableHash = errors.New("unusable password hash")

// Uses bcrypt.
func HashPassword(plaintext string) ([]byte, error) {
	if n := len(plaintext); n < MinPasswordLen || n > MaxPasswordLen {
		return nil, fmt.Errorf("%w: must be %d to %d bytes, got %d",
			ErrInvalidPassword, MinPasswordLen, MaxPasswordLen, n)
	}

	// format is: $2a$[cost]$[22-character salt][31-character hash]
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("auth hash password: %w", err)
	}
	return hash, nil
}

// bcrypt.ErrMismatchedHashAndPassword -> returns false and nil.
// Any other error -> false and the error.
func PasswordMatches(hash []byte, plaintext string) (bool, error) {
	// bcrypt compares only the first 72 bytes.
	// A longer string would authenticate on its first 72.
	if len(plaintext) > MaxPasswordLen {
		EqualizeTiming()
		return false, nil
	}

	err := bcrypt.CompareHashAndPassword(hash, []byte(plaintext))
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
		return false, nil
	default:
		return false, fmt.Errorf("%w: %w", ErrUnusableHash, err)
	}
}

// A bcrypt dummy hash
const dummyHash = "$2a$12$w197MSRbyWSMKybxo4ysO.QmD.JA3Ei0iDlMJE0ol0rQuHqyCmaRa"

// EqualizeTiming runs an unsuccessful bcrypt match.
// Use case: client code wants to simulate the same response time as if the password was checked.
func EqualizeTiming() {
	_ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte("no match"))
}
