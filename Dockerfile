# syntax=docker/dockerfile:1

# --platform=$BUILDPLATFORM keeps the compiler on the native runner arch (fast);
# GOARCH=$TARGETARCH cross-compiles to each target - free for CGO_ENABLED=0 Go.
FROM --platform=$BUILDPLATFORM golang:1.26.7-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
# persistent cache layer managed by BuildKit; '/go/pkg/mod' is GOMODCACHE
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# CGO_ENABLED=0 - no libc dependency, runs on `scratch`/distroless
# -ldflags "-s -w" strips both symbol tables and DWARF; -X injects the build-time version
ARG VERSION=dev
ARG TARGETARCH # injected by buildx: amd64 | arm64
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
	-ldflags="-s -w -X github.com/salandered/wavelen/internal/version.version=${VERSION}" \
	-o /out/ ./cmd/api ./cmd/migrate


FROM alpine:3.22
RUN apk add --no-cache ca-certificates

# run as a non-root user
RUN addgroup -S wavelen && adduser -S -G wavelen wavelen
USER wavelen

COPY --from=build /out/api /api
# the migrations Job runs this instead of the entrypoint
COPY --from=build /out/migrate /migrate

# only records the default port for `docker run -P` and inspect tools.
# The listen port is PORT at runtime; compose publishes it explicitly.
EXPOSE 8080

ENTRYPOINT ["/api"]
