package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/salandered/httputils/httputils"
	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/colorsvc"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
	"github.com/salandered/wavelen/internal/version"
)

const (
	userIDPathValue = "user_id"
	hexPathValue    = "hex"
)

const (
	limitQuery  = "limit"
	sortQuery   = "sort"
	orderQuery  = "order"
	cursorQuery = "cursor"
)

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

func hexFromPath(req *http.Request) (color.Hex, error) {
	raw := req.PathValue(hexPathValue)
	if strings.Contains(raw, "#") {
		return "", fmt.Errorf(
			"%w: want 6 hex digits without '#', got %q",
			color.ErrInvalidHex, raw,
		)
	}
	return color.ParseHex(raw)
}

// Response cursor metadata.
// An absent next_cursor means the end of the list (client stops)
type cursorMeta struct {
	Limit      int    `json:"limit"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// maps a decode or validation error to an HTTP response
func writeRequestError(ctx context.Context, w http.ResponseWriter, err error) {
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		httputils.WriteError(ctx, w,
			errors.New("request body too large"), http.StatusRequestEntityTooLarge)
		return
	}
	httputils.WriteError(ctx, w, err, http.StatusBadRequest)
}

// maps a storage or service error to an HTTP response
func writeStorageError(ctx context.Context, w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, storage.ErrUserNotFound):
		httputils.WriteError(ctx, w, errors.New("user not found"), http.StatusNotFound)
	case errors.Is(err, storage.ErrNotFound):
		httputils.WriteError(ctx, w, errors.New("not found"), http.StatusNotFound)
	case errors.Is(err, storage.ErrDuplicateEmail):
		httputils.WriteError(ctx, w, errors.New("email already registered"), http.StatusConflict)
	case errors.Is(err, colorsvc.ErrQuotaFull):
		httputils.WriteError(ctx, w, errors.New("color quota full"), http.StatusConflict)
	default:
		httputils.WriteError(ctx, w, err, http.StatusInternalServerError)
	}
}
