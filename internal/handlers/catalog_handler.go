package handlers

import (
	"net/http"

	"github.com/salandered/httputils/httputils"
	"github.com/salandered/wavelen/internal/palette"
	"github.com/salandered/wavelen/internal/storage"
)

type CommonColorResp struct {
	Hex  string `json:"hex"`
	Name string `json:"name"`
}

type ListCommonColorsResp struct {
	Colors []CommonColorResp `json:"colors"`
}

func HandleListCommonColors(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	params, err := listCommonColorsParams(req)
	if err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	common, err := palette.List(params)
	if err != nil {
		// the sort is parsed above, so an error here is a bug rather than bad input
		httputils.WriteError(ctx, w, err, http.StatusInternalServerError)
		return
	}

	resp := ListCommonColorsResp{Colors: make([]CommonColorResp, 0, len(common))}
	for _, c := range common {
		resp.Colors = append(resp.Colors, CommonColorResp{Hex: string(c.Hex), Name: c.Name})
	}
	httputils.WriteJSON(ctx, w, http.StatusOK, resp)
}

// Both params are optional; a zero Params is the default ordering.
func listCommonColorsParams(req *http.Request) (palette.SortParams, error) {
	var params palette.SortParams

	query := req.URL.Query()
	if raw := query.Get(sortQuery); raw != "" {
		sort, err := palette.ParseSort(raw)
		if err != nil {
			return params, err
		}
		params.Sort = sort
	}
	if raw := query.Get(orderQuery); raw != "" {
		// storage owns the asc/desc parsing for the whole API, so both listings answer alike
		order, err := storage.ParseSortOrder(raw)
		if err != nil {
			return params, err
		}
		params.Desc = order == storage.OrderDesc
	}
	return params, nil
}
