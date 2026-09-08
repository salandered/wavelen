package server

import (
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/salandered/wavelen/internal/requestid"
)

// A wrapper that embeds the [http.ResponseWriter] and some of its methods.
// Captures the response status and size for logging.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

/*
limitation: no 1xx checks, they would be recorded as if they were final.
net/http does not commit the response on 1xx (except 101), so the real status
that follows is lost, and recoveryMiddleware sees a header that was not written.
See 'response.WriteHeader' in GOROOT/src/net/http/server.go
*/
func (r *statusRecorder) WriteHeader(code int) {
	// only the first write matters, net/http WriteHeader returns on a second one
	if !r.wroteHeader {
		r.status = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK) // mirror net/http: first Write commits a 200
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// [http.ResponseController] walks the wrappers using 'Unwrap' to reach the real writer
// (e.g for Flush or SetWriteDeadline). Embedding does not promote those.
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

/*
Creates server-generated correlation id.
Adds it into the request context, from where the logging handler picks it up:
every log call taking a ctx below this middleware carries the id, and no log site should add it by hand.
Adds this id to the X-Request-ID response header.
The request X-Request-ID header is ignored.
*/
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		id := requestid.New()
		w.Header().Set(requestid.Header, id)
		next.ServeHTTP(w, req.WithContext(requestid.NewContext(req.Context(), id)))
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK} // default 200

		next.ServeHTTP(rec, req)

		level := slog.LevelInfo
		switch {
		case rec.status >= 500:
			level = slog.LevelError
		case rec.status >= 400:
			level = slog.LevelWarn
		}
		attrs := []slog.Attr{
			slog.String("method", req.Method),
			slog.String("path", req.URL.Path),
			slog.String("remote_addr", remoteHost(req)),
			slog.Int("status", rec.status),
			slog.Duration("duration", time.Since(start)),
			slog.Int("bytes", rec.bytes),
		}
		// empty in case of no proxy
		if fwd := forwardedFor(req); fwd != "" {
			attrs = append(attrs, slog.String("forwarded_for", fwd))
		}
		// If the client hung up, or the request deadline passed.
		// Without this we will log the default 200
		if err := req.Context().Err(); err != nil {
			attrs = append(attrs, slog.String("aborted", err.Error()))
		}
		slog.LogAttrs(req.Context(), level, "request", attrs...)
	})
}

// remoteHost is the peer of this connection.
// Behind the proxy that would be it, not a client
func remoteHost(req *http.Request) string {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr // log what arrived
	}
	return host
}

/*
forwardedFor is the rightmost X-Forwarded-For entry - what the nearest proxy observed on
its own connection. The last entry of the last line.

From the https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/X-Forwarded-For

	"When a client connects directly to a server, the client's IP address is sent
	to the server and is often written to server access logs.
	If a client connection passes through any forward or reverse proxies,
	the server only sees the final proxy's IP address, which is often of little use"

	"Trusted proxy count
	The count of reverse proxies between the internet and the server is configured.
	The X-Forwarded-For IP list is searched from the rightmost by that count minus one.
	For example, if there is only one reverse proxy, that proxy will add the client's IP address,
	so the rightmost address should be used.
	If there are three reverse proxies, the last two IP addresses will be internal."
*/
func forwardedFor(req *http.Request) string {
	lines := req.Header.Values("X-Forwarded-For")
	if len(lines) == 0 {
		return ""
	}
	last := lines[len(lines)-1]
	if i := strings.LastIndex(last, ","); i >= 0 {
		last = last[i+1:]
	}
	return strings.TrimSpace(last)
}

// Catches a panic from the downstream handler and turns it into a logged 500.
// Should be inside loggingMiddleware so the 500 it produces gets logged.
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if v == http.ErrAbortHandler { //nolint:errorlint // panic value, not a wrapped err
				panic(v) // sentinel, we repanic
			}
			slog.LogAttrs(req.Context(), slog.LevelError, "panic recovered",
				slog.Any("panic", v),
				slog.String("stack", string(debug.Stack())),
			)
			// don't write the header if the handler already started doing it
			rec, ok := w.(*statusRecorder)
			if !ok || !rec.wroteHeader {
				w.Header().Set("Connection", "close")
				w.WriteHeader(http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, req)
	})
}
