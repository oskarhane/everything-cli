# linear

The `linear` provider: Linear issues, teams, and projects — plus Linear
account management via personal API key or OAuth. Command layout:

```sh
everything-cli linear <resource> <action>
```

All Linear commands talk to the single GraphQL endpoint
`https://api.linear.app/graphql` (one POST per operation).

## Auth (two methods)

`linear account add <name>` onboards with one of two credential types;
the stored account's shape then decides which client every later command
uses. Both variants land at `<config>/accounts/linear/<name>.json`
(mode 0600), and every secret (API key, OAuth client secret, access and
refresh tokens) is registered for redaction at capture/read time — it is
never printed, in any output format, including `--debug`.

### Personal API key (default)

Create a key at Linear → Settings → Account → Security
(`https://linear.app/settings/account/security`). It is sent raw in the
`Authorization` header — no `Bearer` prefix. Capture order at
`account add` time:

1. `--api-key <key>` flag
2. `$LINEAR_API_KEY` environment variable
3. a hidden interactive prompt

Use this method for personal access and scripts — it is the simplest
path and needs no app registration. Prefer the env var over `--api-key`
in scripts so the key stays out of shell history.

### OAuth (`--oauth`)

Browser authorization-code flow with PKCE and a loopback redirect.
Requires a Linear OAuth application (created at
`https://linear.app/settings/api/applications`) with a localhost
redirect URI configured. Flags:

- `--client-id <id>` — OAuth app client ID (empty = `$LINEAR_CLIENT_ID`;
  required one way or the other)
- `--client-secret <secret>` — OAuth app client secret (empty =
  `$LINEAR_CLIENT_SECRET`); optional under PKCE, which the flow uses

`--api-key` and `--oauth` are mutually exclusive. The flow requests the
`read,write` scopes (read covers teams/projects/issues reads; write
covers issue create/update) and resolves the account identity through
Linear's GraphQL `viewer` query. Access tokens live 24 hours; the CLI
refreshes them automatically and persists refreshed tokens back to the
account file. Use this method when an organization mandates OAuth app
identity or managed client credentials.

## account

Manage Linear accounts and their stored credentials.

- `linear account add <name>` — onboard an account. Flags: `--api-key`
  (default method; empty = `$LINEAR_API_KEY`, then a hidden prompt),
  `--oauth` (browser flow with PKCE instead), `--client-id` and
  `--client-secret` (`--oauth` only; env fallbacks `$LINEAR_CLIENT_ID` /
  `$LINEAR_CLIENT_SECRET`). Output: `name`, `provider` — the key is
  deliberately absent so no format can leak it.
- `linear account list` — all configured Linear accounts. Fields:
  `name`, `default` (`true` on the default account in JSON/TOON; a
  `(default)` marker in the table).
- `linear account get <name>` — account metadata (`name`, `provider`).
  The credential is never printed, in any output format.
- `linear account use <name>` — make `<name>` the default Linear
  account. Every command also accepts the global `--account <name>`
  override.
- `linear account remove <name> [--force]` — remove an account and its
  stored credential. Refuses without `--force`.

```sh
everything-cli linear account add work                                  # hidden prompt
LINEAR_API_KEY=lin_api_... everything-cli linear account add work
everything-cli linear account add work --api-key lin_api_...
everything-cli linear account add work --oauth --client-id 7231...
everything-cli linear account list --format json
everything-cli linear account get work --format json
everything-cli linear account use work
everything-cli linear account remove old --force
```

## issue

Manage Linear issues. The `<id>` argument of `get`/`update` accepts
either the issue's UUID or its human identifier (`BLA-123`).

### issue list

- `linear issue list` — list issues, most recently updated first.
  Flags: `--team <team-id>` (team UUID; empty = workspace-wide). All
  pages are followed automatically (see Pagination — there is no
  `--max`).

Output: one row per issue. JSON/TOON fields: `id`, `identifier`,
`title`, `description`, `state` (`{id, name}`), `assignee` (`{id,
name}`), `team` (`{id, name, key}`), `url`, `created_at`, `updated_at`.
Table columns: `identifier`, `title`, `state`, `assignee`, `team`
(the team key), `updated_at` — reference cells render as display names.

```sh
everything-cli linear issue list --format json
everything-cli linear issue list --team 9c1e2f3a-... --format table
```

### issue get

