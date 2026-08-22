package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/user"
)

type UserHandler struct {
	Users storage.UserRepo
}

type CreateUserReq struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type UserResp struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateUserResp struct {
	User UserResp `json:"user"`
}

func (h *UserHandler) HandleCreateUser(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	var data CreateUserReq
	if err := readJSON(w, req, &data); err != nil {
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

	u := user.User{Email: email, Name: name}
	if err := h.Users.CreateUser(ctx, &u); err != nil {
		writeStorageError(ctx, w, err)
		return
	}

	w.Header().Set("Location", fmt.Sprintf("/api/v1/users/%d", u.ID))
	writeJSON(ctx, w, http.StatusCreated, CreateUserResp{User: userToResp(&u)})
}

func userToResp(u *user.User) UserResp {
	return UserResp{
		ID:        int64(u.ID),
		Email:     u.Email,
		Name:      u.Name,
		CreatedAt: u.CreatedAt.UTC(),
	}
}
