package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/salandered/httputils/httputils"
	"github.com/salandered/wavelen/internal/auth"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
)

type contextKey string

const userIDContextKey = contextKey("user_id")

const (
	bearerScheme = "Bearer"
	bearerPrefix = bearerScheme + " "
)

const userIDPathValue = "user_id"

// See dev/auth.md

// Resolves the bearer token to a user id and puts it in the request context.
func authenticate(tokens storage.TokenRepo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
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

			id, err := tokens.UserIDForTokenHash(ctx, hash)
			if err != nil {
				// not writeStorageError: ErrTokenNotFound wraps ErrNotFound and would be a 404
				if errors.Is(err, storage.ErrTokenNotFound) {
					unauthorized(ctx, w)
					return
				}
				httputils.WriteError(ctx, w, err, http.StatusInternalServerError)
				return
			}

			// adding the hash too: logout can delete the row without parsing the header again
			// TODO: i don't like this, a cheap temporary logic
			ctx = auth.ContextWithTokenHash(context.WithValue(ctx, userIDContextKey, id), hash)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	}
}

// Refuses a token which belongs to someone other than the user inside the path.
func requireOwner(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		authenticated, ok := ctx.Value(userIDContextKey).(user.ID)
		if !ok {
			// authenticate did not run: a code bug
			slog.ErrorContext(ctx, "requireOwner reached with no authenticated user")
			httputils.WriteError(ctx, w,
				errors.New("internal server error"), http.StatusInternalServerError)
			return
		}

		// same 400 the handler would answer, but earlier
		requested, err := user.ParseID(req.PathValue(userIDPathValue))
		if err != nil {
			httputils.WriteError(ctx, w, err, http.StatusBadRequest)
			return
		}

		if authenticated != requested {
			httputils.WriteError(ctx, w,
				errors.New("not your user"), http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, req)
	})
}

func unauthorized(ctx context.Context, w http.ResponseWriter) {
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/WWW-Authenticate
	w.Header().Set("WWW-Authenticate", bearerScheme)
	httputils.WriteError(ctx, w, errors.New("invalid credentials"), http.StatusUnauthorized)
}
