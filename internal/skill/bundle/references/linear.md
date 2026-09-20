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

Both flags are `--oauth`-only: passing either without `--oauth` fails
fast with an error naming `--oauth`. `--api-key` and `--oauth` are
mutually exclusive. The flow requests the
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
  `--client-secret` (`--oauth` only — rejected with an error naming
  `--oauth` when passed without it; env fallbacks `$LINEAR_CLIENT_ID` /
  `$LINEAR_CLIENT_SECRET`). Output: `name`, `provider` — the key is
  deliberately absent so no format can leak it.
- `linear account auth <name>` — re-run the OAuth flow for an existing
  OAuth account, using the client credentials already stored in the
  account (no flags or env needed). Flag: `--scopes <csv>` (empty = the
  account's current scopes). Output: `name`, `provider`. OAuth accounts
  only — an API-key account errors with guidance to re-create it via
  `linear account add`; an unknown account errors pointing at `linear
  account add`; an identity mismatch is a hard error and nothing is
  saved.
- `linear account list` — all configured Linear accounts. Fields:
  `name`, `default` (`true` on the default account in JSON/TOON; a
  `(default)` marker in the table).
- `linear account get <name>` — account metadata (`name`, `provider`).
  The credential is never printed, in any output format.
- `linear account use <name>` — make `<name>` the default Linear
  account. Every command also accepts the global `--account <name>`
  override.
- `linear account whoami` — the identity the acting account (the
  default, or the `--account <name>` override) authenticates as,
  resolved through Linear's GraphQL `viewer` query. Fields: `id`,
  `name`, `email` — useful to confirm which credential a run will use,
  and to grab your own user UUID for `--assignee me`-style filters.
- `linear account remove <name> [--force]` — remove an account and its
  stored credential. Refuses without `--force`. Removing the default
  promotes another Linear account and announces the new default.

```sh
everything-cli linear account add work                                  # hidden prompt
LINEAR_API_KEY=lin_api_... everything-cli linear account add work
everything-cli linear account add work --api-key lin_api_...
everything-cli linear account add work --oauth --client-id 7231...
everything-cli linear account auth work                       # OAuth accounts only
everything-cli linear account list --format json
everything-cli linear account get work --format json
everything-cli linear account use work
everything-cli linear account whoami
everything-cli linear account whoami --account work --format table
everything-cli linear account remove old --force
```

## issue

Manage Linear issues. The `<id>` argument of `get`/`update` accepts
either the issue's UUID or its human identifier (`BLA-123`).

### issue list

- `linear issue list` — list issues, most recently updated first.
  Scoping is server-side: every flag below composes into ONE
  `issues(filter: ...)` query (see Pagination), so a filtered pull
  neither requests nor pages the issues it excludes. All pages are
  followed automatically (there is no `--max`). Flags:

  - `--team <team-id>` — team UUID; empty = workspace-wide.
  - `--assignee <user-id|me>` — assignee user UUID, or `me` for the
    current account (`me` resolves through one GraphQL `viewer` call
    shared by `--assignee` and `--created-by` for the whole run);
    empty = any assignee.
  - `--created-by <user-id|me>` — creator user UUID, or `me` as
    above; empty = any creator.
  - `--updated-since <ts>` — only issues updated since `ts`. Accepted
    forms (same as google calendar's timestamp flags): RFC3339
    (`2026-09-03T14:00:00Z`), naive RFC3339
    (`2026-09-03T14:00:00` — read in the local timezone), a date
    (`2026-09-03`, local midnight), or relative offsets anchored at
    now (`now`, `-1d`, `+7d`, `-30m`, `+2h`; bare `7d` counts
    forward). Empty = no filter.

Output: one row per issue. JSON/TOON fields: `id`, `identifier`,
`title`, `description`, `state` (`{id, name, type}` — `type` is the
workflow-state kind: unstarted/started/completed/canceled),
`assignee` (`{id, name}`), `creator` (`{id, name}` — omitted when
null), `team` (`{id, name, key}`), `url`, `created_at`, `updated_at`,
plus `started_at`, `completed_at`, `canceled_at` (each omitted when
empty). Table columns: `identifier`, `title`, `state`, `assignee`,
`team` (the team key), `updated_at` — reference cells render as
display names.

```sh
everything-cli linear issue list --format json
everything-cli linear issue list --team 9c1e2f3a-... --format table
everything-cli linear issue list --assignee me --updated-since -1d --format json

# change-detection pull: everything touched since a stored watermark
everything-cli linear issue list --updated-since -7d --format json
everything-cli linear issue list --updated-since 2026-09-02T00:00:00Z --format json
```

### issue get

- `linear issue get <id>` — show one issue by UUID or human identifier
  (`BLA-123`). Same JSON fields as list (state with `type`, `creator`,
  `started_at`, `completed_at`, `canceled_at` included, with the same
  omit-when-null/empty semantics); the table adds `id`, `description`,
  `creator`, `started_at`, `completed_at`, `canceled_at`, `url`, and
  `created_at` to the column set.

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
  Echoes the created issue with the `issue get` field set — state with
  its `type`, `creator`, `started_at`, `completed_at`, `canceled_at`
  included — capture `identifier` and `url` from the JSON.

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
  are untouched. Echoes the updated issue with the `issue get` field
  set.

```sh
everything-cli linear issue update BLA-123 --state 8b9c0d1e-...
everything-cli linear issue update BLA-123 --title "Fix login redirect (regression)" \
  --assignee 4d5e6f7a-...
```

### issue comments

- `linear issue comments <id>` — every comment on one issue. `<id>`
  accepts the issue's UUID or its human identifier (`BLA-123`), like
  `issue get`; there is exactly one positional argument and no
  provider-specific flags. All comment pages are followed
  automatically (see Pagination).

Output: one row per comment. JSON/TOON fields: `id`, `body`,
`created_at`, `updated_at`, `parent_id` (present only on replies —
the threading link to the parent comment; top-level comments omit
it), `user` (`{id, name}` — the comment author, omitted when null).
Table columns: `created_at`, `user` (the display name), `body`. An
issue with no comments prints an empty list (`[]` in JSON).

```sh
everything-cli linear issue comments BLA-123 --format json
everything-cli linear issue comments 8c8a1b2c-0000-4000-8000-000000000001 --format table
```

### issue comment create

- `linear issue comment create <id>` — post a comment on one issue.
  `<id>` accepts the issue's UUID or its human identifier (`BLA-123`),
  like `issue get` / `issue comments`. Flags: `--body <text>` (required
  — the comment text), `--parent <comment-id>` (threads the comment as
  a reply to that comment).
- Echoes the created comment as one object. JSON/TOON fields: `id`,
  `body`, `created_at`, `updated_at`, `parent_id` (present only on
  replies), `user` (`{id, name}`, omitted when null). Table columns:
  `created_at`, `user` (display name), `body`.

```sh
everything-cli linear issue comment create BLA-123 --body "Looking into this now"
everything-cli linear issue comment create BLA-123 --body "Fixed in #482" \
  --parent 8c8a1b2c-0000-4000-8000-000000000042
```

### issue attachment create

- `linear issue attachment create <id>` — link an attachment to one
  issue. `<id>` accepts the issue's UUID or its human identifier
  (`BLA-123`), like `issue get`. Flags: `--url <url>` (required — the
  link target), `--title <title>` (required — the link label),
  `--subtitle <text>` (optional — supporting text).
- Linear treats the same `url` on the same issue as idempotent:
  re-posting an existing URL updates that attachment rather than
  creating a duplicate.
- Echoes the created attachment as one object. JSON/TOON fields: `id`,
  `title`, `url`, `subtitle` (omitted when empty), `created_at`. Table
  columns: `created_at`, `title`, `url`.

```sh
everything-cli linear issue attachment create BLA-123 \
  --url https://example.com/pr/482 --title "PR #482"
everything-cli linear issue attachment create BLA-123 \
  --url https://example.com/pr/482 --title "PR #482" --subtitle "Fix login redirect"
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
the API default. `issue list` scopes server-side: the
`--team`/`--assignee`/`--created-by`/`--updated-since` flags compose
into the one `issues(filter: ..., orderBy: updatedAt)` query, so a
scoped pull replaces whole-workspace paging as the way to narrow a
change-detection sweep — pagination only walks what the filter admits.

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
retry, so wait for the bucket to refill and rerun. `issue list`
always runs as one `issues(filter: ..., orderBy: updatedAt)` query:
the `--team`/`--assignee`/`--created-by`/`--updated-since` filters are
applied server-side and cut both the request count and the page count,
so prefer them over pulling and filtering client-side. An unfiltered
`issue list` still pages the whole workspace at one request per 50
issues — scope it on large workspaces.

## Tips & gotchas (linear)

- Team key vs team ID: `BLA-123` is keyed by the team's human key, but
  `--team` takes the team UUID. Run `linear team list` to map key →
  `id` before scoping or creating.
- `issue get`/`issue update` accept both the UUID and the `BLA-123`
  human identifier; `issue create --team`, `--assignee`, and `--state`
  take UUIDs only. Discover state and assignee UUIDs from an existing
  issue's JSON (`issue get BLA-123 --format json` → `state.id`,
  `assignee.id`); the state object now also carries `type`
  (unstarted/started/completed/canceled), and its lifecycle timestamps
  `started_at`/`completed_at`/`canceled_at` tell you when it moved.
  `linear account whoami` covers your OWN user id only — there is
  still no user-listing command for other users' UUIDs.
- `--title` is required on `issue create` even though the API allows
  untitled issues.
- `issue create`/`issue update` echo the full issue; use `--format
  json` to capture the new `identifier`/`url` programmatically.
- There are no delete/trash verbs on linear resources — move issues
  through workflow states instead (`issue update <id> --state
  <completed-state-id>`).
- `linear account remove` refuses without `--force`.
