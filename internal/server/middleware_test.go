package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/salandered/wavelen/internal/requestid"
	"github.com/stretchr/testify/require"

	logging "github.com/salandered/slogenv"
)

func TestRecoveryMiddlewarePanicBeforeWriteReturns500(t *testing.T) {
	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})
	rec := httptest.NewRecorder()

	loggingMiddleware(recoveryMiddleware(h)).ServeHTTP(rec, genericRequest())

	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestRecoveryMiddlewarePanicAfterPartialWriteKeepsOriginalStatus(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"partial":`)
		panic("boom")
	})
	rec := httptest.NewRecorder()

	loggingMiddleware(recoveryMiddleware(h)).ServeHTTP(rec, genericRequest())

	// headers already flushed -> recovery must NOT rewrite the status to 500
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, `{"partial":`, rec.Body.String())
}

func TestRecoveryMiddlewareOnErrAbortHandlerRepanics(t *testing.T) {
	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	})
	rec := httptest.NewRecorder()

	defer func() {
		require.Equal(t, http.ErrAbortHandler, recover()) //nolint:errorlint
	}()

	recoveryMiddleware(h).ServeHTTP(rec, genericRequest())

	t.Fatal("expected re-panic, did not happen")
}

func TestLoggingMiddlewareRecordsStatusAndBytes(t *testing.T) {
	logged := captureLogs(t, func() {
		h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTeapot)
			_, _ = io.WriteString(w, "abcde")
		})
		loggingMiddleware(h).ServeHTTP(httptest.NewRecorder(), genericRequest())
	})

	require.Len(t, logged, 1)
	require.Equal(t, "request", logged[0]["msg"])
	require.EqualValues(t, http.StatusTeapot, logged[0]["status"])
	require.EqualValues(t, 5, logged[0]["bytes"])
}

func TestLoggingMiddlewareLevelFollowsStatus(t *testing.T) {
	for status, want := range map[int]string{
		http.StatusOK:                  "INFO",
		http.StatusBadRequest:          "WARN",
		http.StatusInternalServerError: "ERROR",
	} {
		logged := captureLogs(t, func() {
			h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
			})
			loggingMiddleware(h).ServeHTTP(httptest.NewRecorder(), genericRequest())
		})

		require.Len(t, logged, 1)
		require.Equal(t, want, logged[0]["level"], "status %d", status)
	}
}

// The whole point of wiring requestid.LogAttrs into the handler: a handler that logs with
// the request ctx gets the id without naming it.
func TestRequestIDReachesLogRecordsBelowTheMiddleware(t *testing.T) {
	var handlerSawID string

	logged := captureLogs(t, func() {
		h := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			handlerSawID = requestid.FromContext(req.Context())
			slog.InfoContext(req.Context(), "in handler")
			w.WriteHeader(http.StatusOK)
		})
		requestIDMiddleware(loggingMiddleware(h)).ServeHTTP(httptest.NewRecorder(), genericRequest())
	})

	require.NotEmpty(t, handlerSawID)
	require.Len(t, logged, 2) // the handler's own record, then the access log
	for _, record := range logged {
		require.Equal(t, handlerSawID, record["request_id"], "record %q", record["msg"])
	}
}

func TestLogRecordsCarryNoRequestIDWithoutTheMiddleware(t *testing.T) {
	logged := captureLogs(t, func() {
		h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		loggingMiddleware(h).ServeHTTP(httptest.NewRecorder(), genericRequest())
	})

	require.Len(t, logged, 1)
	require.NotContains(t, logged[0], "request_id")
}

// Installs a JSON logger over the same context handler main uses, and returns the records
// fn produced. The default logger is restored on cleanup.
func captureLogs(t *testing.T, fn func()) []map[string]any {
	t.Helper()

	var buf bytes.Buffer
	h := logging.NewContextHandler(
		slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}),
		requestid.LogAttrs,
	)
	restore := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(restore) })

	fn()

	var out []map[string]any
	for line := range bytes.Lines(bytes.TrimSpace(buf.Bytes())) {
		var record map[string]any
		require.NoError(t, json.Unmarshal(line, &record))
		out = append(out, record)
	}
	return out
}

func genericRequest() *http.Request {
	return httptest.NewRequest(http.MethodGet, "/x", nil)
}
