package authsvc

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/salandered/wavelen/internal/auth"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
)

// Answers the same for an unknown address or a wrong password
var ErrInvalidCredentials = errors.New("invalid credentials")

type AuthSvc struct {
	storage storage.Storage
	ttl     time.Duration
}

func New(store storage.Storage, ttl time.Duration) *AuthSvc {
	return &AuthSvc{storage: store, ttl: ttl}
}

// Login creates and stores a hashed token.
func (a *AuthSvc) Login(ctx context.Context, email, password string) (*auth.Token, error) {
	// login takes the address as is (unlike signup): an invalid one means a failed login, not 400.
	// => the endpoint has only one failure answer
	normalized, err := user.NormalizeEmail(email)
	if err != nil {
		return nil, a.refuse()
	}

	u, err := a.storage.UserByEmail(ctx, normalized)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			return nil, a.refuse()
		}
		return nil, err
	}

	matches, err := auth.PasswordMatches(u.PasswordHash, password)
	if err != nil {
		// the stored bytes are not a bcrypt hash. A data bug.
		// treating as a refusal currently. panic?
		slog.ErrorContext(ctx, "unusable password hash", "user_id", u.ID, "error", err)
		return nil, ErrInvalidCredentials
	}
	if !matches {
		return nil, ErrInvalidCredentials
	}

	token := auth.NewToken(u.ID, a.ttl)
	if err := a.storage.InsertToken(ctx, token); err != nil {
		return nil, err
	}
	return token, nil
}

func (a *AuthSvc) refuse() error {
	auth.EqualizeTiming()
	return ErrInvalidCredentials
}
