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
	"github.com/salandered/wavelen/internal/dbconfig"
	"github.com/salandered/wavelen/internal/requestid"
	"github.com/salandered/wavelen/internal/server"
	"github.com/salandered/wavelen/internal/storage"
	"github.com/salandered/wavelen/internal/version"

	logging "github.com/salandered/slogenv"
)

var ErrConfig = errors.New("invalid config")

const (
	maxConns        = 25
	maxConnIdleTime = 15 * time.Minute

	pingTimeout = 3 * time.Second // a single attempt
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
	slog.Info("wavelen starting", "version", version.Get())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		slog.Info("shutdown signal received")
		// after the first signal, restore default handling
		// a second Ctrl+C kills immediately (not a graceful shutdown)
		stop()
	}()

	cfg, err := serverConfig()
	if err != nil {
		return err
	}

	// may be a part of the app config later
	colorQuota, err := getColorQuota()
	if err != nil {
		return err
	}

	tokenTTL, err := getAuthTokenTTL()
	if err != nil {
		return err
	}

	// Startup does not wait for the database
	pool, err := openPool(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()
	go logDBReachable(ctx, pool)

	return server.Start(ctx, server.NewHandler(storage.New(pool), colorQuota, tokenTTL), cfg)
}

func openPool(ctx context.Context) (*pgxpool.Pool, error) {
	dsn, err := dbconfig.DSN("postgres")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConfig, err)
	}

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("%w: database: %w", ErrConfig, err)
	}
	poolCfg.MaxConns = maxConns
	poolCfg.MaxConnIdleTime = maxConnIdleTime

	// The effective db config.
	// ConnConfig carries the parsed DSN; the password is never logged.
	slog.Info("database config",
		"host", poolCfg.ConnConfig.Host,
		"port", poolCfg.ConnConfig.Port,
		"database", poolCfg.ConnConfig.Database,
		"user", poolCfg.ConnConfig.User,
		"tls", poolCfg.ConnConfig.TLSConfig != nil,
		"max_conns", poolCfg.MaxConns,
		"max_conn_idle_time", poolCfg.MaxConnIdleTime,
	)

	// NewWithConfig does not dial: MinConns is 0, so the pool is usable with the
	// db down and opens connections on first use.
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("database pool: %w", err)
	}
	return pool, nil
}

// logDBReachable only logs whether the database answers at boot.
func logDBReachable(ctx context.Context, pool *pgxpool.Pool) {
	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		slog.Warn("database not reachable at startup", "error", err)
		return
	}
	slog.Info("database connection pool established")
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

func getColorQuota() (int, error) {
	colorQuota, err := intFromEnv("USER_COLOR_QUOTA", server.DefaultUserColorQuota)
	if err != nil {
		return 0, err
	}
	if colorQuota <= 0 {
		return 0, fmt.Errorf(
			"%w: USER_COLOR_QUOTA should be positive, got %d", ErrConfig, colorQuota)
	}
	return colorQuota, nil
}

func getAuthTokenTTL() (time.Duration, error) {
	ttl, err := durationFromEnv("AUTH_TOKEN_TTL", server.DefaultAuthTokenTTL)
	if err != nil {
		return 0, err
	}
	if ttl <= 0 {
		return 0, fmt.Errorf("%w: AUTH_TOKEN_TTL should be positive, got %s", ErrConfig, ttl)
	}
	return ttl, nil
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
