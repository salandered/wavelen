package handlers

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/storage"
)

// A cursor is base64url("<sort>|<order>|<key...>"), unpadded.
// Carries the sort it was created under,
// so changing sort mid-listing is a 400, not a wrong page.
const (
	// consider adding version like cursorVersion = "v1"
	cursorSep         = "|"
	cursorHeaderParts = 2 // sort, order
)

var errInvalidCursor = errors.New("invalid cursor")

// Where cursor actually points at
func cursorKey(sort storage.ColorSort, last color.Color) []string {
	switch sort {
	case storage.SortByCreatedAt:
		// formatted from what Postgres returned, tied by Hex
		return []string{last.CreatedAt.Format(time.RFC3339Nano), string(last.Hex)}
	case storage.SortByHex, storage.SortByColor:
		// color_key is derived from the hex, so the hex alone locates the boundary
		return []string{string(last.Hex)}
	}
	return nil
}

// Unpadded base64 encoding.
// last is the final row of the page, a Color from Postgres.
func encodeCursor(sort storage.ColorSort, order storage.SortOrder, last color.Color) string {
	parts := append(
		[]string{string(sort), string(order)},
		cursorKey(sort, last)..., // nil is unreachable: the caller validated sort
	)
	return base64.RawURLEncoding.EncodeToString([]byte(strings.Join(parts, cursorSep)))
}

// Untrusted input: anything bad is errInvalidCursor.
// sort and order are the ones the request asks for.
func decodeCursor(
	token string, sort storage.ColorSort, order storage.SortOrder,
) (*storage.ColorCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("%w: not unpadded base64url", errInvalidCursor)
	}

	parts := strings.Split(string(raw), cursorSep)
	if len(parts) <= cursorHeaderParts {
		return nil, fmt.Errorf("%w: want more than %d fields, got %d",
			errInvalidCursor, cursorHeaderParts, len(parts))
	}
	gotSort, gotOrder := storage.ColorSort(parts[0]), storage.SortOrder(parts[1])

	if gotSort != sort || gotOrder != order {
		return nil, fmt.Errorf("%w: cursor was created for %s %s, the request asks for %s %s",
			errInvalidCursor, gotSort, gotOrder, sort, order)
	}

	key := parts[cursorHeaderParts:]
	var cursor storage.ColorCursor
	switch sort {
	case storage.SortByCreatedAt:
		if len(key) != 2 {
			return nil, fmt.Errorf("%w: want a timestamp and a hex, got %d values",
				errInvalidCursor, len(key))
		}
		createdAt, err := time.Parse(time.RFC3339Nano, key[0])
		if err != nil {
			return nil, fmt.Errorf("%w: %q is not an RFC3339 timestamp", errInvalidCursor, key[0])
		}
		hex, err := color.ParseHex(key[1])
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errInvalidCursor, err)
		}
		cursor = storage.ColorCursor{CreatedAt: createdAt, Hex: hex}
	case storage.SortByHex, storage.SortByColor:
		if len(key) != 1 {
			return nil, fmt.Errorf("%w: want one hex, got %d values", errInvalidCursor, len(key))
		}
		hex, err := color.ParseHex(key[0])
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errInvalidCursor, err)
		}
		cursor.Hex = hex
	default:
		return nil, fmt.Errorf("%w: unknown sort %q", errInvalidCursor, sort)
	}
	return &cursor, nil
}
