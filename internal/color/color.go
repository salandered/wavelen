package color

import (
	"time"

	"github.com/salandered/strvalid"
)

// Hex is a normalized color code like "#rrggbb"
type Hex string

const HexLen = 7

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

var hexCfg = strvalid.HexConfig{
	Subject:   "hex color",
	EchoValue: true,
}

// ParseHex creates a Hex out of s. Validates and normalizes data from s.
// Normalization: trimmed, lowercased, added '#' if wasn't provided.
func ParseHex(s string) (Hex, error) {
	parsed, err := strvalid.ParseHex(s, hexCfg)
	if err != nil {
		return "", err
	}
	return Hex(parsed), nil
}
