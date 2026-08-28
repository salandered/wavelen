package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

const (
	DefaultPort            = 8080
	DefaultShutdownTimeout = 10 * time.Second
)

var (
	ErrServer   = errors.New("server")
	ErrServe    = fmt.Errorf("%w: serve failed", ErrServer)
	ErrShutdown = fmt.Errorf("%w: shutdown incomplete", ErrServer)
)

type Config struct {
	Port            int
	ShutdownTimeout time.Duration
}

// Start serves until ctx is cancelled, then drains in-flight requests.
// The caller owns error logging.
func Start(ctx context.Context, handler http.Handler, cfg Config) error {
	srv := &http.Server{
		Addr:           ":" + strconv.Itoa(cfg.Port),
		Handler:        handler,
		IdleTimeout:    time.Minute,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 mb
		// net/http writes its own diagnostics here (superfluous WriteHeader, bad
		// Content-Length, TLS handshake errors). Route them through slog.
		ErrorLog: slog.NewLogLogger(slog.Default().Handler(), slog.LevelWarn),
	}
	slog.Info("starting server",
		"addr", srv.Addr,
		"read_timeout", srv.ReadTimeout,
		"write_timeout", srv.WriteTimeout,
		"idle_timeout", srv.IdleTimeout,
		"shutdown_timeout", cfg.ShutdownTimeout,
	)

	// Start the server. Unexpected error is sent to errServeCh
	errServeCh := make(chan error, 1)
	go func() {
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errServeCh <- err
		}
	}()

	// Block until the ctx is cancelled, or the server sent an unexpected error
	select {
	case err := <-errServeCh:
		return fmt.Errorf("%w: %w", ErrServe, err) // ListenAndServe returns unexpected error -> fail fast
	case <-ctx.Done(): // signal -> graceful shutdown
	}

	// Shutdown
	slog.Info("shutting down server", "timeout", cfg.ShutdownTimeout)
	shutdownStart := time.Now()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil { // when succeeds -> ListenAndServe returns ErrServerClosed
		return fmt.Errorf("%w: %w", ErrShutdown, err)
	}

	slog.Info("server stopped", "shutdown took", time.Since(shutdownStart))
	return nil
}
