package dbtrace

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// [Tracer] would log operations which >= these values.
// <= 0 means nothing is logged.
const (
	DefSlowQuery   = 200 * time.Millisecond
	DefSlowAcquire = 100 * time.Millisecond
)

/*
[Tracer] should be assigned to poolCfg.ConnConfig.Tracer.

Query arguments are never logged: they might carry secrets like bcrypt or token hashes.
That is why build-in https://github.com/jackc/pgx/blob/master/tracelog/tracelog.go is not used.

Satisfies tracer interfaces listed below.
*/
type Tracer struct {
	SlowQuery   time.Duration
	SlowAcquire time.Duration
}

// interfaces
var (
	_ pgx.QueryTracer       = (*Tracer)(nil)
	_ pgxpool.AcquireTracer = (*Tracer)(nil)
)

type (
	queryKey   struct{}
	acquireKey struct{}
)

type queryData struct {
	startedAt time.Time
	sql       string
}

/*
[pgx.QueryTracer] docs:

	TraceQueryStart is called at the beginning of Query, QueryRow, and Exec calls.
	The returned context is used for the rest of the call and will be passed to TraceQueryEnd.

ref: https://github.com/jackc/pgx/blob/master/tracelog/tracelog.go#L174
*/
func (t *Tracer) TraceQueryStart(
	ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData,
) context.Context {
	if t.SlowQuery <= 0 {
		return ctx
	}
	return context.WithValue(
		ctx,
		queryKey{},
		queryData{startedAt: time.Now(), sql: data.SQL},
	)
}

/*
TraceQueryEnd measures how long the connection was busy.
For Query it fires from rows.Close, so row iteration and scanning in Go count;
for Exec it fires on the call itself.

Exec can also return before this runs at all, see [Conn.Exec].

ref: https://github.com/jackc/pgx/blob/master/tracelog/tracelog.go#L182
*/

func (t *Tracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	queryData, ok := ctx.Value(queryKey{}).(queryData)
	if !ok {
		return
	}
	interval := time.Since(queryData.startedAt)
	if interval < t.SlowQuery {
		return
	}

	attrs := []slog.Attr{
		slog.Duration("query_duration", interval),
		slog.String("sql", queryData.sql),
	}
	if data.Err != nil {
		attrs = append(attrs, slog.Any("error", data.Err))
	}
	slog.LogAttrs(ctx, slog.LevelWarn, "slow query", attrs...)
}

/*
[pgxpool.AcquireTracer] docs:

	TraceAcquireStart is called at the beginning of Acquire.
	The returned context is used for the rest of the call and will be passed to the TraceAcquireEnd.

ref: https://github.com/jackc/pgx/blob/master/tracelog/tracelog.go#L366
*/
func (t *Tracer) TraceAcquireStart(
	ctx context.Context, _ *pgxpool.Pool, _ pgxpool.TraceAcquireStartData,
) context.Context {
	if t.SlowAcquire <= 0 {
		return ctx
	}
	return context.WithValue(ctx, acquireKey{}, time.Now())
}

/*
[pgxpool.AcquireTracer] docs:

	TraceAcquireEnd is called when a connection has been acquired.

ref: https://github.com/jackc/pgx/blob/master/tracelog/tracelog.go#L372

First request on a cold db might produce a log. See logged stats to distinguish.
*/
func (t *Tracer) TraceAcquireEnd(
	ctx context.Context, pool *pgxpool.Pool, data pgxpool.TraceAcquireEndData,
) {
	startedAt, ok := ctx.Value(acquireKey{}).(time.Time)
	if !ok {
		return
	}
	interval := time.Since(startedAt)
	if interval < t.SlowAcquire {
		return
	}

	stat := pool.Stat()
	attrs := []slog.Attr{
		slog.Duration("acquire_duration", interval),
		slog.Int("acquired_conns", int(stat.AcquiredConns())),
		slog.Int("total_conns", int(stat.TotalConns())),
		slog.Int("max_conns", int(stat.MaxConns())),
		slog.Int64("new_conns", stat.NewConnsCount()),
		slog.Int64("empty_acquires", stat.EmptyAcquireCount()),
	}

	if data.Err != nil {
		attrs = append(attrs, slog.Any("error", data.Err))
	}

	slog.LogAttrs(ctx, slog.LevelWarn, "slow connection acquire", attrs...)
}
