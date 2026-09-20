package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/salandered/httputils/httputils"
	"github.com/salandered/wavelen/internal/color"
)

type HarmonyResp struct {
	Hex     string   `json:"hex"`
	Harmony string   `json:"harmony"`
	Space   string   `json:"space"`
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

	space := color.DefSpace
	if raw := req.URL.Query().Get(spaceQuery); raw != "" {
		if space, err = color.ParseSpace(raw); err != nil {
			writeRequestError(ctx, w, err)
			return
		}
	}

	w.Header().Set("Cache-Control", staticCacheControl)
	httputils.WriteJSON(ctx, w, http.StatusOK, HarmonyResp{
		Hex:     string(hex),
		Harmony: string(harmony),
		Space:   string(space),
		Colors:  hexStrings(harmony.Colors(space, hex)),
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
