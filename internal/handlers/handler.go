package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/salandered/httputils/httputils"
	"github.com/salandered/wavelen/internal/authsvc"
	"github.com/salandered/wavelen/internal/collection"
	"github.com/salandered/wavelen/internal/collectionsvc"
	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/colorsvc"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/version"
)

const (
	hexPathValue          = "hex"
	harmonyPathValue      = "harmony"
	collectionIDPathValue = "id"
)

const (
	limitQuery  = "limit"
	sortQuery   = "sort"
	orderQuery  = "order"
	cursorQuery = "cursor"
)

// Nothing large in bodies.
const maxRequestBodyBytes = 1 << 16 // 64 kb

const redactedValue = "[redacted]"

type VersionResp struct {
	Version string `json:"version"`
}

// HandleVersion reports the build-time version.
func HandleVersion(w http.ResponseWriter, req *http.Request) {
	httputils.WriteJSON(req.Context(), w, http.StatusOK, VersionResp{Version: version.Get()})
}

func collectionIDFromPath(req *http.Request) (collection.ID, error) {
	return collection.ParseID(req.PathValue(collectionIDPathValue))
}

func hexFromPath(req *http.Request) (color.Hex, error) {
	raw := req.PathValue(hexPathValue)
	if strings.Contains(raw, "#") {
		return "", fmt.Errorf("invalid hex color %q: must be 6 hex digits without '#'", raw)
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
	case errors.Is(err, storage.ErrDuplicateNickname):
		httputils.WriteError(ctx, w, errors.New("nickname already taken"), http.StatusConflict)
	case errors.Is(err, authsvc.ErrInvalidCredentials):
		httputils.WriteError(ctx, w, errors.New("invalid credentials"), http.StatusUnauthorized)
	case errors.Is(err, colorsvc.ErrQuotaFull):
		httputils.WriteError(ctx, w, errors.New("color quota full"), http.StatusConflict)
	case errors.Is(err, collectionsvc.ErrQuotaFull):
		httputils.WriteError(ctx, w, errors.New("collection quota full"), http.StatusConflict)
	case errors.Is(err, collectionsvc.ErrDeleteDefault):
		httputils.WriteError(ctx, w,
			errors.New("default collection cannot be deleted"), http.StatusConflict)
	default:
		httputils.WriteError(ctx, w, err, http.StatusInternalServerError)
	}
}
