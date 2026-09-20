package handlers

import (
	"net/http"

	"github.com/salandered/httputils/httputils"
	"github.com/salandered/wavelen/internal/shades"
)

type ShadeFamilyResp struct {
	Name   string            `json:"name"`
	Colors []CommonColorResp `json:"colors"`
}

type ListShadesResp struct {
	Families []ShadeFamilyResp `json:"families"`
}

func HandleListShades(w http.ResponseWriter, req *http.Request) {
	families := shades.Families()

	resp := ListShadesResp{Families: make([]ShadeFamilyResp, 0, len(families))}
	for _, f := range families {
		row := ShadeFamilyResp{Name: f.Name, Colors: make([]CommonColorResp, 0, len(f.Colors))}
		for _, c := range f.Colors {
			row.Colors = append(row.Colors, CommonColorResp{Hex: string(c.Hex), Name: c.Name})
		}
		resp.Families = append(resp.Families, row)
	}

	w.Header().Set("Cache-Control", staticCacheControl)
	httputils.WriteJSON(req.Context(), w, http.StatusOK, resp)
}
