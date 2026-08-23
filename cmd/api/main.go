package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/salandered/wavelen/internal/requestid"
	"github.com/salandered/wavelen/internal/server"
	"github.com/salandered/wavelen/internal/storage"

	logging "github.com/salandered/slogenv"
)

var ErrConfig = errors.New("invalid config")

const (
	defaultDSN      = "postgres://justuser:justuser@localhost:5433/wavelen?sslmode=disable"
	maxConns        = 25
	maxConnIdleTime = 15 * time.Minute
	connectTimeout  = 5 * time.Second
)

func main() {
	logCloser, err := setupLogging()
	if err != nil {
		// the logger isn't ready yet, report to stderr directly
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer func() { _ = logCloser.Close() }()

	if err := run(); err != nil {
		slog.Error("wavelen failed", "error", err)
		os.Exit(1)
	}
}

// LOG_LEVEL, LOG_FORMAT, LOG_FILE and LOG_TIME are read by slogenv, see its README.
// requestid.LogAttrs puts the correlation id in every record logged with a request ctx
func setupLogging() (io.Closer, error) {
	cfg, err := logging.ConfigFromEnv()
	if err != nil {
		return nil, err
	}
	return logging.Setup(cfg, requestid.LogAttrs)
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		// after the first signal, restore default handling
		// a second Ctrl+C kills immediately (not a graceful shutdown)
		stop()
	}()

	cfg, err := serverConfig()
	if err != nil {
		return err
	}

	pool, err := openPool(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()
	slog.Info("database connection pool established")

	return server.Start(ctx, server.NewHandler(storage.New(pool)), cfg)
}

func openPool(ctx context.Context) (*pgxpool.Pool, error) {
	dsn := os.Getenv("WAVELEN_DB_DSN")
	if dsn == "" {
		dsn = defaultDSN
	}

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("%w: WAVELEN_DB_DSN: %w", ErrConfig, err)
	}
	poolCfg.MaxConns = maxConns
	poolCfg.MaxConnIdleTime = maxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("database pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database ping: %w", err)
	}
	return pool, nil
}

func serverConfig() (server.Config, error) {
	port, err := intFromEnv("PORT", server.DefaultPort)
	if err != nil {
		return server.Config{}, err
	}
	shutdownTimeout, err := durationFromEnv("SHUTDOWN_TIMEOUT", server.DefaultShutdownTimeout)
	if err != nil {
		return server.Config{}, err
	}
	if port <= 0 || port > 65535 {
		return server.Config{}, fmt.Errorf("%w: PORT should be in 1..65535, got %d", ErrConfig, port)
	}
	if shutdownTimeout <= 0 {
		return server.Config{}, fmt.Errorf(
			"%w: SHUTDOWN_TIMEOUT should be positive, got %v", ErrConfig, shutdownTimeout)
	}
	return server.Config{Port: port, ShutdownTimeout: shutdownTimeout}, nil
}

func intFromEnv(name string, def int) (int, error) {
	v := os.Getenv(name)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%w: %s=%q: %w", ErrConfig, name, v, err)
	}
	return n, nil
}

func durationFromEnv(name string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(name)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%w: %s=%q: %w", ErrConfig, name, v, err)
	}
	return d, nil
}
