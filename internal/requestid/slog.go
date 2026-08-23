package requestid

import (
	"context"
	"log/slog"
)

// LogAttrs is the slogenv AttrsFunc for this package: every *Context log call below
// requestIDMiddleware carries the id, so no call site adds it by hand.
func LogAttrs(ctx context.Context) []slog.Attr {
	if id := FromContext(ctx); id != "" {
		return []slog.Attr{slog.String("request_id", id)}
	}
	return nil
}
