package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/salandered/httputils/httputils"
)

// limitConcurrent caps at 'limit' how many requests can process 'next'.
// A 'limit' <= 0 disables the cap: just calling 'next'.
func limitConcurrent(limit int, wait time.Duration) func(http.HandlerFunc) http.Handler {
	if limit <= 0 {
		slog.Warn("concurrency cap disabled", "limit", limit)
		return func(next http.HandlerFunc) http.Handler { return next }
	}
	slog.Debug("concurrency cap enabled", "limit", limit, "wait", wait)

	sem := make(chan struct{}, limit)

	return func(next http.HandlerFunc) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, req *http.Request) {
				ctx := req.Context()

				if !acquire(ctx, sem, wait) {
					// ctx cancelled means the client hung up
					if ctx.Err() != nil {
						return
					}
					tooManyRequests(ctx, w)
					return
				}
				defer func() { <-sem }()

				next(w, req)
			},
		)
	}
}

// acquire tries to take a semaphore slot
func acquire(ctx context.Context, sem chan struct{}, wait time.Duration) bool {
	// don't wait if the slot is available
	select {
	case sem <- struct{}{}:
		return true
	default:
	}

	// wait for 'wait' if there are no slots
	waitCtx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()

	select {
	case sem <- struct{}{}:
		return true
	// waitCtx timeout or a parent ctx is cancelled
	case <-waitCtx.Done():
		return false
	}
}

// approximate value, in theory should be derived from the [limitConcurrent]'s 'wait'.
const retryAfterBusy = "2"

func tooManyRequests(ctx context.Context, w http.ResponseWriter) {
	w.Header().Set("Retry-After", retryAfterBusy)
	httputils.WriteError(ctx, w, errors.New("server is busy, retry later"), http.StatusTooManyRequests)
}
