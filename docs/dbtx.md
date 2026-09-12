<!-- markdownlint-disable MD031 -->
# Transactional wrapper

- [Where it helps (use cases)](#where-it-helps-use-cases)
	- [Atomic writes](#atomic-writes)
	- [Holding a lock](#holding-a-lock)
	- [One connection](#one-connection)
- [Where it does not](#where-it-does-not)
- [Implementation details](#implementation-details)
	- [Reference](#reference)
	- [The code flow (raw draft)](#the-code-flow-raw-draft)

Implements the Unit of Work idea (https://martinfowler.com/eaaCatalog/unitOfWork.html)

See `storage.Storage.InTx` in the codebase.

## Where it helps (use cases)

### Atomic writes

All writes land or none of them. `pgx.BeginFunc` commits when `fn`
returns nil and rolls back on error.

`usersvc.CreateUser` writes the account and its default collection:
```go
s.CreateUser(ctx, user)
s.CreateCollection(ctx, user.ID, ...)
```
Without the transaction, a failure on the second call leaves an account with no default
collection (business rule violation)

### Holding a lock

Check-then-act using a row lock.

`colorsvc.AddColor` counts against a quota:
```go
owned, _ := s.LockCollection(ctx, userID, collectionID) // SELECT ... FOR UPDATE
n, _ := s.CountColors(ctx, owned)
if n >= c.quota { ... }
s.AddColor(ctx, owned, hex)
```

Without it two concurrent writes may count `n = quota - 1` and both
insert, and the result is len(collection) = quota + 1.

The lock makes the second write wait for the first to commit.

### One connection

Every call inside `fn` runs on the same connection, because `fn` receives a
`&Postgres{db: tx}` and not the pool ("Tx represents a database transaction acquired from a Pool").

In theory it also allows changing an isolation level for a specific transactions.
Note: currently `InTx` calls `pgx.BeginFunc` which uses `BeginTx(ctx, pgx.TxOptions{})`, so the default is hardcoded.
We will need to switch to `pgx.BeginTxFunc` probably.

## Where it does not

Consistent reads.

`InTx` runs at the default READ COMMITTED.
Each statement takes its snapshot: `SELECT`s in one transaction can see two different states of the db.
`BEGIN` and `COMMIT` around two reads is not the same as reading both at one instant.

If something commits between the two reads, their result might be inconsistent.
Can be fixed with a REPEATABLE READ.

## Implementation details

### Reference

Used as a reference: https://rednafi.com/go/repo-txn-uow/

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
`Storage` is one interface embedding all repo interfaces and `InTx`
`Postgres` is the single struct implementing it

### The code flow (raw draft)

How our code and pgx lib work together.

Our code
```go
type querier interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row }
//	Satisfied by: 
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
func (c *ColorSvc) AddColor(
	ctx context.Context, userID user.ID, id collection.ID, hex color.Hex,
) {
	err := c.storage.InTx(ctx, 
		func(s storage.Storage) error {
			// inside them calls s.db.Exec; s.db.QueryRow; s.db.Query etc
			// resolves the collection, proves the caller owns it, locks the row
			owned, _ := s.LockCollection(ctx, userID, id)
			s.CountColors(ctx, owned)
			if ... { s.HasColor(ctx, owned, hex) }
			else { s.AddColor(ctx, owned, hex) }
		},	) }
```

pool.go
[pgxpool] package
```go
// pool *pgxpool.Pool - manages and hands out connections [Conn]
type Pool struct {...}
// Begin acquires a connection from the [Pool] and starts a transaction.
// [*Tx] is returned, which implements the [pgx.Tx] interface.
// [Tx.Commit] or [Tx.Rollback] must be called on the returned transaction to finalize the transaction block.
func (p *Pool) Begin(ctx context.Context) (pgx.Tx, error)
// Exec acquires a connection from the [Pool] and executes the given SQL.
func (p *Pool) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) 

// what it actually returns
func (p *Pool) Begin(ctx context.Context) (pgx.Tx, error)
	// ...
	return &Tx{t: t, c: c}, nil // Tx is a struct from pgxpool/tx.go, embeds tx.go interface Tx as t
// and t is a struct dbTx (tx.go [pgx] package), "represents a database transaction."
&dbTx{
	conn:        c,
	commitQuery: txOptions.CommitQuery,
}, nil

// where Tx (struct from pgxpool.go) implements Begin
// Begin starts a pseudo nested transaction implemented with a savepoint.
func (tx *Tx) Begin(ctx context.Context) (pgx.Tx, error) {
	return tx.t.Begin(ctx) // tx.dbTx.Begin
}

// and dbTx.Begin
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
[pgx] package
```go
// calls Begin on 'db' and then calls 'fn'. 
// If fn no err -> calls [Tx.Commit]. If err -> calls [Tx.Rollback] 
func BeginFunc(ctx, db, fn func(Tx) err) {
	var tx Tx = db.Begin() // db here is actually an interface which has Begin(ctx context.Context) (Tx, error)
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

pgxpool/tx.go
[pgxpool] package
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

So this resolves as

```go
func (c *ColorSvc) AddColor(ctx, userID, cltID, ...) {
	// c.storage is &Postgres{db: pool <*pgxpool.Pool> interface [querier]}
	c.storage.InTx(ctx, 
		// fn which is called between the Begin and Rollback/Commit
		func(s storage.Storage) error { // s would be &Postgres{db: tx}
			// Repo methods call s.db.Exec; s.db.Query etc. Would be like tx.Exec
			owned, _ := s.LockCollection(ctx, userID, cltID)
			s.CountColors(ctx, owned)
		},	
	) }

func (s *Postgres) InTx(ctx, fn func(Storage) error) error {
	// fn is service logic (collection of calls to repos)
	return pgx.BeginFunc(
		ctx,
		s.db, // <---- pool <*pgxpool.Pool>
		func(tx pgx.Tx) error {
			return fn(&Postgres{db: tx}) // Postgres.db uses Tx
		},
	)
}
```

```go
// pool <*pgxpool.Pool> has Begin (acquires a connection from the [Pool] and starts a transaction)
func BeginFunc(ctx, pool, fn func(Tx) err) {
	// tx is pgxpool/tx.go struct (represents a database transaction acquired from a Pool.), returned as a pgx.Tx interface
	var tx pgx.Tx = pool.Begin() 
	err := fn(tx) // -> fn(&Postgres{db: tx})() -> call is actually service logic
	if err {  tx.Rollback(); return }
	tx.Commit() }
```
