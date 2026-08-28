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

	pingTimeout           = 3 * time.Second  // a single attempt
	defaultConnectTimeout = 30 * time.Second // total budget
	defaultPingBackoff    = 1 * time.Second  // doubles
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

	pingCfg, err := pingConfigFromEnv()
	if err != nil {
		return err
	}

	pool, err := openPool(ctx, pingCfg)
	if err != nil {
		return err
	}
	defer pool.Close()
	slog.Info("database connection pool established")

	return server.Start(ctx, server.NewHandler(storage.New(pool)), cfg)
}

// how long the app waits for the db at start
type pingConfig struct {
	connectTimeout time.Duration
	backoff        time.Duration
}

func openPool(ctx context.Context, pingCfg pingConfig) (*pgxpool.Pool, error) {
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

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("database pool: %w", err)
	}

	if err := pingWithRetry(ctx, pool, pingCfg); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// pingWithRetry waits for the db to accept queries;
// retries with exp backoff
func pingWithRetry(ctx context.Context, pool *pgxpool.Pool, pingCfg pingConfig) error {
	ctx, cancel := context.WithTimeout(ctx, pingCfg.connectTimeout)
	defer cancel()

	backoff := pingCfg.backoff
	for attempt := 1; ; attempt++ {
		attemptCtx, cancelAttempt := context.WithTimeout(ctx, pingTimeout)
		err := pool.Ping(attemptCtx)
		cancelAttempt()
		if err == nil {
			return nil
		}
		slog.Debug("database not ready", "attempt", attempt, "error", err)

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("database ping after %d attempts: %w", attempt, err)
		case <-timer.C:
		}
		backoff *= 2
	}
}

func pingConfigFromEnv() (pingConfig, error) {
	connectTimeout, err := durationFromEnv("DB_CONNECT_TIMEOUT", defaultConnectTimeout)
	if err != nil {
		return pingConfig{}, err
	}
	backoff, err := durationFromEnv("DB_PING_BACKOFF", defaultPingBackoff)
	if err != nil {
		return pingConfig{}, err
	}
	if connectTimeout <= 0 {
		return pingConfig{}, fmt.Errorf(
			"%w: DB_CONNECT_TIMEOUT should be positive, got %v", ErrConfig, connectTimeout)
	}
	if backoff <= 0 {
		return pingConfig{}, fmt.Errorf(
			"%w: DB_PING_BACKOFF should be positive, got %v", ErrConfig, backoff)
	}
	return pingConfig{connectTimeout: connectTimeout, backoff: backoff}, nil
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
