## Wavelen

A JSON API for saving colors. Users keep their own list of hex codes.
A seeded palette of 100 named colors is served read-only for a UI table.

Go 1.26, Postgres, pgx v5.

## Running locally

Postgres runs in Docker on host port 5433 (not 5432).

```sh
docker compose up -d

export WAVELEN_DB_DSN='postgres://justuser:justuser@localhost:5433/wavelen?sslmode=disable'
migrate -path ./migrations -database "$WAVELEN_DB_DSN" up

go run ./cmd/api
```

The app reads `WAVELEN_DB_DSN`, `PORT` (default 8080) and `SHUTDOWN_TIMEOUT` (default 10s).

## Logging

Configured from the environment by [slogenv](https://github.com/salandered/slogenv).

Example:

```sh
LOG_LEVEL=debug LOG_FORMAT=json go run ./cmd/api
```

Every request gets a server-generated correlation id, echoed in the `X-Request-Id` response
header. An inbound `X-Request-Id` is ignored.

## Tests

```sh
go test ./...
```

Integration tests need Docker (use a throwaway Postgres via testcontainers and apply all
migrations to it).

```sh
go test -tags integration ./internal/storage/...
```

Set `WAVELEN_TEST_DB_DSN` to reuse the compose database instead of starting a container
(it will drop all the data!).

```sh
export WAVELEN_TEST_DB_DSN='postgres://justuser:justuser@localhost:5433/wavelen?sslmode=disable'
go test -tags integration ./internal/storage/...
```

## API

See [internal/apispec/api.yaml](internal/apispec/api.yaml).

```sh
curl -X POST localhost:8080/api/v1/users -d '{"email":"ada@example.com","name":"Ada"}'
curl -X POST localhost:8080/api/v1/users/1/colors -d '{"hex":"FF00AA"}'
curl localhost:8080/api/v1/users/1/colors
```
