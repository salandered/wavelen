# Holds the database creds and any log settings
-include .env
export

# For psql and the migrate CLI.
# Keep the defaults in sync with cmd/api/main.go. 
# A password with URL-special characters needs quoting by hand.
DB_URL = postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(or $(POSTGRES_HOST),localhost):$(or $(POSTGRES_PORT),5433)/$(or $(POSTGRES_DB),wavelen)?sslmode=$(or $(POSTGRES_SSLMODE),disable)

# Prints every "## target: description" comment in this file.
.PHONY: help
help:
	@awk '/^## /{sub(/^## /,""); i=index($$0,": "); printf "  %-26s %s\n", substr($$0,1,i-1), substr($$0,i+2)}' $(MAKEFILE_LIST)

# ==================================================================================== #
# DEVELOPMENT
# ==================================================================================== #

## run/api: run the cmd/api application
.PHONY: run/api
run/api:
	go run ./cmd/api

## db/up: start the postgres container
.PHONY: db/up
db/up:
	docker compose up -d

## db/down: stop the postgres container
.PHONY: db/down
db/down:
	docker compose down

## db/psql: connect to the database using psql
.PHONY: db/psql
db/psql:
	@psql "$(DB_URL)"

## db/migrations/new name=$1: create a new migration
.PHONY: db/migrations/new
db/migrations/new:
	@echo Creating migration files for ${name}...
	migrate create -seq -ext=.sql -dir=./migrations ${name}

## db/migrations/up: apply all migrations
.PHONY: db/migrations/up
db/migrations/up:
	@echo Running up migrations...
	@migrate -path "./migrations" -database "$(DB_URL)" up

## db/migrations/down: revert all migrations
.PHONY: db/migrations/down
db/migrations/down:
	@echo Reverting all migrations...
	@migrate -path "./migrations" -database "$(DB_URL)" down

# ==================================================================================== #
# QUALITY CONTROL
# ==================================================================================== #

## tidy: tidy module dependencies
.PHONY: tidy
tidy:
	@echo Tidying module dependencies...
	go mod tidy

## lint: report lint issues
.PHONY: lint
lint:
	golangci-lint run ./...

## fmt: format all .go files
.PHONY: fmt
fmt:
	golangci-lint fmt ./...

## audit: run quality control checks
.PHONY: audit
audit:
	@echo Checking module dependencies...
	go mod tidy -diff
	go mod verify
	@echo Linting...
	golangci-lint run ./...
	@echo Running tests...
	go test -race -vet=off ./...

# ==================================================================================== #
# TESTS
# ==================================================================================== #

## test: unit tests
.PHONY: test
test:
	go test ./...

## test/all: unit and integration tests, needs Docker
.PHONY: test/all
test/all:
	go test -race -tags=integration ./...

# ==================================================================================== #
# BUILD
# ==================================================================================== #
# -ldflags='-s' strips both symbol tables and DWARF
# build/api runs both: the local (windows/amd64) build and the Ubuntu Linux one.
# Windows needs "-o=./bin/api.exe", not "-o=./bin/api"
## build/api: build the cmd/api application for both platforms
.PHONY: build/api
build/api: build/api/local build/api/linux

## build/api/local: build the cmd/api application for this machine
.PHONY: build/api/local
build/api/local:
	@echo Building cmd/api for windows/amd64...
	go build -ldflags='-s' -o=./bin/api.exe ./cmd/api

## build/api/linux: build the cmd/api application for Ubuntu Linux
# GOOS/GOARCH come from a target-specific export: make puts them into the child
# environment itself. A "VAR=value cmd" prefix would not work, because make runs
# recipes through cmd.exe on Windows and that form is POSIX shell syntax.
# CGO_ENABLED=0 because this machine has CGO_ENABLED=1 in go env: cross-compiling would
# otherwise call the local mingw gcc against linux headers it does not have.
.PHONY: build/api/linux
build/api/linux: export GOOS=linux
build/api/linux: export GOARCH=amd64
build/api/linux: export CGO_ENABLED=0
build/api/linux:
	@echo Building cmd/api for linux/amd64...
	go build -ldflags='-s' -o=./bin/linux_amd64/api ./cmd/api
