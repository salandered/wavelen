# Injected into the binary as version.version. The CI release job passes the computed tag;
# a plain `make build/api` leaves it at the Go default.
VERSION ?= dev
LDFLAGS = -s -X github.com/salandered/wavelen/internal/version.version=$(VERSION)

# Holds the database creds and any log settings
-include .env
export

# For psql and the migrate CLI.
# NOTE: Keep the defaults in sync with internal/dbconfig. 
# A password with URL-special characters needs quoting by hand.
DB_URL = postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(or $(POSTGRES_HOST),localhost):$(or $(POSTGRES_PORT),5433)/$(or $(POSTGRES_DB),wavelen)?sslmode=$(or $(POSTGRES_SSLMODE),disable)

# Prints every "## target: description" comment in this file.
.PHONY: help
help:
	@echo "see the file :D"

# ==================================================================================== #
# DEVELOPMENT
# ==================================================================================== #

## run/api: run the cmd/api application
.PHONY: run/api
run/api:
	go run ./cmd/api

## dc/up: build and run the stack in docker
.PHONY: dc/up
dc/up:
	docker compose up -d --build

## dc/down: stop the stack
.PHONY: dc/down
dc/down:
	docker compose down

## web/up: run the stack + web web
.PHONY: web/up
web/up:
	docker compose -f docker-compose.yml -f docker-compose.web.yml up -d --build

## web/down: stop the stack + web
.PHONY: web/down
web/down:
	docker compose -f docker-compose.yml -f docker-compose.web.yml down

## db/up: start only the postgres container
.PHONY: db/up
db/up:
	docker compose up -d postgres

## db/psql: connect to the database using psql
.PHONY: db/psql
db/psql:
	@psql "$(DB_URL)"

# ==================================================================================== #
# MIGRATIONS
# ==================================================================================== #

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

## audit: all - tidy, lints, tests
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
# -ldflags carries '-s' (strips both symbol tables and DWARF) and the version -X
# build/api runs both: the local (windows/amd64) build and the Ubuntu Linux one.
# Windows needs "-o=./bin/api.exe", not "-o=./bin/api"
## build/api: build the cmd/api application for both platforms
.PHONY: build/api
build/api: build/api/local build/api/linux

## build/api/local: build the cmd/api application for this machine
.PHONY: build/api/local
build/api/local:
	@echo Building cmd/api for windows/amd64...
	go build -ldflags='$(LDFLAGS)' -o=./bin/api.exe ./cmd/api

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
	go build -ldflags='$(LDFLAGS)' -o=./bin/linux_amd64/api ./cmd/api


# ==================================================================================== #
# KUBERNETES
# ==================================================================================== #

## k8s/secret: create the db secret in the cluster from .env
# --dry-run ... - kubectl create fails if the object exists
.PHONY: k8s/secret
k8s/secret:
	@kubectl create secret generic wavelen-db \
		--from-literal=POSTGRES_USER='$(POSTGRES_USER)' \
		--from-literal=POSTGRES_PASSWORD='$(POSTGRES_PASSWORD)' \
		--dry-run=client -o yaml | kubectl apply -f -

## k8s/up: apply manifests from k8s/
.PHONY: k8s/up
k8s/up:
	kubectl apply -f k8s/