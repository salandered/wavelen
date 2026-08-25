package handlers

import (
	"net/http"

	"github.com/salandered/httputils/httputils"
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

	params, err := listCommonColorsParams(req)
	if err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	common, err := h.Catalog.ListCommonColors(ctx, params)
	if err != nil {
		writeStorageError(ctx, w, err)
		return
	}

	resp := ListCommonColorsResp{Colors: make([]CommonColorResp, 0, len(common))}
	for _, c := range common {
		resp.Colors = append(resp.Colors, CommonColorResp{Hex: string(c.Hex), Name: c.Name})
	}
	httputils.WriteJSON(ctx, w, http.StatusOK, resp)
}

// Both params are optional; the defaults come from storage.
func listCommonColorsParams(req *http.Request) (storage.ListCommonColorsParams, error) {
	params := storage.ListCommonColorsParams{
		Sort:  storage.DefaultCatalogSort,
		Order: storage.DefaultCatalogOrder,
	}

	var err error
	query := req.URL.Query()
	if raw := query.Get(sortQuery); raw != "" {
		if params.Sort, err = storage.ParseCatalogSort(raw); err != nil {
			return params, err
		}
	}
	if raw := query.Get(orderQuery); raw != "" {
		if params.Order, err = storage.ParseSortOrder(raw); err != nil {
			return params, err
		}
	}
	return params, nil
}
