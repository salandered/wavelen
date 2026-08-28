// DEV DEAD CODE
// pingWithRetry used to gate startup: run() blocked
// until the db is ok. Don't want it to race with a live probe.
// The pool connects lazily and /readyz reports the real state.
//
// It needs DB_CONNECT_TIMEOUT and DB_PING_BACKOFF;

// # Total budget for reaching the db at startup
// # default: 30s
// DB_CONNECT_TIMEOUT=

// # Wait between connection attempts, doubled after every failure
// # default: 1s
// DB_PING_BACKOFF=
//
//nolint:unused
package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultConnectTimeout = 30 * time.Second // total budget
	defaultPingBackoff    = 1 * time.Second  // doubles
)

// how long the app waits for the db at start
type pingConfig struct {
	connectTimeout time.Duration
	backoff        time.Duration
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
