package handlers

import (
	"net/http"

	"github.com/salandered/httputils/httputils"
	"github.com/salandered/wavelen/internal/color"
)

const harmonyCacheControl = "public, max-age=31536000, immutable"

type ComplementResp struct {
	Hex        string `json:"hex"`
	Complement string `json:"complement"`
}

type TriadResp struct {
	Hex   string   `json:"hex"`
	Triad []string `json:"triad"`
}

func HandleComplement(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	hex, err := hexFromPath(req)
	if err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	w.Header().Set("Cache-Control", harmonyCacheControl)
	httputils.WriteJSON(ctx, w, http.StatusOK, ComplementResp{
		Hex:        string(hex),
		Complement: string(color.Complement(hex)),
	})
}

func HandleTriad(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	hex, err := hexFromPath(req)
	if err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	second, third := color.Triad(hex)

	w.Header().Set("Cache-Control", harmonyCacheControl)
	httputils.WriteJSON(ctx, w, http.StatusOK, TriadResp{
		Hex:   string(hex),
		Triad: []string{string(second), string(third)},
	})
}
