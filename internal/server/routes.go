package server

import (
	"io/fs"
	"net/http"
	"time"

	"github.com/salandered/wavelen/internal/authsvc"
	"github.com/salandered/wavelen/internal/collectionsvc"
	"github.com/salandered/wavelen/internal/colorsvc"
	"github.com/salandered/wavelen/internal/handlers"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/usersvc"
)

type HandlerConfig struct {
	WebFS               fs.FS // static UI, rooted at index.html
	UserColorQuota      int
	UserCollectionQuota int
	AuthTokenTTL        time.Duration
	AuthConcurLimit     int
	AuthConcurWait      time.Duration
}

// NewHandler builds the routes and wraps them in the middleware chain.
func NewHandler(s storage.Storage, cfg HandlerConfig) http.Handler {
	return requestIDMiddleware(loggingMiddleware(recoveryMiddleware(newMux(s, cfg))))
}

func newMux(s storage.Storage, cfg HandlerConfig) *http.ServeMux {
	health := &handlers.HealthHandler{Health: s}
	users := &handlers.UserHandler{UserSvc: usersvc.New(s)}
	colors := &handlers.ColorHandler{ColorSrv: colorsvc.New(s, cfg.UserColorQuota)}
	collections := &handlers.CollectionHandler{
		CollectionSvc: collectionsvc.New(s, cfg.UserCollectionQuota),
	}
	tokens := &handlers.TokenHandler{TokenSvc: authsvc.New(s, cfg.AuthTokenTTL)}

	authed := authenticate(s) // func(authedHandlerFunc) http.Handler
	// one semaphore, shared by bcryptLimited routes
	bcryptLimited := limitConcurrent(cfg.AuthConcurLimit, cfg.AuthConcurWait)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /livez", health.HandleLive)
	mux.HandleFunc("GET /readyz", health.HandleReady)
	mux.HandleFunc("GET /api/v1/version", handlers.HandleVersion)

	// index.html at "/", and 404 for every unmatched request.
	// Note: registered without a method. "GET /" would make any non GET turn into 405.
	mux.Handle("/", handlers.NewStaticHandler(cfg.WebFS))

	//// auth
	// sign up
	mux.Handle("POST /api/v1/users", bcryptLimited(users.HandleCreateUser))
	// login
	mux.Handle("POST /api/v1/tokens", bcryptLimited(tokens.HandleCreateToken))
	// logout
	mux.Handle("DELETE /api/v1/tokens", authed(tokens.HandleDeleteToken))
	// the account behind the token
	mux.Handle("GET /api/v1/me", authed(users.HandleGetMe))
	// delete the accoubt
	mux.Handle("DELETE /api/v1/me", authed(users.HandleDeleteMe))

	//// user's data
	mux.Handle("GET /api/v1/me/collections", authed(collections.HandleListCollections))
	mux.Handle("POST /api/v1/me/collections", authed(collections.HandleCreateCollection))
	mux.Handle("GET /api/v1/me/collections/{id}", authed(collections.HandleGetCollection))
	mux.Handle("DELETE /api/v1/me/collections/{id}", authed(collections.HandleDeleteCollection))

	mux.Handle("GET /api/v1/me/collections/{id}/colors", authed(colors.HandleListColors))
	mux.Handle("POST /api/v1/me/collections/{id}/colors", authed(colors.HandleAddColor))
	mux.Handle("DELETE /api/v1/me/collections/{id}/colors", authed(colors.HandleDeleteAllColors))
	mux.Handle("DELETE /api/v1/me/collections/{id}/colors/{hex}", authed(colors.HandleDeleteColor))

	//// common data and operations
	mux.HandleFunc("GET /api/v1/colors", handlers.HandleListCommonColors)
	// one route for every harmony
	mux.HandleFunc("GET /api/v1/colors/{hex}/{harmony}", handlers.HandleHarmony)

	return mux
}
