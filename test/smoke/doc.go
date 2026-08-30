//go:build !smoke

// Package smoke holds the build-tagged smoke suite: end-to-end tests that
// run the real command tree against a real Google account, read-only.
//
// Every test file in this package carries a `smoke` build tag, so `go build
// ./...`, `go test ./...`, and `make test` never compile or run it; this doc
// file carries the inverse `!smoke` tag so the package still exists untagged
// and this comment stays reachable. The package compiles to nothing without
// the tag.
//
// Run the suite with:
//
//	go test -tags=smoke ./test/smoke/... -v
//
// Point GOOGLE_CLI_CONFIG_DIR at a scratch config dir to avoid touching the
// real one. Note the read commands persist a refreshed token back to the
// account file (production behavior of the dialer), so a scratch dir is the
// safe way to experiment:
//
//	GOOGLE_CLI_CONFIG_DIR=$(mktemp -d) go test -tags=smoke ./test/smoke/... -v
//
// Without a configured account (or with credentials present but the token
// expired and its refresh failing) every test skips cleanly: those are
// environment problems, not test failures.
package smoke
