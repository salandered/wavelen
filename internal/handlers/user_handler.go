package handlers

import (
	"net/http"
	"time"

	"github.com/salandered/httputils/httputils"
	"github.com/salandered/wavelen/internal/auth"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
)

type UserHandler struct {
	Users storage.UserRepo
}

type CreateUserReq struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

// No id: nothing uses it client-side
type UserResp struct {
	Email     string    `json:"email"`
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

	email, err := user.NormalizeEmail(data.Email)
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

	u := user.User{Email: email, Name: name, PasswordHash: hash}
	if err := h.Users.CreateUser(ctx, &u); err != nil {
		writeStorageError(ctx, w, err)
		return
	}

	httputils.WriteJSON(ctx, w, http.StatusCreated, CreateUserResp{User: userToResp(&u)})
}

// Returns the account which belongs to the token
func (h *UserHandler) HandleGetMe(w http.ResponseWriter, req *http.Request, userID user.ID) {
	ctx := req.Context()

	u, err := h.Users.UserByID(ctx, userID)
	if err != nil {
		writeStorageError(ctx, w, err)
		return
	}
	httputils.WriteJSON(ctx, w, http.StatusOK, MeResp{User: userToResp(u)})
}

func userToResp(u *user.User) UserResp {
	return UserResp{
		Email:     u.Email,
		Name:      u.Name,
		CreatedAt: u.CreatedAt.UTC(),
	}
}
