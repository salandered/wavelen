package dbtrace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/salandered/wavelen/internal/requestid"
	"github.com/stretchr/testify/require"

	"github.com/salandered/slogenv"
)

const (
	anyCallFast = time.Hour // no call is ever this slow
	anyCallSlow = time.Nanosecond

	// rewind the start time to make a call count as slow
	elapsed = time.Second
)

func TestQueryUnderThresholdIsNotLogged(t *testing.T) {
	tr := &Tracer{SlowQuery: anyCallFast}

	logged := captureLogs(t, func() {
		ctx, trOn := traceQueryStartRewind(tr, context.Background(), pgx.TraceQueryStartData{SQL: `SELECT 1`})
		require.True(t, trOn)
		tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})
	})

	require.Empty(t, logged)
}

func TestZeroQueryThresholdLogsNothing(t *testing.T) {
	tr := &Tracer{}

	logged := captureLogs(t, func() {
		ctx, trOn := traceQueryStartRewind(tr, context.Background(), pgx.TraceQueryStartData{SQL: `SELECT 1`})
		require.False(t, trOn)
		tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})
	})

	require.Empty(t, logged)
}

// TraceQueryEnd may run without a start on the Exec path that returns early.
func TestQueryEndWithoutStartLogsNothing(t *testing.T) {
	tr := &Tracer{SlowQuery: anyCallSlow}

	logged := captureLogs(t, func() {
		tr.TraceQueryEnd(context.Background(), nil, pgx.TraceQueryEndData{})
	})

	require.Empty(t, logged)
}

func TestSlowQueryIsLoggedWithSQLAndDuration(t *testing.T) {
	tr := &Tracer{SlowQuery: anyCallSlow}

	logged := captureLogs(t, func() {
		ctx, _ := traceQueryStartRewind(tr, context.Background(), pgx.TraceQueryStartData{SQL: `SELECT 1`})
		tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})
	})

	require.Len(t, logged, 1)
	require.Equal(t, "slow query", logged[0]["msg"])
	require.Equal(t, "WARN", logged[0]["level"])
	require.Equal(t, `SELECT 1`, logged[0]["sql"])
	// time.Since adds the gap
	require.GreaterOrEqual(t, logged[0]["query_duration"], float64(elapsed))
	require.NotContains(t, logged[0], "error")
}

func TestSlowQueryNeverLogsArgs(t *testing.T) {
	tr := &Tracer{SlowQuery: anyCallSlow}

	logged := captureLogs(t, func() {
		ctx, _ := traceQueryStartRewind(tr, context.Background(), pgx.TraceQueryStartData{
			SQL:  `INSERT INTO users (nickname, password_hash) VALUES ($1, $2)`,
			Args: []any{"bob", "$2a$12$secret"},
		})
		tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})
	})

	require.Len(t, logged, 1)
	require.NotContains(t, mustJSON(t, logged[0]), "secret")
	require.NotContains(t, mustJSON(t, logged[0]), "bob")
}

func TestSlowQueryCarriesError(t *testing.T) {
	tr := &Tracer{SlowQuery: anyCallSlow}

	logged := captureLogs(t, func() {
		ctx, _ := traceQueryStartRewind(tr, context.Background(), pgx.TraceQueryStartData{SQL: `SELECT 1`})
		tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: errors.New("timeout")})
	})

	require.Len(t, logged, 1)
	require.Equal(t, "timeout", logged[0]["error"])
}

func TestSlowQueryCarriesRequestID(t *testing.T) {
	tr := &Tracer{SlowQuery: anyCallSlow}

	logged := captureLogs(t, func() {
		ctx := requestid.NewContext(context.Background(), "req-42")
		ctx, _ = traceQueryStartRewind(tr, ctx, pgx.TraceQueryStartData{SQL: `SELECT 1`})
		tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})
	})

	require.Len(t, logged, 1)
	require.Equal(t, "req-42", logged[0]["request_id"])
}

func TestZeroAcquireThresholdStoresNoStart(t *testing.T) {
	tr := &Tracer{}

	ctx := tr.TraceAcquireStart(context.Background(), nil, pgxpool.TraceAcquireStartData{})

	require.Nil(t, ctx.Value(acquireKey{}))
}

func TestAcquireUnderThresholdIsNotLogged(t *testing.T) {
	tr := &Tracer{SlowAcquire: anyCallFast}

	logged := captureLogs(t, func() {
		ctx := traceAcquireStartRewind(tr, context.Background())
		tr.TraceAcquireEnd(ctx, nil, pgxpool.TraceAcquireEndData{})
	})

	require.Empty(t, logged)
}

func TestAcquireEndWithoutStartLogsNothing(t *testing.T) {
	tr := &Tracer{SlowAcquire: anyCallSlow}

	logged := captureLogs(t, func() {
		tr.TraceAcquireEnd(context.Background(), nil, pgxpool.TraceAcquireEndData{})
	})

	require.Empty(t, logged)
}

/*
traceQueryStartRewind wraps the tr.TraceQueryStart,
rewinding the start time it returned by 'elapsed'.
If tr.TraceQueryStart didn't register the start time at all, returns false.
The rewind also puts the logged duration close to 'elapsed'.
*/
func traceQueryStartRewind(tr *Tracer, ctx context.Context, data pgx.TraceQueryStartData) (context.Context, bool) {
	ctx = tr.TraceQueryStart(ctx, nil, data)

	origQueryData, ok := ctx.Value(queryKey{}).(queryData)
	if !ok {
		return ctx, false // nothing was added to ctx
	}
	origQueryData.startedAt = origQueryData.startedAt.Add(-elapsed)
	return context.WithValue(ctx, queryKey{}, origQueryData), true
}

// Same rewind as traceQueryStartRewind.
func traceAcquireStartRewind(tr *Tracer, ctx context.Context) context.Context {
	ctx = tr.TraceAcquireStart(ctx, nil, pgxpool.TraceAcquireStartData{})

	startedAt, ok := ctx.Value(acquireKey{}).(time.Time)
	if !ok {
		return ctx
	}
	return context.WithValue(ctx, acquireKey{}, startedAt.Add(-elapsed))
}

func mustJSON(t *testing.T, record map[string]any) string {
	t.Helper()

	out, err := json.Marshal(record)
	require.NoError(t, err)
	return string(out)
}

// Same shape as captureLogs in internal/server.
func captureLogs(t *testing.T, fn func()) []map[string]any {
	t.Helper()

	var buf bytes.Buffer
	h := slogenv.NewContextHandler(
		slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}),
		requestid.LogAttrs,
	)
	restore := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(restore) })

	fn()

	var out []map[string]any
	for line := range bytes.Lines(bytes.TrimSpace(buf.Bytes())) {
		var record map[string]any
		require.NoError(t, json.Unmarshal(line, &record))
		out = append(out, record)
	}
	return out
}
