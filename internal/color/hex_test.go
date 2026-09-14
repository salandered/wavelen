package color

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// test only what NewHex adds, not strvalid.
func TestNewHexNormalizes(t *testing.T) {
	got, err := NewHex("  FF0000 ")

	require.NoError(t, err)
	require.Equal(t, Hex("#ff0000"), got)
}

func TestNewHexRejectsMalformed(t *testing.T) {
	got, err := NewHex("#fff")

	require.Error(t, err)
	require.Empty(t, got)
}

func TestHexBytesReadEachChannel(t *testing.T) {
	h := Hex("#0a80ff")

	require.Equal(t, 10, h.RByte())
	require.Equal(t, 128, h.GByte())
	require.Equal(t, 255, h.BByte())
}

func TestHexNormsReadEachChannel(t *testing.T) {
	h := Hex("#0a80ff")

	require.InDelta(t, 10.0/255, h.RNorm(), 1e-12)
	require.InDelta(t, 128.0/255, h.GNorm(), 1e-12)
	require.InDelta(t, 1.0, h.BNorm(), 1e-12)
}

func TestHexBoundsAreBlackAndWhite(t *testing.T) {
	black, white := Hex("#000000"), Hex("#ffffff")

	require.Equal(t, 0, black.RByte())
	require.Equal(t, 0.0, black.GNorm())
	require.Equal(t, 255, white.BByte())
	require.Equal(t, 1.0, white.RNorm())
}
