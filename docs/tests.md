# Tests

- [Running](#running)
- [Test locations](#test-locations)
- [OpenAPI spec validation](#openapi-spec-validation)
- [Integration tests](#integration-tests)
	- [One container per package](#one-container-per-package)
- [Coverage](#coverage)
- [Testify](#testify)
- [Linters](#linters)
- [Misc](#misc)

## Running

```sh
make lint      # golangci-lint
make test      # go test ./...
make test/all  # includes integration, needs Docker
make audit     # all in one + tidy, etc
```

A single suite method:

```sh
go test -tags=integration -run TestStorageSuite ./internal/storage/ \
  "-testify.m=TestCreateSecondDefaultForOneUser"
```

- `-run` selects the Go entry point (`TestStorageSuite`)
- `-testify.m` selects suite methods by regex (`TestOne|TestTwo` matches several)

## Test locations

- `internal/server/api_test.go` - the API suite. Test every handler over the real router and
  middleware chain, using mocked storage.
- `internal/storage` - repository tests, tagged `integration`.
- `internal/collectionsvc`, `internal/colorsvc`, `internal/usersvc` - service tests, tagged `integration`.
- the rest - plain unit tests

## OpenAPI spec validation

Responses produced in tests are checked against `api/api.yaml`.

- `SetupSuite` loads the embedded spec, calls `spec.Validate`, builds a `gorillamux` router from it.
- The request helpers call `validateAgainstSpec` on every response.
- `IncludeResponseStatus` makes an undeclared status code a failure.
- A path the spec does not describe will pass if the server answered 404.

## Integration tests

Storage runs against a throwaway Postgres via
[testcontainers-go](https://github.com/testcontainers/testcontainers-go).

- `storagetest.Start` boots `postgres:18-alpine` on a random host port
- then applies the migrations from the embedded FS
- `SetupTest` resets the db data after every test.

### One container per package

Every package with an integration suite spawns _its own_ test container.

This is accepted. The image is pulled once and shared, the containers start in parallel, and
migrations are lightweighted.

## Coverage

Both CI jobs write `cover.out` and upload it to Codecov (`unit` and
`integration`). `codecov.yml` waits for both before it reports.

## Testify

Tests use the [testify `suite` package](https://github.com/stretchr/testify) on top of
`testing`.

```
Go test runner
  |
  +-- finds TestAPISuite(t)               <- Go rule
        |
        +-- suite.Run(t, APISuite)        <- testify takes over
              |
              +-- SetupSuite()            once, loads and validates api.yaml
              +-- SetupTest()             per method, fresh mock and httptest server
              +--   SetupSubTest()        per s.Run(...), resets the mock
              +--   TestSomethingIWrote()
              +--   ...
              +-- TearDownTest()          per method, closes the server
```

## Linters

We use [golangci-lint](https://golangci-lint.run). The config is `.golangci.yml`, CI pins the version.

```sh
golangci-lint run ./...  # report
golangci-lint fmt ./...  # format, same as make fmt
```

## Misc

```sh
go fix ./...  # https://go.dev/blog/gofix
```
