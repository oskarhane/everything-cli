# slack

The `slack` provider: read Slack workspaces through the Slack Web API
(`https://slack.com/api`) — full-text message search, channel history and
conversation listing, threads, workspace members, and file download.
Command layout:

```sh
everything-cli slack <resource> <action>
```

Most verbs hang off a resource (`search messages`, `channel history`,
`channel list`, `user get`, `user list`); `thread` is the one exception —
it sits directly under the provider, invoked as `everything-cli slack
thread` (the provider-stripped form is `everything-cli thread`). The
provider is **read-only**: no verb posts, edits, deletes, or reacts.

## Auth (xoxp user token)

Slack authenticates with a **user token** (`xoxp-...`), sent as
`Authorization: Bearer xoxp-...` on every Web API call. User tokens come
from installing a Slack app with user scopes (e.g. `search:read`,
`channels:history`, `channels:read`, `groups:history`, `im:history`,
`mpim:history`, `users:read`, `files:read`) into a workspace; the token
starts with `xoxp-`.

1. `everything-cli slack account add <name>` — captures the token. Capture
   order: the `--api-key` flag, then the `$SLACK_API_KEY` environment
   variable, then a hidden prompt (never echoed). Prefer the env var or
   the prompt — a literal `--api-key xoxp-...` lands in shell history. Add
   validates the token against `auth.test` (a bad token aborts with
   `invalid_auth`, no account written) and stores the workspace identity.
2. `everything-cli slack account use <name>` — set the default Slack
   account. Every command also accepts the global `--account <name>`
   override.

The token is stored at `<config>/accounts/slack/<name>.json` (mode 0600)
and registered for redaction at capture and read time: it is never
printed, in any output format, including `--debug`. `slack account get`
and `slack account list` show metadata only. Treat the token as a secret
on par with an OAuth refresh token.

**Bot and app tokens (`xoxb-...`) do not work.** `search messages`
requires a user token; with a bot token Slack answers `ok: false` with
`not_allowed_token_type`, which surfaces as `slack API error:
not_allowed_token_type`. `slack account add` warns when the captured
token does not start with `xoxp-`. Bot tokens also only see conversations
the bot was invited to, so use an `xoxp-` user token throughout.

## account

Manage Slack accounts and their stored user tokens.

- `slack account add <name>` — add an account from an `xoxp-` user token
  (validated via `auth.test`). Flag: `--api-key <token>` (empty =
  `$SLACK_API_KEY`, then a hidden prompt). Prints the added account's
  `name` only — never the token.
- `slack account list` — list configured Slack accounts. Fields: `name`,
  `default` (the default account carries `default: true` in JSON/TOON,
  `(default)` in table output).
- `slack account get <name>` — account metadata: `name`, `provider`, plus
  the workspace identity captured at add time — `team`, `team_id`,
  `user`, `user_id` (sorted key order after the first two). The token is
  never printed.
- `slack account use <name>` — make `<name>` the default Slack account.
- `slack account remove <name> --force` — remove an account and its
  stored user token. Refuses without `--force`. Removing the default
  promotes another Slack account and announces the new default.

```sh
everything-cli slack account add work                              # hidden prompt
SLACK_API_KEY=xoxp-... everything-cli slack account add work       # non-interactive
everything-cli slack account add work --api-key xoxp-...           # careful: shell history
everything-cli slack account list --format json
everything-cli slack account get work --format json
everything-cli slack account use work
everything-cli slack account remove old --force
```

## search

### search messages

