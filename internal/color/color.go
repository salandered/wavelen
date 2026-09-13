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

// Feel is the perceptual ordering key of h
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

// Space defines the target color wheel for hue adjustments
type Space string

const (
	OKLab Space = "oklab"
	HSL   Space = "hsl"
)

// DefSpace is what an unset Space means.
const DefSpace = HSL

/*
Each row is a harmony name + its hue rotations or the sweep

Sweep moves lightness or chroma with the hue held, it has no angle to turn and the Space does not reach it.
One of the two fields is set per row.
*/
var harmonies = [...]struct {
	name Harmony
	degs []float64
	of   func(Hex) []Hex
}{
	{name: Complement, degs: []float64{complementDeg}},
	{name: SplitComplement, degs: []float64{splitComplementDeg, 360 - splitComplementDeg}},
	{name: Triad, degs: []float64{triadDeg, 2 * triadDeg}},
	{name: Analogous, degs: []float64{-analogousDeg, analogousDeg}},
	{name: Square, degs: []float64{squareDeg, 2 * squareDeg, 3 * squareDeg}},
	{name: Ramp, of: ramp},
	{name: Tones, of: tones},
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

var ErrUnknownSpace = errors.New("unknown color space")

// ParseSpace creates a Space out of s.
// Unknown returns error.
func ParseSpace(s string) (Space, error) {
	switch space := Space(s); space {
	case OKLab, HSL:
		return space, nil
	}
	return "", fmt.Errorf("%w %q: want %q or %q", ErrUnknownSpace, s, OKLab, HSL)
}

// HarmonyNames lists the harmony names in table order.
func HarmonyNames() []Harmony {
	out := make([]Harmony, len(harmonies))
	for i, h := range harmonies {
		out[i] = h.name
	}
	return out
}

// Colors runs the harmony on hex, turning the hue on space's wheel.
// An unknown Harmony returns nil. Use [ParseHarmony].
func (harmony Harmony) Colors(space Space, hex Hex) []Hex {
	for _, h := range harmonies {
		if h.name != harmony {
			continue
		}
		if h.of != nil {
			return h.of(hex)
		}
		return rotations(space, hex, h.degs)
	}
	return nil
}

// The angles that the rotation harmonies would turn.
const (
	complementDeg      = 180
	splitComplementDeg = 150
	triadDeg           = 120
	analogousDeg       = 30
	squareDeg          = 90
)

// A harmony that is a hue rotation, turned on space's wheel.
func rotations(space Space, h Hex, degs []float64) []Hex {
	out := make([]Hex, len(degs))
	for i, deg := range degs {
		out[i] = rotate(space, h, deg)
	}
	return out
}
