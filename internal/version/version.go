// Package version holds the build-time version string.
package version

// Dev is the version of a build with no injection: plain `go run`/`go build`.
const Dev = "dev"

// Overridden at build time via -ldflags "-X ...internal/version.version=...".
var version = Dev

func Get() string {
	return version
}
