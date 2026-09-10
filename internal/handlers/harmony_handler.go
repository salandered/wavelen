package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/salandered/httputils/httputils"
	"github.com/salandered/wavelen/internal/color"
)

const harmonyCacheControl = "public, max-age=31536000, immutable"

type HarmonyResp struct {
	Hex     string   `json:"hex"`
	Harmony string   `json:"harmony"`
	Colors  []string `json:"colors"`
}

func HandleHarmony(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	hex, err := hexFromPath(req)
	if err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	harmony, err := color.ParseHarmony(req.PathValue(harmonyPathValue))
	if err != nil {
		// wrong path segment
		httputils.WriteError(ctx, w, fmt.Errorf("%w: one of %s", err, harmonyNameList()),
			http.StatusNotFound)
		return
	}

	w.Header().Set("Cache-Control", harmonyCacheControl)
	httputils.WriteJSON(ctx, w, http.StatusOK, HarmonyResp{
		Hex:     string(hex),
		Harmony: string(harmony),
		Colors:  hexStrings(harmony.Colors(hex)),
	})
}

func hexStrings(colors []color.Hex) []string {
	out := make([]string, len(colors))
	for i, h := range colors {
		out[i] = string(h)
	}
	return out
}

// The supported harmonies as one comma separated string.
func harmonyNameList() string {
	names := color.HarmonyNames()
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = string(n)
	}
	return strings.Join(out, ", ")
}
