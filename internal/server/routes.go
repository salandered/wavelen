package server

import (
	"net/http"

	"github.com/salandered/wavelen/internal/colorsvc"
	"github.com/salandered/wavelen/internal/handlers"
	"github.com/salandered/wavelen/internal/storage"
)

// NewHandler builds the routes and wraps them in the middleware chain.
func NewHandler(s storage.Storage, colorQuota int) http.Handler {
	return requestIDMiddleware(loggingMiddleware(recoveryMiddleware(newMux(s, colorQuota))))
}

func newMux(s storage.Storage, colorQuota int) *http.ServeMux {
	health := &handlers.HealthHandler{Health: s}
	users := &handlers.UserHandler{Users: s}
	colors := &handlers.ColorHandler{ColorSrv: colorsvc.New(s, colorQuota)}
	catalog := &handlers.CatalogHandler{Catalog: s}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", handlers.HandleRoot)
	mux.HandleFunc("GET /livez", health.HandleLive)
	mux.HandleFunc("GET /readyz", health.HandleReady)
	// users
	mux.HandleFunc("POST /api/v1/users", users.HandleCreateUser)
	// user's colors
	mux.HandleFunc("GET /api/v1/users/{user_id}/colors", colors.HandleListColors)
	mux.HandleFunc("POST /api/v1/users/{user_id}/colors", colors.HandleAddColor)
	mux.HandleFunc("DELETE /api/v1/users/{user_id}/colors/{hex}", colors.HandleDeleteColor)
	// common colors palette
	mux.HandleFunc("GET /api/v1/colors", catalog.HandleListCommonColors)
	// harmonies
	mux.HandleFunc("GET /api/v1/colors/{hex}/complement", handlers.HandleComplement)
	mux.HandleFunc("GET /api/v1/colors/{hex}/triad", handlers.HandleTriad)

	return mux
}
