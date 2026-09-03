<!-- markdownlint-disable MD031 -->

## Auth

Using stateful bearer tokens.

One wrapper:

| Q              | Wrapper        | Reads                      | Refuses with |
| -------------- | -------------- | -------------------------- | ------------ |
| Who is calling | `authenticate` | the `Authorization` header | 401          |

Every per-user route is `/api/v1/me/...`.

### How db looks

```mermaid
erDiagram
    users ||--o{ tokens : "ON DELETE CASCADE"
    users {
        bigserial id PK
        citext email UK
        bytea password_hash "bcrypt, cost 12"
    }
    tokens {
        bytea hash PK "SHA-256 of the plaintext"
        bigint user_id FK
        timestamptz expiry
    }
```

We don't store what a client sends.
A password is `bcrypt`, a token is `SHA-256`. The plaintext token exists only in the login response. A db leak would not give any tokens that can be used to log in.

A token carries at least 128 bits from `crypto/rand.Text`.

### Login

`POST /api/v1/tokens` is public, the only endpoint that reads `password_hash`.

```mermaid
sequenceDiagram
    participant C as Client
    participant H as TokenHandler
    participant S as authsvc.AuthSvc
    participant DB as database

    C->>H: POST /api/v1/tokens {email, password}
    H->>S: Login(ctx, email, password)
    S->>S: user.NormalizeEmail
    S->>DB: SELECT ... FROM users WHERE email = $1
    alt no such user
        DB-->>S: ErrUserNotFound
        S->>S: auth.EqualizeTiming (a compare that cannot succeed)
        S-->>H: ErrInvalidCredentials
        H-->>C: 401 invalid credentials
    else found
        DB-->>S: user with password_hash
        S->>S: bcrypt compare
        alt wrong password
            S-->>H: ErrInvalidCredentials
            H-->>C: 401 invalid credentials
        else match
            S->>S: auth.NewToken -> plaintext + SHA-256
            S->>DB: INSERT INTO tokens (hash, user_id, expiry)
            S-->>H: token
            H-->>C: 201 {token, expiry}
        end
    end
```

The response is only the token. Info about who is logged in - `GET /api/v1/me`.

An unknown email, invalid email and a wrong password answer with the same status, same body, and spend the same amount of time (see `EqualizeTiming`).
So a client would not know what emails are registered.

### Logout

`DELETE /api/v1/tokens` revokes the token that the this request carries.

Revoking is just deleting the row.

### An authenticated request

The wrapper is applied at the route line (`internal/server/routes.go`), not inside the middleware
chain: endpoints like `/livez` are not wrapped and don't use storage.

```mermaid
flowchart TD
    A["GET /api/v1/me/colors"] --> B{"Authorization: Bearer ...?"}
    B -- absent or malformed --> E1["401, no query made"]
    B -- present --> C["SHA-256 the token"]
    C --> D{"SELECT user_id FROM tokens<br/>WHERE hash = $1 AND expiry > now()"}
    D -- no row --> E2["401, unknown or expired"]
    D -- row --> F["the user id"]
    F --> H["handler runs, the id is one of its arguments"]
```

The wrapper takes an `authedHandlerFunc` (`func(w, r, user.ID)`) and returns a `http.Handler`.

See `internal/server/auth.go`.

### Future

Revoking all of a user's tokens, password change/reset, expired token cleanup.
