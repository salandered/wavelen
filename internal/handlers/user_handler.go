package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/salandered/httputils/httputils"
	"github.com/salandered/wavelen/internal/auth"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
)

type UserService interface {
	CreateUser(ctx context.Context, u *user.User) error
	UserByID(ctx context.Context, id user.ID) (*user.User, error)
	DeleteUser(ctx context.Context, id user.ID) error
	ExportUser(
		ctx context.Context, id user.ID,
	) (*user.User, []storage.CltWithColors, error)
}

type UserHandler struct {
	UserSvc UserService
}

type CreateUserReq struct {
	Nickname string `json:"nickname"`
	Password string `json:"password"`
}

// keeps the data out of the log
func (r CreateUserReq) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("nickname", r.Nickname),
		slog.String("password", redactedValue),
	)
}

// No id: nothing uses it client-side
type UserResp struct {
	Nickname  string    `json:"nickname"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateUserResp struct {
	User UserResp `json:"user"`
}

type MeResp struct {
	User UserResp `json:"user"`
}

func (h *UserHandler) HandleCreateUser(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	var data CreateUserReq
	if err := httputils.ReadJSON(w, req, &data, maxRequestBodyBytes); err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	nickname, err := user.NormalizeNickname(data.Nickname)
	if err != nil {
		writeRequestError(ctx, w, err)
		return
	}
	hash, err := auth.HashPassword(data.Password)
	if err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	u := user.User{Nickname: nickname, PasswordHash: hash}
	if err := h.UserSvc.CreateUser(ctx, &u); err != nil {
		writeStorageError(ctx, w, err)
		return
	}

	httputils.WriteJSON(ctx, w, http.StatusCreated, CreateUserResp{User: userToResp(&u)})
}

// Returns the account which belongs to the token
func (h *UserHandler) HandleGetMe(w http.ResponseWriter, req *http.Request, userID user.ID) {
	ctx := req.Context()

	u, err := h.UserSvc.UserByID(ctx, userID)
	if err != nil {
		writeStorageError(ctx, w, err)
		return
	}
	httputils.WriteJSON(ctx, w, http.StatusOK, MeResp{User: userToResp(u)})
}

// Deletes the userID account and all the data assosiated with it.
// userID is derived from the auth token.
// All the user tokens will also be deleted, so a repeat answers 401.
func (h *UserHandler) HandleDeleteMe(w http.ResponseWriter, req *http.Request, userID user.ID) {
	ctx := req.Context()

	if err := h.UserSvc.DeleteUser(ctx, userID); err != nil {
		writeStorageError(ctx, w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Export handler

// The export format version.
// Bump it if the JSON structure changes.
const exportVer = 1

// reusing existing resps for consistency
type ExportCollectionResp struct {
	CollectionResp
	Colors []SavedColorResp `json:"colors"`
}

type ExportResp struct {
	Format      int                    `json:"format"`
	ExportedAt  time.Time              `json:"exported_at"`
	User        UserResp               `json:"user"`
	Collections []ExportCollectionResp `json:"collections"`
}

func (h *UserHandler) HandleExport(w http.ResponseWriter, req *http.Request, userID user.ID) {
	ctx := req.Context()

	usr, collections, err := h.UserSvc.ExportUser(ctx, userID)
	if err != nil {
		writeStorageError(ctx, w, err)
		return
	}

	now := time.Now().UTC()
	resp := ExportResp{
		Format:      exportVer,
		ExportedAt:  now,
		User:        userToResp(usr),
		Collections: make([]ExportCollectionResp, 0, len(collections)),
	}

	for _, c := range collections {
		colors := make([]SavedColorResp, 0, len(c.Colors)) // an empty one renders [], not null
		for _, saved := range c.Colors {
			colors = append(colors, SavedColorResp{
				Hex:       string(saved.Hex),
				CreatedAt: saved.CreatedAt.UTC(),
			})
		}
		resp.Collections = append(resp.Collections, ExportCollectionResp{
			CollectionResp: collectionToResp(&c.Clt),
			Colors:         colors,
		})
	}

	// before WriteJSON, which writes the status
	w.Header().Set("Content-Disposition", contentDisposition(usr.Nickname, now))
	httputils.WriteJSON(ctx, w, http.StatusOK, resp)
}

// A client that uses the header saves the file under this filename.
// A nickname is [a-z0-9_-], so no escaping or quoting here.
// see https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Content-Disposition
func contentDisposition(nickname string, ts time.Time) string {
	return fmt.Sprintf(
		`attachment; filename="wavelen-%s-%s.json"`,
		nickname, ts.Format("20060102T150405Z"),
	)
}

// utils

func userToResp(u *user.User) UserResp {
	return UserResp{
		Nickname:  u.Nickname,
		CreatedAt: u.CreatedAt.UTC(),
	}
}
