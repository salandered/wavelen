package handlers

import (
	"net/http"

	"github.com/salandered/wavelen/internal/storage"
)

type CatalogHandler struct {
	Catalog storage.CatalogRepo
}

type CommonColorResp struct {
	Hex  string `json:"hex"`
	Name string `json:"name"`
}

type ListCommonColorsResp struct {
	Colors []CommonColorResp `json:"colors"`
}

func (h *CatalogHandler) HandleListCommonColors(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	common, err := h.Catalog.ListCommonColors(ctx)
	if err != nil {
		writeStorageError(ctx, w, err)
		return
	}

	resp := ListCommonColorsResp{Colors: make([]CommonColorResp, 0, len(common))}
	for _, c := range common {
		resp.Colors = append(resp.Colors, CommonColorResp{Hex: string(c.Hex), Name: c.Name})
	}
	writeJSON(ctx, w, http.StatusOK, resp)
}
