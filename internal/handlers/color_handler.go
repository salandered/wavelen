package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/salandered/httputils/httputils"
	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
)

type ColorService interface {
	AddColor(ctx context.Context, userID user.ID, hex color.Hex) (bool, error)
	ListColors(
		ctx context.Context, userID user.ID, p storage.ListColorsParams,
	) (storage.ColorPage, error)
	DeleteColor(ctx context.Context, userID user.ID, hex color.Hex) error
}

type ColorHandler struct {
	ColorSrv ColorService
}

type AddColorReq struct {
	Hex string `json:"hex"`
}

// The normalized hex, so the client learns what was actually stored.
type AddColorResp struct {
	Hex string `json:"hex"`
}

type SavedColorResp struct {
	Hex       string    `json:"hex"`
	CreatedAt time.Time `json:"created_at"`
}

type ListColorsResp struct {
	Colors   []SavedColorResp `json:"colors"`
	Metadata cursorMeta       `json:"metadata"`
}

func (h *ColorHandler) HandleAddColor(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	userID, err := userIDFromPath(req)
	if err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	var data AddColorReq
	if err := httputils.ReadJSON(w, req, &data, maxRequestBodyBytes); err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	hex, err := color.ParseHex(data.Hex)
	if err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	created, err := h.ColorSrv.AddColor(ctx, userID, hex)
	if err != nil {
		writeStorageError(ctx, w, err)
		return
	}

	status := http.StatusOK // already saved
	if created {
		status = http.StatusCreated
	}
	httputils.WriteJSON(ctx, w, status, AddColorResp{Hex: string(hex)})
}

func (h *ColorHandler) HandleListColors(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	userID, err := userIDFromPath(req)
	if err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	params, err := listColorsParams(req)
	if err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	page, err := h.ColorSrv.ListColors(ctx, userID, params)
	if err != nil {
		writeStorageError(ctx, w, err)
		return
	}

	resp := ListColorsResp{
		Colors:   make([]SavedColorResp, 0, len(page.Colors)),
		Metadata: cursorMeta{Limit: params.Limit},
	}
	for _, s := range page.Colors {
		resp.Colors = append(resp.Colors, SavedColorResp{
			Hex:       string(s.Hex),
			CreatedAt: s.CreatedAt.UTC(),
		})
	}

	if page.HasMore && len(page.Colors) > 0 {
		last := page.Colors[len(page.Colors)-1]
		resp.Metadata.NextCursor = encodeCursor(params.Sort, params.Order, last)
	}

	httputils.WriteJSON(ctx, w, http.StatusOK, resp)
}

func (h *ColorHandler) HandleDeleteColor(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	userID, err := userIDFromPath(req)
	if err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	hex, err := hexFromPath(req)
	if err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	if err := h.ColorSrv.DeleteColor(ctx, userID, hex); err != nil {
		writeStorageError(ctx, w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Every param is optional,  defaults come from storage
func listColorsParams(req *http.Request) (storage.ListColorsParams, error) {
	params := storage.ListColorsParams{
		Sort:  storage.DefaultColorSort,
		Order: storage.DefaultColorOrder,
	}

	limit, err := httputils.ParseIntQuery(
		req, limitQuery, storage.DefaultColorLimit, 1, storage.MaxColorLimit)
	if err != nil {
		return params, err
	}
	params.Limit = int(limit) // bounded by MaxColorLimit above

	query := req.URL.Query()
	if raw := query.Get(sortQuery); raw != "" {
		if params.Sort, err = storage.ParseColorSort(raw); err != nil {
			return params, err
		}
	}
	if raw := query.Get(orderQuery); raw != "" {
		if params.Order, err = storage.ParseSortOrder(raw); err != nil {
			return params, err
		}
	}
	// validate against the sort and order
	if raw := query.Get(cursorQuery); raw != "" {
		if params.After, err = decodeCursor(raw, params.Sort, params.Order); err != nil {
			return params, err
		}
	}
	return params, nil
}
