package color

import (
	"errors"
	"fmt"
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

// Feel is the perceptual ordering key of h.
func Feel(h Hex) int32 {
	return perceptualSortKey(h)
}

// Harmony is a supported color scheme
type Harmony string

const (
	Complement      Harmony = "complement"
	SplitComplement Harmony = "split-complement"
	Triad           Harmony = "triad"
	Analogous       Harmony = "analogous"
	Square          Harmony = "square"
	Ramp            Harmony = "ramp"
	Tones           Harmony = "tones"
)

// The table of: harmony name, harmony formula.
var harmonies = [...]struct {
	name Harmony
	of   func(Hex) []Hex
}{
	{Complement, rotations(complementDeg)},
	{SplitComplement, rotations(splitComplementDeg, 360-splitComplementDeg)},
	{Triad, rotations(triadDeg, 2*triadDeg)},
	{Analogous, rotations(-analogousDeg, analogousDeg)},
	{Square, rotations(squareDeg, 2*squareDeg, 3*squareDeg)},
	{Ramp, ramp},
	{Tones, tones},
}

var ErrUnknownHarmony = errors.New("unknown harmony")

// ParseHarmony creates a Harmony out of s.
func ParseHarmony(s string) (Harmony, error) {
	harmony := Harmony(s)
	for _, h := range harmonies {
		if h.name == harmony {
			return harmony, nil
		}
	}
	return "", fmt.Errorf("%w %q", ErrUnknownHarmony, s)
}

// HarmonyNames lists the harmony names in table order.
func HarmonyNames() []Harmony {
	out := make([]Harmony, len(harmonies))
	for i, h := range harmonies {
		out[i] = h.name
	}
	return out
}

// Colors runs the harmony on hex.
// A unknown Harmony answers nil. Use [ParseHarmony].
func (harmony Harmony) Colors(hex Hex) []Hex {
	for _, h := range harmonies {
		if h.name == harmony {
			return h.of(hex)
		}
	}
	return nil
}

// A harmony turns the hue only. The rotations are the definition.
const (
	complementDeg      = 180
	splitComplementDeg = 150
	triadDeg           = 120
	analogousDeg       = 30
	squareDeg          = 90
)

// A harmony that is a hue rotation.
func rotations(degs ...float64) func(Hex) []Hex {
	return func(h Hex) []Hex {
		out := make([]Hex, len(degs))
		for i, deg := range degs {
			out[i] = rotate(h, deg)
		}
		return out
	}
}