- `slack search messages` — full-text search across the workspace
  (Slack's `search.messages`; user tokens only). Flags:
  - `--query <q>` (required) — Slack search syntax, e.g. `from:me
    deploy`, `in:#general incident`, `has:link`.
  - `--sort <order>` — `score` (Slack's default) or `timestamp`.
  - `--max <n>` — total max matches across pages (default `25`; `0` = no
    cap).

  JSON/TOON output is one object: `query` (echoed by Slack) plus
  `messages`, an array that is `[]` (never null) when nothing matches.
  Each match: `channel_id`, `channel_name`, `user`, `username`, `ts`,
  `text`, `thread_ts` (thread replies only; omitted otherwise),
  `reply_count`, `reactions` (only when present), `files` (attachments:
  array of `{id, name, mimetype, size}`, omitted when the match has none),
  `edited`, `permalink`. Table columns: `channel_id`, `channel_name`,
  `user`, `username`, `ts`, `text`, `permalink`, `thread_ts`, `files`
  (the `files` cell joins `name:id` with commas).

```sh
everything-cli slack search messages --query "from:me deploy" --format table
everything-cli slack search messages --query "incident" --sort timestamp --max 3 --format json
everything-cli slack search messages --query "from:me" --max 0 --format json
```

## channel

### channel history

- `slack channel history` — one conversation's messages, newest first
  (`conversations.history`). Flags:
  - `--channel <id>` (required) — conversation id (`C...` channel,
    `D...` DM, `G...` group DM).
  - `--oldest <ts>` — only messages at or after this Slack timestamp,
    e.g. `1512085950.000216`.
  - `--latest <ts>` — only messages at or before this timestamp.
  - `--max <n>` — total max messages across pages (default `25`; `0` =
    no cap).

  Output fields (JSON/TOON; table order same): `ts`, `channel_id`,
  `user`, `text`, `thread_ts` (omitted when the message is not a thread
  reply), `reply_count`, `reactions` (array of `{name, count}`, only when
  present; table cell joins `name:count` with commas), `files`
  (attachments: array of `{id, name, mimetype, size}`, omitted when the
  message has none; `FILES` table cell joins `name:id` with commas),
  `edited` (boolean).

```sh
everything-cli slack channel history --channel C0B3HMXFEUV --format json
everything-cli slack channel history --channel C0B3HMXFEUV --oldest 1512085950.000216 --latest 1512090000.000000 --format table
```

### channel list

- `slack channel list` — the conversations visible to the token
  (`conversations.list`). Flags:
  - `--types <csv>` — conversation kinds: `public_channel`,
    `private_channel`, `im`, `mpim` (default all four, in that order).
  - `--max <n>` — total max channels across pages (default `25`; `0` =
    no cap).

  Output fields: `id`, `name`, `is_private`, `user` (`user` is the
  counterpart member for `im` entries and omitted elsewhere; `is_private`
  is false for DMs).

```sh
everything-cli slack channel list --format json
everything-cli slack channel list --types public_channel,private_channel --format table
```

## thread

- `slack thread` — one conversation thread: the parent message followed
  by its replies in Slack order (`conversations.replies`). Flags:
  - `--channel <id>` (required) — channel holding the thread, e.g.
    `C0B3HMXFEUV`.
  - `--ts <ts>` (required) — timestamp of the thread's parent message,
    e.g. `1726038000.000100` (the `ts` of the message that started it).
  - `--max <n>` — max messages across pages (default `25`; `0` = no
    cap). The parent message counts: `--max 11` returns the parent plus
    at most 10 replies.

  JSON/TOON is the shared message array (same fields as `channel
  history`, including `channel_id` and `files` — attachments
  `{id, name, mimetype, size}`, omitted when none; table: `ts`, `user`,
  `text`, `thread_ts`, `reply_count`, `edited`, `files` — `channel_id`
  stays out of the table because every row shares it, and the `files`
  cell joins `name:id` with commas).

```sh
everything-cli slack thread --channel C0B3HMXFEUV --ts 1726038000.000100 --format json
everything-cli slack thread --channel C0B3HMXFEUV --ts 1726038000.000100 --max 11 --format table
```

## file

### file download

- `slack file download <file-id> [--out <path>]` — download one uploaded
  file's content (`files.info` + the authenticated `url_private`). The
  positional `<file-id>` is Slack's file id (e.g. `F0B3HMXFEUV` — take it
  from a message's `files` array, where each entry's table cell renders
  `name:id`). Flag:
  - `--out <path>` — write the bytes to this local file instead of
    stdout. Empty streams to stdout, so redirect or pipe it
    (`everything-cli slack file download F0B3HMXFEUV > report.pdf`).

  `file download` resolves the file with `files.info`, then GETs the
  returned `url_private` with the same authenticated token and streams the
  body. `url_private` is wire-only and is never printed. The download is
  unbounded (no size cap — consume or redirect as needed). Reading
  attachments needs the `files:read` user scope; without it Slack answers
  `missing_scope`. Like every Slack verb, this is read-only: it fetches
  bytes and changes nothing.

```sh
everything-cli slack file download F0B3HMXFEUV > report.pdf
everything-cli slack file download F0B3HMXFEUV --out report.pdf
```

## user

### user get

- `slack user get --user <id>` — one workspace member by Slack user ID
  (`users.info`). `--user` is required (e.g. `U02H6ECK2` — discover ids
  via `user list` or message `user` fields). Output fields: `id`, `name`
  (handle), `real_name`, `display_name`.

```sh
everything-cli slack user get --user U02H6ECK2 --format json
everything-cli slack user get --user U02H6ECK2 --format table
```

### user list

- `slack user list` — workspace members (`users.list`). Flags:
  - `--query <text>` — case-insensitive substring matched against
    `name`, `real_name`, and `display_name`; empty = every member.
  - `--max <n>` — maximum matching members to return (default `25`; `0`
    = no cap). The filter runs client-side per page and paging stops as
    soon as `--max` matches are collected, so a query that never matches
    pages every member before answering.

  Output fields: same as `user get` (`id`, `name`, `real_name`,
  `display_name`).

```sh
everything-cli slack user list --format json
everything-cli slack user list --query oskar --format table
everything-cli slack user list --query eng --max 0
```

## Pagination

Every paged Slack read takes `--max <n>`: the total items returned
(default `25`; `0` = no cap), counted after filtering and truncating any
overshoot from the last page. What one item is depends on the verb:
matches for `search messages`, messages for `channel history`,
conversations for `channel list`, messages (parent plus replies) for
`thread`, members for `user list`.

- `search messages` pages by page number (`count=100` per request),
  following `messages.paging` until `--max` matches are gathered or the
  last page is reached.
- `channel history`, `channel list`, `thread`, and `user list` follow
  `response_metadata.next_cursor`, requesting up to 200 items per page
  (the remaining budget when smaller).

As a guard against a misbehaving endpoint looping cursors forever, a
listing gives up after 100 pages with an error like `channel history did
not terminate after 100 pages` — far beyond any real listing, so this
should never fire against the real API.

## Errors

Slack answers HTTP 200 with `"ok": false` for application errors; the CLI
surfaces the raw code, plus scope detail when Slack supplies it:

- **`missing_scope`** — `slack API error: missing_scope (needed:
  <scopes>; provided: <scopes>)`: the token lacks a scope the verb needs
  (e.g. `search:read` for search, `channels:history` for channel history,
  `missing_scope files:read` for `file download` and any message with
  attachments). Remediation: re-install the app with the missing user
  scope, then add the account again with `slack account add`.
- **`invalid_auth`** — the token is invalid, revoked, or wrong (also
  raised by `account add`'s `auth.test` validation). Remediation: add a
  valid `xoxp-` token with `slack account add`.
- **`not_allowed_token_type`** — the acting token is a bot/app token
  (`xoxb-...`), which cannot call `search.messages`. Remediation: add a
  user token (`xoxp-...`); see Auth.
- **`channel_not_found`** — the `--channel` id does not exist or the
  token cannot see it; verify the id via `channel list`.
- **429** — `slack API rate limit exceeded (429): retry after Ns`: wait
  and rerun; there is no automatic retry.
- Other non-200 statuses surface as `slack API returned <status>:
  <body>`.

Decoding is deliberately tolerant (Slack payloads are subtype-heavy), but
a malformed response fails loudly with `decoding slack /<method> response
(upstream schema changed?): ...` rather than silently dropping data.
Report that error — it means the CLI's pinned schema needs an update.

## Tips & gotchas (slack)

- `search messages` needs an `xoxp-` user token; bot tokens fail with
  `not_allowed_token_type`. `account add` warns when the token does not
  start with `xoxp-`.
- `--max 0` means "everything", still bounded by the 100-page
  runaway-cursor guard. Prefer a `--max` for a quick look; the default is
  25.
- Slack timestamps (`ts`) are strings like `1726038000.000100` — pass
  them through verbatim to `--oldest`, `--latest`, and `--ts`.
- To read a thread, take a message's `ts` (from `search messages` or
  `channel history`) and pass it as `thread --ts` with the same
  `--channel`; a reply's own `thread_ts` is the parent's `ts`.
- `user list --query` filters client-side after each page arrives — use
  it to resolve a member by name before `user get`, or to find ids for
  message `user` fields.
- Attachments appear as a message's `files` array (`{id, name, mimetype,
  size}`), omitted when there are none; the `FILES` table column shows
  `name:id`. Download one with `slack file download <id>`.
- Read-only provider: there are no send, edit, delete, or reaction verbs;
  `file download` only fetches bytes.
- Output field names are snake_case in every format; table headers render
  UPPER-CASE.
