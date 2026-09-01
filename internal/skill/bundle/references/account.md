# account

Manage Google accounts and their cached OAuth tokens. Tokens are stored at
`<config>/accounts/<name>.json` (mode 0600) and never printed.

## account list

List all configured accounts; the default carries `default: true`.

```sh
google-cli account list
google-cli account list --format json
```

## account add <name>

Authorize a new account via OAuth. Flags: `--credentials <path>` (explicit
OAuth client credentials.json), `--scopes <csv>` (empty = Gmail
modify/send/compose + Calendar + Drive + Docs + Sheets + Slides +
userinfo.email).

```sh
google-cli account add work
google-cli account add work --credentials ~/google/credentials.json \
  --scopes https://www.googleapis.com/auth/gmail.send
```

Scope minimization: `--scopes` accepts a comma-separated list for narrower
grants. The default set is full Gmail + Calendar + Drive/Docs/Sheets/Slides +
userinfo.email; with a narrowed grant, Drive/Docs/Sheets/Slides commands fail
fast with the missing scope and a re-consent action instead of returning raw
403s from Google.
## account get <name>

Show account metadata (name, email, scopes, default). Token values are never
printed, in any output format.

```sh
google-cli account get work --format json
```

## account use <name>

Make `<name>` the default account for subsequent commands.

```sh
google-cli account use work
```

## account remove <name> [--force]

Remove an account and its cached token. Refuses without `--force`.

```sh
google-cli account get work
google-cli account remove work --force
```
