// Package version holds the build-time version string.
package version

// Overridden at build time via -ldflags "-X ...internal/version.version=...".
// Defaults to "dev" for plain `go run`/`go build`.
var version = "dev"

func Get() string {
	return version
}