- `linear issue get <id>` — show one issue by UUID or human identifier
  (`BLA-123`). Same JSON fields as list; the table adds `id`,
  `description`, `url`, and `created_at` to the column set.

```sh
everything-cli linear issue get BLA-123 --format json
everything-cli linear issue get 8b9c0d1e-... --format table
```

### issue create

- `linear issue create` — create an issue in a team. Flags: `--team
  <team-id>` (required, UUID), `--title <text>` (required — the CLI
  demands it even though the API marks it nullable), `--description
  <markdown>`, `--assignee <user-id>` (UUID), `--state <state-id>`
  (workflow state UUID). With no `--state`, the issue lands in the
  team's first Backlog state (or Triage, if the team has it enabled).
  Echoes the created issue with the `issue get` field set — capture
  `identifier` and `url` from the JSON.

```sh
everything-cli linear issue create --team 9c1e2f3a-... --title "Fix login redirect"
everything-cli linear issue create --team 9c1e2f3a-... --title "Fix login redirect" \
  --description "Users land on / after logout" --assignee 4d5e6f7a-... --state 8b9c0d1e-...
everything-cli linear issue create --team 9c1e2f3a-... --title "Follow up" --format json
```

### issue update

- `linear issue update <id>` — update one issue (UUID or `BLA-123`).
  Flags: `--title`, `--description` (markdown), `--assignee <user-id>`,
  `--state <state-id>`. Only the flags given are sent — omitted fields
  are untouched. Echoes the updated issue.

```sh
everything-cli linear issue update BLA-123 --state 8b9c0d1e-...
everything-cli linear issue update BLA-123 --title "Fix login redirect (regression)" \
  --assignee 4d5e6f7a-...
```

## team

- `linear team list` — list every team in the workspace. Fields: `id`
  (the UUID `--team` flags want), `name`, `key` (the issue-identifier
  prefix, e.g. `BLA`).

```sh
everything-cli linear team list --format json
everything-cli linear team list --format table
```

## project

- `linear project list` — list every project in the workspace. Fields:
  `id`, `name`, `description`, `state`.

```sh
everything-cli linear project list --format json
everything-cli linear project list --format table
```

## Pagination

Linear uses Relay-style cursor pagination (`first`/`after`,
`pageInfo { hasNextPage endCursor }`). The CLI requests 50 items per
call and follows cursors automatically until the listing is exhausted,
so `issue list`, `team list`, and `project list` always return the full
result set — there is no `--max` flag on linear commands (unlike the
google provider). A runaway-cursor guard stops a listing after 1000
pages rather than looping forever. Archived resources are excluded by
the API default.

## Rate limits

Linear rate-limits with a leaky bucket per hour (from Linear's docs):

| Credential | Requests/hour | Complexity points/hour |
| --- | --- | --- |
| Personal API key | 2,500 per user | 3,000,000 |
| OAuth app | 5,000 per user | 2,000,000 |

A single query may not exceed 10,000 complexity points. Responses carry
`X-RateLimit-Requests-*` and `X-RateLimit-Complexity-*` headers. When
the limit is hit Linear answers HTTP 400 with a GraphQL error whose
extension code is `RATELIMITED`; the CLI surfaces it as an error (e.g.
`linear API error: <message> (RATELIMITED)`) — there is no automatic
retry, so wait for the bucket to refill and rerun. Workspace-wide
`issue list` costs one request per 50 issues; prefer `--team` scoping on
large workspaces.

## Tips & gotchas (linear)

- Team key vs team ID: `BLA-123` is keyed by the team's human key, but
  `--team` takes the team UUID. Run `linear team list` to map key →
  `id` before scoping or creating.
- `issue get`/`issue update` accept both the UUID and the `BLA-123`
  human identifier; `issue create --team`, `--assignee`, and `--state`
  take UUIDs only. Discover state and assignee UUIDs from an existing
  issue's JSON (`issue get BLA-123 --format json` → `state.id`,
  `assignee.id`); there is no state- or user-listing command.
- `--title` is required on `issue create` even though the API allows
  untitled issues.
- `issue create`/`issue update` echo the full issue; use `--format
  json` to capture the new `identifier`/`url` programmatically.
- There are no delete/trash verbs on linear resources — move issues
  through workflow states instead (`issue update <id> --state
  <completed-state-id>`).
- `linear account remove` refuses without `--force`.
