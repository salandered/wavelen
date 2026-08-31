# Default version (CI release job passes the computed tag)
VERSION ?= dev
# '-s' - strips symbol tables and DWARF; '-X' for version
LDFLAGS = -s -X github.com/salandered/wavelen/internal/version.version=$(VERSION)

# Holds the database creds and any log settings
-include .env
export

# For psql and the migrate CLI.
# NOTE: defaults as in internal/dbconfig. 
# A password with URL-special characters needs quoting by hand.
DB_URL = postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(or $(POSTGRES_HOST),localhost):$(or $(POSTGRES_PORT),5433)/$(or $(POSTGRES_DB),wavelen)?sslmode=$(or $(POSTGRES_SSLMODE),disable)

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
# TODO: using migrate CLI, could be a version drift from the lib cmd/migrate uses
# Also consider: build/migrate (so build/api no longer covers what the
# image builds) and a run/migrate sibling for run/api

## migrate/new name=$1: create a new migration
.PHONY: migrate/new
migrate/new:
	@echo Creating migration files for ${name}...
	migrate create -seq -ext=.sql -dir=./migrations ${name}

## migrate/up: apply all migrations
.PHONY: migrate/up
migrate/up:
	@echo Running up migrations...
	@migrate -path "./migrations" -database "$(DB_URL)" up

## migrate/down: revert all migrations
.PHONY: migrate/down
migrate/down:
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
	go mod tidy -diff
	go mod verify
	golangci-lint run ./...
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

## build/api: build the cmd/api application for win and linux
.PHONY: build/api
build/api: build/api/win build/api/linux

## build/api/win: build the cmd/api app for Win
# Windows needs "-o=./bin/api.exe", not "-o=./bin/api"
.PHONY: build/api/win
build/api/win:
	@echo Building cmd/api for windows/amd64...
	go build -ldflags='$(LDFLAGS)' -o=./bin/api.exe ./cmd/api

## build/api/linux: build the cmd/api app for Ubuntu Linux
.PHONY: build/api/linux
# A "VAR=value cmd" not working when running make on Win
build/api/linux: export GOOS=linux
build/api/linux: export GOARCH=amd64
build/api/linux: export CGO_ENABLED=0
build/api/linux:
	@echo Building cmd/api for linux/amd64...
	go build -ldflags='$(LDFLAGS)' -o=./bin/linux_amd64/api ./cmd/api


# ==================================================================================== #
# KUBERNETES
# ==================================================================================== #

# Bump means applying the new CRDs first: Helm installs a chart's CRDs once and doesn't upgrade them.
CERT_MANAGER_VERSION ?= v1.21.1

## k8s/addons: install cluster add-ons at pinned versions, then the issuers
.PHONY: k8s/addons
k8s/addons:
	helm repo add jetstack https://charts.jetstack.io
	helm repo update jetstack
	helm upgrade --install cert-manager jetstack/cert-manager \
		--version $(CERT_MANAGER_VERSION) \
		--namespace cert-manager --create-namespace \
		--values deploy/cert-manager-values.yaml \
		--wait
	kubectl apply -f deploy/clusterissuer.yaml

## k8s/secret: create the db secret in the cluster from .env
# --dry-run - kubectl create fails if the object exists
# "$(POSTGRES_USER)" - double quotes, Win does not strip ', so 'value' lands in the Secret
.PHONY: k8s/secret
k8s/secret:
	@kubectl create secret generic wavelen-db \
		--from-literal=POSTGRES_USER="$(POSTGRES_USER)" \
		--from-literal=POSTGRES_PASSWORD="$(POSTGRES_PASSWORD)" \
		--dry-run=client -o yaml | kubectl apply -f -

## k8s/apply/vps: install or upgrade the wavelen release on the VPS k3s cluster
.PHONY: k8s/apply/vps
k8s/apply/vps:
	helm upgrade --install wavelen deploy/wavelen --values deploy/values-vps.yaml

## k8s/apply/k3d: install or upgrade the wavelen release on the local k3d cluster
.PHONY: k8s/apply/k3d
k8s/apply/k3d:
	helm upgrade --install wavelen deploy/wavelen --values deploy/values-k3d-mac.yaml



.PHONY: helm/dev/compare
helm/dev/compare:
	helm get manifest wavelen > current.yaml
	helm template wavelen deploy/wavelen -f deploy/values-vps.yaml > next.yaml

