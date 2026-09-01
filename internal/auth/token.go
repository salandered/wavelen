package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"time"

	"github.com/salandered/wavelen/internal/user"
)

// Plaintext is handed to the client at login, not stored.
// Hash is a hash of the Plaintext, stored.
type Token struct {
	Plaintext string
	Hash      []byte
	UserID    user.ID
	Expiry    time.Time
}

// NewToken creates a token valid for ttl from now.
func NewToken(userID user.ID, ttl time.Duration) *Token {
	// at least 128 bits of randomness, RFC 4648 base32
	plaintext := rand.Text()
	return &Token{
		Plaintext: plaintext,
		Hash:      HashToken(plaintext),
		UserID:    userID,
		Expiry:    time.Now().Add(ttl),
	}
}

// SHA-256
func HashToken(plaintext string) []byte {
	sum := sha256.Sum256([]byte(plaintext))
	return sum[:]
}
