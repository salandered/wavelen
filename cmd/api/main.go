package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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
	connectTimeout  = 5 * time.Second
)

// Defaults for the the database DSN
const (
	defaultDBHost    = "localhost"
	defaultDBPort    = "5433"
	defaultDBName    = "wavelen"
	defaultDBSSLMode = "disable"
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
	dsn, err := dsnFromEnv()
	if err != nil {
		return nil, err
	}

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("%w: database: %w", ErrConfig, err)
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

// url.URL escapes every part, so the password may contain any character.
func dsnFromEnv() (string, error) {
	user, password := os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD")
	if user == "" || password == "" {
		return "", fmt.Errorf("%w: POSTGRES_USER and POSTGRES_PASSWORD are required", ErrConfig)
	}
	dsn := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host: net.JoinHostPort(
			stringFromEnv("POSTGRES_HOST", defaultDBHost),
			stringFromEnv("POSTGRES_PORT", defaultDBPort),
		),
		Path:     "/" + stringFromEnv("POSTGRES_DB", defaultDBName),
		RawQuery: url.Values{"sslmode": {stringFromEnv("POSTGRES_SSLMODE", defaultDBSSLMode)}}.Encode(),
	}
	return dsn.String(), nil
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

func stringFromEnv(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
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
