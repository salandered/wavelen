package requestid

import (
	"context"
	"crypto/rand"
)

// Header is where the id is echoed back. An inbound one is ignored, see requestIDMiddleware.
const Header = "X-Request-Id"

type contextKey struct{}

func New() string {
	// TODO: consider using uuid with go 1.27
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
