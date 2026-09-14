package color

import (
	"github.com/salandered/strvalid"
)

// Hex is a normalized color code like "#rrggbb"
// Only Hex created by [NewHex] should be trusted.
type Hex string

func (h Hex) RByte() int {
	return h.channelByte(1)
}
func (h Hex) GByte() int {
	return h.channelByte(3)
}
func (h Hex) BByte() int {
	return h.channelByte(5)
}

func (h Hex) RNorm() float64 {
	return h.channelNormalized(1)
}
func (h Hex) GNorm() float64 {
	return h.channelNormalized(3)
}
func (h Hex) BNorm() float64 {
	return h.channelNormalized(5)
}

// One channel of h as 0 to 255, read at i and i+1.
func (h Hex) channelByte(i int) int {
	return hexDigit(h[i])<<4 | hexDigit(h[i+1])
}

// 0-255 to [0, 1]
func (h Hex) channelNormalized(i int) float64 {
	return float64(h.channelByte(i)) / 255
}

// h is normalized, anything else is unreachable
func hexDigit(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	}
	return 0
}

var hexCfg = strvalid.HexConfig{
	Subject:   "hex color",
	EchoValue: true,
}

// NewHex creates a Hex out of s. Validates and normalizes data from s.
// Normalization: trimmed, lowercased, added '#' if wasn't provided.
func NewHex(s string) (Hex, error) {
	parsed, err := strvalid.ParseHex(s, hexCfg)
	if err != nil {
		return "", err
	}
	return Hex(parsed), nil
}
