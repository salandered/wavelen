<img src="web/images/logo.png" alt="alt text" width="358">

[![CI](https://github.com/salandered/wavelen/actions/workflows/ci.yml/badge.svg)](https://github.com/salandered/wavelen/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/salandered/wavelen/branch/main/graph/badge.svg)](https://codecov.io/gh/salandered/wavelen)
[![Go Version](https://img.shields.io/github/go-mod/go-version/salandered/wavelen)](go.mod)
[![Latest tag](https://img.shields.io/github/v/tag/salandered/wavelen?sort=semver&label=release)](https://github.com/salandered/wavelen/tags)
[![License: PolyForm Shield 1.0.0](https://img.shields.io/badge/License-PolyForm_Shield_1.0.0-blue.svg)](LICENSE)

A web app for saving colors. Users keep their own list of hex codes.

Go 1.27, Postgres, pgx v5.

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

The api binary embeds `web/` and serves it at `/`.

Open http://localhost:8080.

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

The spec - [api/api.yaml](api/api.yaml).
Curl examples - [docs/api.md](docs/api.md).

## Configuration

See [env.template](.env.template). Acts as a config doc as well.

## License

[PolyForm Shield 1.0.0](LICENSE).

Allowed:

- reading, forking and modifying the source
- running your own instance, private or internal to a company
- non-competing commercial use, including inside a paid product
- redistribution, as long as the license text or its URL and the `Required Notice:` line
  travel with it

Not allowed:

- using this code to provide a product or service that competes with wavelen

The list above is a summary. See [LICENSE](LICENSE) for the actual terms.

Versions released before this license was adopted stay under MIT.

Icons keep their own terms, see [web/LICENSE-lucide.txt](web/LICENSE-lucide.txt).
