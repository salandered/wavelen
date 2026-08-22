package server

import (
	"net/http"

	"github.com/salandered/wavelen/internal/handlers"
	"github.com/salandered/wavelen/internal/storage"
)

// NewHandler builds the routes and wraps them in the middleware chain.
func NewHandler(s storage.Storage) http.Handler {
	return requestIDMiddleware(loggingMiddleware(recoveryMiddleware(newMux(s))))
}

func newMux(s storage.Storage) *http.ServeMux {
	users := &handlers.UserHandler{Users: s}
	colors := &handlers.ColorHandler{Colors: s}
	catalog := &handlers.CatalogHandler{Catalog: s}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", handlers.HandleRoot)
	// users
	mux.HandleFunc("POST /api/v1/users", users.HandleCreateUser)
	// user's colors
	mux.HandleFunc("GET /api/v1/users/{user_id}/colors", colors.HandleListColors)
	mux.HandleFunc("POST /api/v1/users/{user_id}/colors", colors.HandleAddColor)
	// common colors palette
	mux.HandleFunc("GET /api/v1/colors", catalog.HandleListCommonColors)

	return mux
}
