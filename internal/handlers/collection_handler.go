package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/salandered/httputils/httputils"
	"github.com/salandered/wavelen/internal/clt"
	"github.com/salandered/wavelen/internal/color"
	"github.com/salandered/wavelen/internal/icon"
	"github.com/salandered/wavelen/internal/user"
)

type CollectionService interface {
	CreateCollection(
		ctx context.Context, userID user.ID, p clt.CreateParams,
	) (*clt.Collection, error)
	ListCollections(ctx context.Context, userID user.ID) ([]clt.Collection, error)
	CollectionByID(
		ctx context.Context, userID user.ID, id clt.ID,
	) (*clt.Collection, error)
	UpdateCollection(
		ctx context.Context, userID user.ID, id clt.ID, p clt.UpdateParams,
	) (*clt.Collection, error)
	DeleteCollection(ctx context.Context, userID user.ID, id clt.ID) error
}

type CollectionHandler struct {
	CollectionSvc CollectionService
}

type CreateCollectionReq struct {
	Name   string `json:"name"`
	Icon   string `json:"icon"`
	Accent string `json:"accent"`
}

// using pointers - nil means "wasn't sent"
type UpdateCollectionReq struct {
	Name   *string `json:"name"`
	Icon   *string `json:"icon"`
	Accent *string `json:"accent"`
}

type CollectionResp struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Icon      string    `json:"icon"`
	Accent    string    `json:"accent"`
	IsDefault bool      `json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
}

type OneCollectionResp struct {
	Collection CollectionResp `json:"collection"`
}

type ListCollectionsResp struct {
	Collections []CollectionResp `json:"collections"`
}

func (h *CollectionHandler) HandleCreateCollection(
	w http.ResponseWriter, req *http.Request, userID user.ID,
) {
	ctx := req.Context()

	var data CreateCollectionReq
	if err := httputils.ReadJSON(w, req, &data, maxRequestBodyBytes); err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	params, err := createCollectionParams(data)
	if err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	created, err := h.CollectionSvc.CreateCollection(ctx, userID, params)
	if err != nil {
		writeStorageError(ctx, w, err)
		return
	}

	httputils.WriteJSON(ctx, w, http.StatusCreated,
		OneCollectionResp{Collection: collectionToResp(created)})
}

func (h *CollectionHandler) HandleListCollections(
	w http.ResponseWriter, req *http.Request, userID user.ID,
) {
	ctx := req.Context()

	collections, err := h.CollectionSvc.ListCollections(ctx, userID)
	if err != nil {
		writeStorageError(ctx, w, err)
		return
	}

	resp := ListCollectionsResp{Collections: make([]CollectionResp, 0, len(collections))}
	for _, c := range collections {
		resp.Collections = append(resp.Collections, collectionToResp(&c))
	}
	httputils.WriteJSON(ctx, w, http.StatusOK, resp)
}

func (h *CollectionHandler) HandleGetCollection(
	w http.ResponseWriter, req *http.Request, userID user.ID,
) {
	ctx := req.Context()

	id, err := collectionIDFromPath(req)
	if err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	col, err := h.CollectionSvc.CollectionByID(ctx, userID, id)
	if err != nil {
		writeStorageError(ctx, w, err)
		return
	}
	httputils.WriteJSON(ctx, w, http.StatusOK,
		OneCollectionResp{Collection: collectionToResp(col)})
}

func (h *CollectionHandler) HandleUpdateCollection(
	w http.ResponseWriter, req *http.Request, userID user.ID,
) {
	ctx := req.Context()

	id, err := collectionIDFromPath(req)
	if err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	var data UpdateCollectionReq
	if err := httputils.ReadJSON(w, req, &data, maxRequestBodyBytes); err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	params, err := updateCollectionParams(data)
	if err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	updated, err := h.CollectionSvc.UpdateCollection(ctx, userID, id, params)
	if err != nil {
		writeStorageError(ctx, w, err)
		return
	}

	httputils.WriteJSON(ctx, w, http.StatusOK,
		OneCollectionResp{Collection: collectionToResp(updated)})
}

func (h *CollectionHandler) HandleDeleteCollection(
	w http.ResponseWriter, req *http.Request, userID user.ID,
) {
	ctx := req.Context()

	id, err := collectionIDFromPath(req)
	if err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	if err := h.CollectionSvc.DeleteCollection(ctx, userID, id); err != nil {
		writeStorageError(ctx, w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func createCollectionParams(data CreateCollectionReq) (clt.CreateParams, error) {
	name, err := clt.NormalizeName(data.Name)
	if err != nil {
		return clt.CreateParams{}, err
	}

	// not required
	slug := clt.DefIconSlug
	if data.Icon != "" {
		if slug, err = icon.ParseSlug(data.Icon); err != nil {
			return clt.CreateParams{}, err
		}
	}

	// not required
	accent := clt.DefIconAccent
	if data.Accent != "" {
		if accent, err = color.NewHex(data.Accent); err != nil {
			return clt.CreateParams{}, err
		}
	}
	return clt.CreateParams{Name: name, Icon: slug, Accent: accent}, nil
}

// Validates present fields. All nils is refused.
func updateCollectionParams(data UpdateCollectionReq) (clt.UpdateParams, error) {
	if data.Name == nil && data.Icon == nil && data.Accent == nil {
		return clt.UpdateParams{},
			errors.New("nothing to update: send name, icon or accent")
	}

	var p clt.UpdateParams

	if data.Name != nil {
		name, err := clt.NormalizeName(*data.Name)
		if err != nil {
			return clt.UpdateParams{}, err
		}
		p.Name = &name
	}

	if data.Icon != nil {
		slug, err := icon.ParseSlug(*data.Icon)
		if err != nil {
			return clt.UpdateParams{}, err
		}
		p.Icon = &slug
	}

	if data.Accent != nil {
		accent, err := color.NewHex(*data.Accent)
		if err != nil {
			return clt.UpdateParams{}, err
		}
		p.Accent = &accent
	}
	return p, nil
}

func collectionToResp(c *clt.Collection) CollectionResp {
	return CollectionResp{
		ID:        c.ID.String(),
		Name:      c.Name,
		Icon:      string(c.IconSlug),
		Accent:    string(c.IconAccent),
		IsDefault: c.IsDefault,
		CreatedAt: c.CreatedAt.UTC(),
	}
}
