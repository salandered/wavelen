package handlers

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/stretchr/testify/require"
)

// microseconds, what timestamptz stores
var lastRow = color.Color{
	Hex:       "#ff00aa",
	CreatedAt: time.Date(2026, 8, 23, 14, 0, 0, 825023000, time.UTC),
}

func TestCursorRoundTripBySortedColumn(t *testing.T) {
	cases := []struct {
		sort  storage.ColorSort
		order storage.SortOrder
		want  storage.ColorCursor
	}{
		{
			storage.SortByCreatedAt,
			storage.OrderDesc,
			storage.ColorCursor{CreatedAt: lastRow.CreatedAt, Hex: lastRow.Hex},
		},
		{
			storage.SortByCreatedAt,
			storage.OrderAsc,
			storage.ColorCursor{CreatedAt: lastRow.CreatedAt, Hex: lastRow.Hex},
		},
		{storage.SortByHex, storage.OrderAsc, storage.ColorCursor{Hex: lastRow.Hex}},
		{storage.SortByHex, storage.OrderDesc, storage.ColorCursor{Hex: lastRow.Hex}},
		{storage.SortByColor, storage.OrderAsc, storage.ColorCursor{Hex: lastRow.Hex}},
		{storage.SortByColor, storage.OrderDesc, storage.ColorCursor{Hex: lastRow.Hex}},
	}

	for _, c := range cases {
		t.Run(string(c.sort)+" "+string(c.order), func(t *testing.T) {
			got, err := decodeCursor(encodeCursor(c.sort, c.order, lastRow), c.sort, c.order)

			require.NoError(t, err)
			require.True(t, c.want.CreatedAt.Equal(got.CreatedAt))
			require.Equal(t, c.want.Hex, got.Hex)
		})
	}
}

func TestDecodeCursorRejectsMalformedTokens(t *testing.T) {
	encode := func(raw string) string {
		return base64.RawURLEncoding.EncodeToString([]byte(raw))
	}

	cases := map[string]string{
		"not base64":       "not base64!",
		"padded base64":    base64.URLEncoding.EncodeToString([]byte("created_at|desc|x|y")),
		"header only":      encode("created_at|desc"),
		"sort mismatch":    encode("hex|desc|#ff00aa"),
		"order mismatch":   encode("created_at|asc|2026-08-23T14:00:00Z|#ff00aa"),
		"key too short":    encode("created_at|desc|2026-08-23T14:00:00Z"),
		"key too long":     encode("created_at|desc|2026-08-23T14:00:00Z|#ff00aa|extra"),
		"bad timestamp":    encode("created_at|desc|yesterday|#ff00aa"),
		"bad tiebreak hex": encode("created_at|desc|2026-08-23T14:00:00Z|not-a-hex"),
	}

	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := decodeCursor(token, storage.SortByCreatedAt, storage.OrderDesc)

			require.ErrorIs(t, err, errInvalidCursor)
			require.Nil(t, got)
		})
	}
}

func TestDecodeCursorRejectsAHexTokenWithAnUnparseableHex(t *testing.T) {
	token := base64.RawURLEncoding.EncodeToString([]byte("hex|asc|#zzzzzz"))

	got, err := decodeCursor(token, storage.SortByHex, storage.OrderAsc)

	require.ErrorIs(t, err, errInvalidCursor)
	require.Nil(t, got)
}

func TestDecodeCursorRejectsAColorTokenUnderTheHexSort(t *testing.T) {
	token := encodeCursor(storage.SortByColor, storage.OrderAsc, lastRow)

	got, err := decodeCursor(token, storage.SortByHex, storage.OrderAsc)

	require.ErrorIs(t, err, errInvalidCursor)
	require.Nil(t, got)
}
