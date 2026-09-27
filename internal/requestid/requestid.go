package requestid

import (
	"context"
	"uuid"
)

// Header is where the id is echoed back. An inbound one is ignored, see requestIDMiddleware.
const Header = "X-Request-Id"

type contextKey struct{}

// New returns a uuidv7.
func New() string {
	return uuid.NewV7().String()
}

func NewContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

// FromContext returns "" when the request did not pass the middleware.
func FromContext(ctx context.Context) string {
	id, _ := ctx.Value(contextKey{}).(string)
	return id
}
