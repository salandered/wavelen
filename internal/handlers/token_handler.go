package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/salandered/httputils/httputils"
	"github.com/salandered/wavelen/internal/auth"
)

// the handler is served by a service, not by repo (like color svc)
type TokenService interface {
	Login(ctx context.Context, email, password string) (*auth.Token, error)
	Logout(ctx context.Context, hash []byte) error
}

type TokenHandler struct {
	TokenSvc TokenService
}

type CreateTokenReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// UserID is what the caller would need to put in the path
type CreateTokenResp struct {
	Token  string    `json:"token"`
	Expiry time.Time `json:"expiry"`
	UserID int64     `json:"user_id"`
}

func (h *TokenHandler) HandleCreateToken(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	var data CreateTokenReq
	if err := httputils.ReadJSON(w, req, &data, maxRequestBodyBytes); err != nil {
		writeRequestError(ctx, w, err)
		return
	}

	token, err := h.TokenSvc.Login(ctx, data.Email, data.Password)
	if err != nil {
		writeStorageError(ctx, w, err)
		return
	}

	httputils.WriteJSON(ctx, w, http.StatusCreated, CreateTokenResp{
		Token:  token.Plaintext,
		Expiry: token.Expiry.UTC(),
		UserID: int64(token.UserID),
	})
}

func (h *TokenHandler) HandleDeleteToken(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	hash, ok := auth.TokenHashFromContext(ctx)
	if !ok {
		// the route was registered without the auth wrapper: a code bug
		// TODO: more prominent
		slog.ErrorContext(ctx, "HandleDeleteToken reached with no token hash")
		httputils.WriteError(ctx, w,
			errors.New("internal server error"), http.StatusInternalServerError)
		return
	}

	if err := h.TokenSvc.Logout(ctx, hash); err != nil {
		writeStorageError(ctx, w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
