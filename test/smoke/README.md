# Smoke suite

Build-tagged end-to-end tests that run the real `google-cli` command tree
against a real Google account. This is the only suite allowed to touch a real
account, and it is strictly read-only: it invokes just the three read
commands (`account list`, `gmail label list`, `calendar list`). Write
endpoints (label/calendar create, update, delete, acl, message send, ...) are
out of scope and must never be added here.

## Build tags

Every Go file here carries a `smoke` build tag (with an inverse-tagged
`doc.go` so the package still exists untagged), so `go build ./...`, `go test
./...`, and `make test` never compile or run the suite. Without the tag the
package compiles to nothing.

## Running

```sh
go test -tags=smoke ./test/smoke/... -v
```

The suite resolves the config dir exactly like production:
`$GOOGLE_CLI_CONFIG_DIR`, else `~/.config/google-cli`. Without a configured
account, or when credentials exist but the token is expired and its refresh
fails, every test skips (`t.Skip`) — those are environment problems, not test
failures.

To run against a scratch config instead of your real one, populate a
directory with `credentials.json` and `accounts/` (via `google-cli account
add` with `GOOGLE_CLI_CONFIG_DIR` pointing at it), then:

```sh
GOOGLE_CLI_CONFIG_DIR=/path/to/scratch-config go test -tags=smoke ./test/smoke/... -v
```

Note: the read commands persist a refreshed token back to the account file
(production behavior of the dialer), so prefer a scratch config when
experimenting.

## Checks

```sh
gofmt -l test/smoke
go vet -tags=smoke ./test/smoke/...
go test -tags=smoke ./test/smoke/...
```
