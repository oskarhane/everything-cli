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

Minimal Drive profile: grant `drive.file` instead of the full `drive` scope to
reach only files this app created or opened — safe against untrusted doc
content turning a share command into account-wide exfiltration. Example:

```sh
google-cli account add work --scopes https://www.googleapis.com/auth/gmail.modify,https://www.googleapis.com/auth/calendar,https://www.googleapis.com/auth/drive.file,https://www.googleapis.com/auth/documents,https://www.googleapis.com/auth/spreadsheets,https://www.googleapis.com/auth/presentations
```

File read/write leaves (list, get, create, upload, download, trash, untrash,
delete) work under `drive.file`; the sharing commands (`drive file share`,
`drive file unshare`, `drive file permissions`) require the full
`https://www.googleapis.com/auth/drive` scope and fail fast with a re-consent
action on a drive.file-only account.

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
