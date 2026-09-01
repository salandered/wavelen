package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/salandered/httputils/httputils"
	"github.com/salandered/wavelen/internal/auth"
)

// the handler is served by a service, not by repo (like color svc)
type TokenService interface {
	Login(ctx context.Context, email, password string) (*auth.Token, error)
}

type TokenHandler struct {
	TokenSvc TokenService
}

type CreateTokenReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type CreateTokenResp struct {
	Token  string    `json:"token"`
	Expiry time.Time `json:"expiry"`
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
	})
}
