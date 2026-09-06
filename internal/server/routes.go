package server

import (
	"net/http"
	"time"

	"github.com/salandered/wavelen/internal/authsvc"
	"github.com/salandered/wavelen/internal/colorsvc"
	"github.com/salandered/wavelen/internal/handlers"
	"github.com/salandered/wavelen/internal/storage"
)

// HandlerConfig carries what the routes need
type HandlerConfig struct {
	UserColorQuota  int
	AuthTokenTTL    time.Duration
	AuthConcurLimit int
	AuthConcurWait  time.Duration
}

// NewHandler builds the routes and wraps them in the middleware chain.
func NewHandler(s storage.Storage, cfg HandlerConfig) http.Handler {
	return requestIDMiddleware(loggingMiddleware(recoveryMiddleware(newMux(s, cfg))))
}

func newMux(s storage.Storage, cfg HandlerConfig) *http.ServeMux {
	health := &handlers.HealthHandler{Health: s}
	users := &handlers.UserHandler{Users: s}
	colors := &handlers.ColorHandler{ColorSrv: colorsvc.New(s, cfg.UserColorQuota)}
	catalog := &handlers.CatalogHandler{Catalog: s}
	tokens := &handlers.TokenHandler{TokenSvc: authsvc.New(s, cfg.AuthTokenTTL)}

	authed := authenticate(s) // func(authedHandlerFunc) http.Handler
	// one semaphore, shared by bcryptLimited routes
	bcryptLimited := limitConcurrent(cfg.AuthConcurLimit, cfg.AuthConcurWait)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", handlers.HandleRoot)
	mux.HandleFunc("GET /livez", health.HandleLive)
	mux.HandleFunc("GET /readyz", health.HandleReady)

	//// auth
	// sign up
	mux.Handle("POST /api/v1/users", bcryptLimited(users.HandleCreateUser))
	// login
	mux.Handle("POST /api/v1/tokens", bcryptLimited(tokens.HandleCreateToken))
	// logout
	mux.Handle("DELETE /api/v1/tokens", authed(tokens.HandleDeleteToken))
	// account behind the token
	mux.Handle("GET /api/v1/me", authed(users.HandleGetMe))

	//// user's data
	mux.Handle("GET /api/v1/me/colors", authed(colors.HandleListColors))
	mux.Handle("POST /api/v1/me/colors", authed(colors.HandleAddColor))
	mux.Handle("DELETE /api/v1/me/colors/{hex}", authed(colors.HandleDeleteColor))

	//// common data and operations
	mux.HandleFunc("GET /api/v1/colors", catalog.HandleListCommonColors)
	mux.HandleFunc("GET /api/v1/colors/{hex}/complement", handlers.HandleComplement)
	mux.HandleFunc("GET /api/v1/colors/{hex}/triad", handlers.HandleTriad)

	return mux
}
