package color

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Hex is a color code as it is stored, lowercased. Example: "#rrggbb"
type Hex string

// HexLen is the length of the storable form.
const HexLen = 7

var ErrInvalidHex = errors.New("invalid hex color")

// Color is one entry of a user's own list.
type Color struct {
	Hex       Hex
	CreatedAt time.Time
}

// Common is one entry of the shared palette.
type Common struct {
	Hex  Hex
	Name string
}

// ParseHex creates a Hex out of s. Validates and normalizes data from s.
// Normalization: trimmed, lowercased, added '#' if wasn't provided.
func ParseHex(s string) (Hex, error) {
	digits := strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(digits) != HexLen-1 {
		return "", fmt.Errorf("%w: want 6 hex digits, got %q", ErrInvalidHex, s)
	}

	var b strings.Builder
	b.Grow(HexLen)
	b.WriteByte('#')
	for i := range len(digits) {
		c := digits[i]
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f':
			b.WriteByte(c)
		case c >= 'A' && c <= 'F':
			b.WriteByte(c - 'A' + 'a')
		default:
			return "", fmt.Errorf("%w: want 6 hex digits, got %q", ErrInvalidHex, s)
		}
	}
	return Hex(b.String()), nil
}
