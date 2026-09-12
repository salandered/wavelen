## Data model

```mermaid
erDiagram
    users ||--o{ tokens : "ON DELETE CASCADE"
    users ||--|{ collections : "ON DELETE CASCADE"
    collections ||--o{ collection_colors : "ON DELETE CASCADE"

    users {
        bigserial id PK
        citext nickname UK "login identifier, case insensitive"
        text name "free-form display name"
        bytea password_hash "bcrypt"
        timestamptz created_at
    }
    tokens {
        bytea hash PK "SHA-256 of the plaintext"
        bigint user_id FK
        timestamptz expiry
    }
    collections {
        uuid id PK "uuidv7"
        bigint user_id FK
        text name "free-form, not unique"
        text icon_slug "icon sprite slug"
        text icon_accent "tint hex"
        boolean is_default "one true row per user"
        timestamptz created_at
    }
    collection_colors {
        uuid collection_id PK,FK
        text hex PK "lowercase rrggbb with a leading hash"
        integer color_key "perceptual sort key, derived from hex"
        timestamptz created_at
    }
```

### Tables

`users` is the account. No email column for now, so there is no account
recovery.

`tokens` is the bearer session. See [auth.md](auth.md).

`collections` is the once collection.

`collection_colors` is a saved color. The key is `(collection_id, hex)` instead of its own id.

### Indexes

| Index                                  | On                                       | Does                                            |
| -------------------------------------- | ---------------------------------------- | ----------------------------------------------- |
| `collections_one_default_per_user`     | `collections (user_id) WHERE is_default` | one default per account, and the default lookup |
| `collection_colors_created_at_hex_idx` | `(collection_id, created_at, hex)`       | `sort=created_at`, keyset paging                |
| `collection_colors_color_key_hex_idx`  | `(collection_id, color_key, hex)`        | `sort=color`, keyset paging                     |

`sort=hex` needs no index: the PK is already a btree on `(collection_id, hex)`.

Both secondary indexes carry `hex` as the tiebreak.
