package requestid

import (
	"context"
	"crypto/rand"
)

// Header carries the correlation id in and back out.
const Header = "X-Request-Id"

type contextKey struct{}

func New() string {
	return rand.Text()
}

func NewContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

// FromContext returns "" when the request did not pass the middleware.
func FromContext(ctx context.Context) string {
	id, _ := ctx.Value(contextKey{}).(string)
	return id
}
