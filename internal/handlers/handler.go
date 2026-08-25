package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
	"github.com/salandered/wavelen/internal/version"
)

const userIDPathValue = "user_id"

// Nothing large in bodies.
const maxRequestBodyBytes = 1 << 16 // 64 kb

func HandleRoot(w http.ResponseWriter, req *http.Request) {
	if _, err := fmt.Fprintf(w, "wavelen version %v\n", version.Get()); err != nil {
		slog.ErrorContext(req.Context(), "failed writing root response", "error", err)
	}
}

func userIDFromPath(req *http.Request) (user.ID, error) {
	return user.ParseID(req.PathValue(userIDPathValue))
}

// Decodes the incoming req.
// Checks: only one JSON object; bounded; no unknown fields.
func readJSON(w http.ResponseWriter, req *http.Request, dst any) error {
	req.Body = http.MaxBytesReader(w, req.Body, maxRequestBodyBytes)
	dec := json.NewDecoder(req.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return err
	}

	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("body must contain a single JSON object")
	}

	slog.DebugContext(req.Context(), "request decoded", "body", dst)
	return nil
}

func writeJSON(ctx context.Context, w http.ResponseWriter, statusCode int, data any) {
	rawJSON, err := json.Marshal(data)
	if err != nil {
		writeError(ctx, w, fmt.Errorf("marshalling response body: %w", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode) // before Write

	if _, err := w.Write(rawJSON); err != nil {
		// the status line already went out, nothing left to do but record it
		slog.ErrorContext(ctx, "failed writing response body", "status", statusCode, "error", err)
	}
	slog.DebugContext(ctx, "response sent",
		"bytes", len(rawJSON), "payload", truncatePayload(rawJSON))
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeError(ctx context.Context, w http.ResponseWriter, err error, statusCode int) {
	msg := err.Error()
	if statusCode >= http.StatusInternalServerError {
		slog.ErrorContext(ctx, "request failed", "status", statusCode, "error", err)
		msg = "internal server error" // the client should not see the actual error
	} else {
		slog.WarnContext(ctx, "request rejected", "status", statusCode, "error", err)
	}

	rawJSON, marshalErr := json.Marshal(errorResponse{Error: msg})
	if marshalErr != nil {
		slog.ErrorContext(ctx, "failed marshalling error response", "error", marshalErr)
		// all bad, just return a plain text
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if _, writeErr := w.Write(rawJSON); writeErr != nil {
		slog.ErrorContext(ctx, "failed writing error response",
			"status", statusCode, "error", writeErr)
	}
}

// maps a decode or validation error to an HTTP response
func writeRequestError(ctx context.Context, w http.ResponseWriter, err error) {
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		writeError(ctx, w, errors.New("request body too large"), http.StatusRequestEntityTooLarge)
		return
	}
	writeError(ctx, w, err, http.StatusBadRequest)
}

// maps a storage-layer error to an HTTP response
func writeStorageError(ctx context.Context, w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, storage.ErrUserNotFound):
		writeError(ctx, w, errors.New("user not found"), http.StatusNotFound)
	case errors.Is(err, storage.ErrNotFound):
		writeError(ctx, w, errors.New("not found"), http.StatusNotFound)
	case errors.Is(err, storage.ErrDuplicateEmail):
		writeError(ctx, w, errors.New("email already registered"), http.StatusConflict)
	default:
		writeError(ctx, w, err, http.StatusInternalServerError)
	}
}

const maxLoggedPayload = 512

func truncatePayload(rawJSON []byte) string {
	if len(rawJSON) <= maxLoggedPayload {
		return string(rawJSON)
	}
	// json.Marshal emits raw UTF-8, so just a cut might split a rune
	return strings.ToValidUTF8(string(rawJSON[:maxLoggedPayload]), "") + "..."
}
