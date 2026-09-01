<!-- markdownlint-disable MD031 -->

## Auth

Using stateful bearer tokens.

Two wrappers:

| Q                  | Wrapper        | Reads                      | Refuses with |
| ------------------ | -------------- | -------------------------- | ------------ |
| Who is calling     | `authenticate` | the `Authorization` header | 401          |
| May they have this | `requireOwner` | `{user_id}` in the path    | 403          |

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

An unknown email, invalid email and a wrong password answer with the same status, same body, and spend the same amount of time (see `EqualizeTiming`). So client would not know what emails are registered.

### An authenticated request

The two wrappers are applied at the route line (`internal/server/routes.go`), not inside the middleware chain:

- we don't know the `req.PathValue` until `ServeMux` has matched
- endpoints like `/livez` are not wrapped and don't use storage

```mermaid
flowchart TD
    A["GET /api/v1/users/{user_id}/colors"] --> B{"Authorization: Bearer ...?"}
    B -- absent or malformed --> E1["401, no query made"]
    B -- present --> C["SHA-256 the token"]
    C --> D{"SELECT user_id FROM tokens<br/>WHERE hash = $1 AND expiry > now()"}
    D -- no row --> E2["401, unknown or expired"]
    D -- row --> F["user id into request context"]
    F --> G{"context id == {user_id}?"}
    G -- no --> E3["403, no lookup made"]
    G -- yes --> H["handler runs"]
```

First wrapper validates token, second checks that the permission is allowed. Second one does not use repo, just checks token user_id vs the route path. 403 cannot leak whether the user in the path exists.

See `internal/server/auth.go`.

### Future

Logout, token revocation, password change/reset, expired tokens cleanup
