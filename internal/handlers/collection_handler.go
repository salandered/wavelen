package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/salandered/httputils/httputils"
	"github.com/salandered/wavelen/internal/collection"
	"github.com/salandered/wavelen/internal/user"
)

type CollectionService interface {
	CreateCollection(
		ctx context.Context, userID user.ID, name string,
	) (*collection.Collection, error)
	ListCollections(ctx context.Context, userID user.ID) ([]collection.Collection, error)
	CollectionByID(
		ctx context.Context, userID user.ID, id collection.ID,
	) (*collection.Collection, error)
	DeleteCollection(ctx context.Context, userID user.ID, id collection.ID) error
}

type CollectionHandler struct {
	CollectionSvc CollectionService
}

type CreateCollectionReq struct {
	Name string `json:"name"`
}

type CollectionResp struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
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

	name, err := collection.NormalizeName(data.Name)
	if err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	created, err := h.CollectionSvc.CreateCollection(ctx, userID, name)
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

func collectionToResp(c *collection.Collection) CollectionResp {
	return CollectionResp{
		ID:        c.ID.String(),
		Name:      c.Name,
		IsDefault: c.IsDefault,
		CreatedAt: c.CreatedAt.UTC(),
	}
}
