package server

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/salandered/httputils/httputils"
	"github.com/salandered/wavelen/internal/auth"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
)

const (
	bearerScheme = "Bearer"
	bearerPrefix = bearerScheme + " "
)

// See docs/auth.md

// A handler on an authenticated route.
// The user id is an argument - the wrapper [authenticate] resolves it and calls [authedHandlerFunc]
type authedHandlerFunc func(w http.ResponseWriter, req *http.Request, userID user.ID)

// Resolves the bearer token to a user id and hands it to next.
func authenticate(tokenRepo storage.TokenRepo) func(authedHandlerFunc) http.Handler {
	return func(next authedHandlerFunc) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// indicates to any caches that the response may vary based on the value
			// of the Authorization header in the request.
			w.Header().Add("Vary", "Authorization")

			ctx := req.Context()

			plaintext, ok := strings.CutPrefix(req.Header.Get("Authorization"), bearerPrefix)
			if !ok || plaintext == "" {
				unauthorized(ctx, w)
				return
			}

			hash := auth.HashToken(plaintext)

			user_id, err := tokenRepo.UserIDForTokenHash(ctx, hash)
			if err != nil {
				// not writeStorageError: ErrTokenNotFound wraps ErrNotFound and would be a 404
				if errors.Is(err, storage.ErrTokenNotFound) {
					unauthorized(ctx, w)
					return
				}
				httputils.WriteError(ctx, w, err, http.StatusInternalServerError)
				return
			}

			// the hash is in the context, logout reads it
			ctx = auth.ContextWithTokenHash(ctx, hash)
			next(w, req.WithContext(ctx), user_id)
		})
	}
}

func unauthorized(ctx context.Context, w http.ResponseWriter) {
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/WWW-Authenticate
	w.Header().Set("WWW-Authenticate", bearerScheme)
	httputils.WriteError(ctx, w, errors.New("invalid credentials"), http.StatusUnauthorized)
}
