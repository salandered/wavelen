package handlers

import (
	"net/http"
	"time"

	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/storage"
)

type ColorHandler struct {
	Colors storage.ColorRepo
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
	Colors []SavedColorResp `json:"colors"`
}

func (h *ColorHandler) HandleAddColor(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	userID, err := userIDFromPath(req)
	if err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	var data AddColorReq
	if err := readJSON(w, req, &data); err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	hex, err := color.ParseHex(data.Hex)
	if err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	created, err := h.Colors.AddColor(ctx, userID, hex)
	if err != nil {
		writeStorageError(ctx, w, err)
		return
	}

	status := http.StatusOK // already saved
	if created {
		status = http.StatusCreated
	}
	writeJSON(ctx, w, status, AddColorResp{Hex: string(hex)})
}

func (h *ColorHandler) HandleListColors(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	userID, err := userIDFromPath(req)
	if err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	saved, err := h.Colors.ListColors(ctx, userID)
	if err != nil {
		writeStorageError(ctx, w, err)
		return
	}

	resp := ListColorsResp{Colors: make([]SavedColorResp, 0, len(saved))}
	for _, s := range saved {
		resp.Colors = append(resp.Colors, SavedColorResp{
			Hex:       string(s.Hex),
			CreatedAt: s.CreatedAt.UTC(),
		})
	}
	writeJSON(ctx, w, http.StatusOK, resp)
}
