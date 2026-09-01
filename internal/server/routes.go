package server

import (
	"net/http"
	"time"

	"github.com/salandered/wavelen/internal/authsvc"
	"github.com/salandered/wavelen/internal/colorsvc"
	"github.com/salandered/wavelen/internal/handlers"
	"github.com/salandered/wavelen/internal/storage"
)

// NewHandler builds the routes and wraps them in the middleware chain.
func NewHandler(s storage.Storage, colorQuota int, tokenTTL time.Duration) http.Handler {
	return requestIDMiddleware(loggingMiddleware(recoveryMiddleware(newMux(s, colorQuota, tokenTTL))))
}

func newMux(s storage.Storage, colorQuota int, tokenTTL time.Duration) *http.ServeMux {
	health := &handlers.HealthHandler{Health: s}
	users := &handlers.UserHandler{Users: s}
	colors := &handlers.ColorHandler{ColorSrv: colorsvc.New(s, colorQuota)}
	catalog := &handlers.CatalogHandler{Catalog: s}
	tokens := &handlers.TokenHandler{TokenSvc: authsvc.New(s, tokenTTL)}

	allowed := func(h http.HandlerFunc) http.Handler {
		return authenticate(s)(requireOwner(h))
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", handlers.HandleRoot)
	mux.HandleFunc("GET /livez", health.HandleLive)
	mux.HandleFunc("GET /readyz", health.HandleReady)
	// users
	mux.HandleFunc("POST /api/v1/users", users.HandleCreateUser)
	// login
	mux.HandleFunc("POST /api/v1/tokens", tokens.HandleCreateToken)
	// user's colors
	mux.Handle("GET /api/v1/users/{user_id}/colors", allowed(colors.HandleListColors))
	mux.Handle("POST /api/v1/users/{user_id}/colors", allowed(colors.HandleAddColor))
	mux.Handle("DELETE /api/v1/users/{user_id}/colors/{hex}", allowed(colors.HandleDeleteColor))
	// common colors palette
	mux.HandleFunc("GET /api/v1/colors", catalog.HandleListCommonColors)
	// harmonies
	mux.HandleFunc("GET /api/v1/colors/{hex}/complement", handlers.HandleComplement)
	mux.HandleFunc("GET /api/v1/colors/{hex}/triad", handlers.HandleTriad)

	return mux
}
