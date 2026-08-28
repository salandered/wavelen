## Wavelen

A JSON API for saving colors. Users keep their own list of hex codes.
A seeded palette of 100 named colors is served read-only for a UI table.

Go 1.26, Postgres, pgx v5.

## Running locally

Config comes from `.env`. Copy the template and fill in `POSTGRES_USER` and
`POSTGRES_PASSWORD`

```sh
cp .env.template .env
```

```sh
make db/up # starts db in docker
make migrate/up
make run/api # run app
```

`make help` lists the other targets.

## Running in Docker

```sh
make dc/up # docker compose stack - app and db
make migrate/up

make dc/down # stop the stack
```

## Web UI

Optional, nothing needs it.

```sh
make web/up   # the stack plus a caddy container
make web/down
```

Open <http://localhost:8089>.

## Logging

Configured from the environment by [slogenv](https://github.com/salandered/slogenv).
use `.env`.

Every request gets a server-generated correlation id, echoed in the `X-Request-Id` response
header. An inbound `X-Request-Id` is ignored.

## Tests

```sh
go test ./...
```

Integration tests need Docker. They start a throwaway Postgres via testcontainers and
apply all migrations.

```sh
go test -tags integration ./internal/storage/...
```

## Versioning and CI

```sh
make build/api VERSION=0.2.0
VERSION=0.2.0 docker compose build   # the compose default is dev
```

## API

See [api/api.yaml](api/api.yaml).

```sh
curl -X POST localhost:8080/api/v1/users -d '{"email":"ada@example.com","name":"Ada"}'
curl -X POST localhost:8080/api/v1/users/1/colors -d '{"hex":"FF00AA"}'
curl localhost:8080/api/v1/users/1/colors
curl localhost:8080/api/v1/colors
curl 'localhost:8080/api/v1/colors?sort=hex&order=desc'
```
