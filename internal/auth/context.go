package auth

import "context"

type contextKey string

const tokenHashContextKey = contextKey("token_hash")

// The hash, not the plaintext
func ContextWithTokenHash(ctx context.Context, hash []byte) context.Context {
	return context.WithValue(ctx, tokenHashContextKey, hash)
}

// False if the request did not go through the auth wrapper.
func TokenHashFromContext(ctx context.Context) ([]byte, bool) {
	hash, ok := ctx.Value(tokenHashContextKey).([]byte)
	return hash, ok
}
