package server

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/salandered/wavelen/internal/requestid"
)

// Reuses an inbound header so a correlation id survives across hops.
// Consider doing the Apex approach
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		id := req.Header.Get(requestid.Header)
		if id == "" {
			id = requestid.New()
		}
		w.Header().Set(requestid.Header, id)
		next.ServeHTTP(w, req.WithContext(requestid.NewContext(req.Context(), id)))
	})
}

// Records the status code for the access log. Optional ResponseWriter interfaces
// (Flush, Hijack) are not forwarded, this API has no use for them.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, req)

		slog.InfoContext(req.Context(), "request",
			"request_id", requestid.FromContext(req.Context()),
			"method", req.Method,
			"path", req.URL.Path,
			"status", rec.status,
			"duration", time.Since(start),
		)
	})
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			p := recover()
			if p == nil {
				return
			}
			slog.ErrorContext(req.Context(), "panic in handler",
				"request_id", requestid.FromContext(req.Context()),
				"panic", p,
				"stack", string(debug.Stack()),
			)
			w.Header().Set("Connection", "close")
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}()
		next.ServeHTTP(w, req)
	})
}
