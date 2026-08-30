<!-- markdownlint-disable MD031 -->

## Transactional Unit of Work

See https://martinfowler.com/eaaCatalog/unitOfWork.html

### Reference

Read https://rednafi.com/go/repo-txn-uow/

| Article                                                | We                                                   |
| ------------------------------------------------------ | ---------------------------------------------------- |
| `DBTX` interface, satisfied by `*sql.DB` and `*sql.Tx` | `querier`, satisfied by `*pgxpool.Pool` and `pgx.Tx` |
| `Tx(ctx, fn func(Store) error) error` on the store     | `InTx(ctx, fn func(Storage) error) error`            |
| `defer tx.Rollback()` + explicit `Commit`              | `pgx.BeginFunc`                                      |
| Service depends on the store interface, not SQL        | `colorsvc`                                           |

#### Nesting

**Article**
Nesting is guarded via type-asserting `s.db.(*sql.DB)`
Reason: `*sql.Tx` has no `Begin`.

**We**
Same guard, but `pgx.Tx` has `Begin`, so actually a guard can be deleted and nesting will be ok

#### UnitOfWork

**Article**

Uses `UnitOfWork.RunInTx(ctx, fn func(Stores) error)`, because repos are separate structs
in separate packages, each has its own `DBTX`. Transaction cannot span two of them.

**We**

`Storage` is one struct with all repo interfaces
`InTx` is within `Storage`

### The flow (raw)

Storage, Postgres and querier
```go
type querier interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row }
//	Covers: 
// 		*pgxpool.Pool
//		pgx.Tx

type Storage interface {
	UserRepo
	ColorRepo
	// ...
	InTx(ctx context.Context, fn func(Storage) error) error
}

type Postgres struct { db querier } // satisfies Storage. Would be pool or tx

func New(pool *pgxpool.Pool) *Postgres { return &Postgres{db: pool} }

func (s *Postgres) InTx(ctx context.Context, fn func(Storage) error) error {
	if _, ok := s.db.(*pgxpool.Pool); !ok { return ErrNestedTx }
	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		return fn(&Postgres{db: tx}) 	})
}
```

Service method example
```go
func (c *Colors) AddColor(ctx context.Context, userID user.ID, hex color.Hex) {
	err := c.storage.InTx(ctx, 
		func(s storage.Storage) error {
			// inside them calls s.db.Exec; s.db.QueryRow; s.db.Query etc
			s.LockUser(ctx, userID)
			s.CountColors(ctx, userID)
			if ... { s.HasColor(ctx, userID, hex) }
			else { s.AddColor(ctx, userID, hex) }
		},	) }
```

pool.go
```go
// pool *pgxpool.Pool - manages and lands connections [Conn]
pgxpool.Pool 
// Begin acquires a connection from the [Pool] and starts a transaction.
// [*Tx] is returned, which implements the [pgx.Tx] interface.
// [Tx.Commit] or [Tx.Rollback] must be called on the returned transaction to finalize the transaction block.
func (p *Pool) Begin(ctx context.Context) (pgx.Tx, error)
// Exec acquires a connection from the [Pool] and executes the given SQL.
func (p *Pool) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) 

// what it actually returns
func (p *Pool) Begin(ctx context.Context) (pgx.Tx, error)
	// ...
	return &Tx{t: t, c: c}, nil // Tx is a struct from pgxpool.go, embeds tx.go interface Tx as t
// and t is a struct dbTx (tx.go), where dbTx "represents a database transaction."
&dbTx{
	conn:        c,
	commitQuery: txOptions.CommitQuery,
}, nil

// Tx (struct from pgxpool.go) implements Begin
// Begin starts a pseudo nested transaction implemented with a savepoint.
func (tx *Tx) Begin(ctx context.Context) (pgx.Tx, error) {
	return tx.t.Begin(ctx) // tx.dbTx.Begin
}

// dbTx.Begin
// Begin starts a pseudo nested transaction implemented with a savepoint.
func (tx *dbTx) Begin(ctx context.Context) (Tx, error) {
	if tx.closed { return nil, ErrTxClosed }
	tx.savepointNum++
	_, err := tx.conn.Exec(ctx, "savepoint sp_"+strconv.FormatInt(tx.savepointNum, 10))
	if err != nil {	return nil, err	}
	return &dbSimulatedNestedTx{tx: tx, savepointNum: tx.savepointNum}, nil
}
```

tx.go
```go
// calls Begin on 'db' and then calls 'fn'. 
// If fn no err -> calls [Tx.Commit]. If err -> calls [Tx.Rollback] 
func BeginFunc(ctx, db, fn func(Tx) err) {
	var tx Tx = db.Begin() // interface which has Begin(ctx context.Context) (Tx, error)
	err := fn(tx)
	if err
	   tx.Rollback(); return
	tx.Commit() }

// Tx represents a database transaction.
type Tx interface {
	Begin(ctx context.Context) (Tx, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
	
	Exec(ctx context.Context, sql string, arguments ...any) (commandTag pgconn.CommandTag, err error)
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) Row
	//... 	

	Conn() *Conn }
```

pgxpool.go
```go
// Tx represents a database transaction acquired from a Pool.
type Tx struct {
	t pgx.Tx
	c *Conn
}

// Begin starts a pseudo nested transaction implemented with a savepoint.
func (tx *Tx) Begin(ctx context.Context) (pgx.Tx, error) {
	return tx.t.Begin(ctx)
}
```

So this resolves like this

```go
func (c *Colors) AddColor(ctx context.Context, userID user.ID, hex color.Hex) {
	// c.storage is &Postgres{db: pool [querier]}
	// this is a fn which is called between the Begin and Rollback/Commit
	c.storage.InTx(ctx, 
		func(s storage.Storage) error { // s would be &Postgres{db: tx}
			// inside them calls s.db.Exec; s.db.Query etc. Would be like tx.Exec
			s.LockUser(ctx, userID)
			s.CountColors(ctx, userID)
		},	) }

func (s *Postgres) InTx(ctx context.Context, fn func(Storage) error) error {
	// fn is service logic (collection of calls to repos)
	return pgx.BeginFunc(
		ctx,
		s.db, // <---- pool, has Begin (acquires a conn and starts a transaction)
		func(tx pgx.Tx) error {
			return fn(&Postgres{db: tx}) // Postgres.db uses Tx
		},
	)
}
```

```go
func BeginFunc(ctx, pool, fn func(Tx) err) {
	var tx Tx = pool.Begin() // interface which has Begin(ctx context.Context) (Tx, error)
	err := fn(tx) // -> fn(&Postgres{db: tx})() -> call is actually service logic
	if err
	   tx.Rollback(); return
	tx.Commit() }
```
