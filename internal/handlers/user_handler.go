package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/salandered/httputils/httputils"
	"github.com/salandered/wavelen/internal/auth"
	"github.com/salandered/wavelen/internal/user"
)

type UserService interface {
	CreateUser(ctx context.Context, u *user.User) error
	UserByID(ctx context.Context, id user.ID) (*user.User, error)
}

type UserHandler struct {
	UserSvc UserService
}

type CreateUserReq struct {
	Nickname string `json:"nickname"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

// No id: nothing uses it client-side
type UserResp struct {
	Nickname  string    `json:"nickname"`
	Name      string    `json:"name"`
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
	name, err := user.NormalizeName(data.Name)
	if err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	hash, err := auth.HashPassword(data.Password)
	if err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	u := user.User{Nickname: nickname, Name: name, PasswordHash: hash}
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

func userToResp(u *user.User) UserResp {
	return UserResp{
		Nickname:  u.Nickname,
		Name:      u.Name,
		CreatedAt: u.CreatedAt.UTC(),
	}
}
