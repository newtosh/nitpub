// Package version holds the build-time version string, set via
// -ldflags "-X github.com/newtosh/nitpub/internal/version.Version=..." (see
// Makefile). Unset in `go run`/plain `go build`, where it stays "dev".
package version

var Version = "dev"
