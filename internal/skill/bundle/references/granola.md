# granola

The `granola` provider: read Granola notes via the official Granola public
API (`https://public-api.granola.ai`, paths are `/v1/...`). Command layout:

```sh
everything-cli granola <resource> <action>
```

The provider is read-only: it lists and gets notes. There are no write,
trash, or delete verbs.

## Auth (API key)

Granola authenticates with a `grn_` API key, sent as `Authorization:
Bearer grn_...`.

1. Create a key in the Granola desktop app: **Settings → Connectors → API
   keys → Create new key**, choosing the note access scopes (Personal
   notes and/or Public notes). Creating API keys requires a **Business or
   Enterprise plan**. Workspace admins on Business/Enterprise can also
   create non-expiring workspace API keys; on Enterprise, admins gate
   member scopes in **Settings → Workspace → General → API access for
   members**.
2. `everything-cli granola account add <name>` — captures the key. Capture
   order: the `--api-key` flag, then the `$GRANOLA_API_KEY` environment
   variable, then a hidden prompt (never echoed). Prefer the prompt or
   the env var — a literal `--api-key grn_...` lands in shell history.
3. `everything-cli granola account use <name>` — set the default Granola
   account. Every command also accepts the global `--account <name>`
   override.

The key is stored at `<config>/accounts/granola/<name>.json` (mode 0600)
and registered for redaction at capture and read time: it is never
printed, in any output format, including `--debug`. `granola account get`
and `granola account list` show metadata only. Treat the key as a secret
on par with an OAuth refresh token.

## account

Manage Granola accounts and their stored API keys.

- `granola account add <name>` — add an account from a `grn_` API key.
  Flag: `--api-key <key>` (empty = `$GRANOLA_API_KEY`, then a hidden
  prompt). Prints the added account's `name` only — never the key.
- `granola account list` — list configured Granola accounts. Fields:
  `name`, `default` (the default account carries `default: true` in
  JSON/TOON, `(default)` in table output).
- `granola account get <name>` — account metadata. Fields: `name`,
  `provider`. The API key is never printed.
- `granola account use <name>` — make `<name>` the default Granola
  account.
- `granola account remove <name> --force` — remove an account and its
  stored key. Refuses without `--force`.

```sh
everything-cli granola account add work                                  # hidden prompt
GRANOLA_API_KEY=grn_... everything-cli granola account add work          # non-interactive
everything-cli granola account add work --api-key grn_...                # careful: shell history
everything-cli granola account list --format json
everything-cli granola account get work --format json
everything-cli granola account use work
everything-cli granola account remove old --force
```

## note

Read Granola notes. Note ids look like `not_abc123def456gh`; folder ids
like `fol_abc123def456gh`.

**Only notes with a generated AI summary and transcript are returned by
the API** — a note still processing (or never summarized) is absent from
`note list` and answers 404 on `note get`.

### note list

- `granola note list` — every note matching the filters, following cursor
  pagination automatically (page size 30, the API's maximum). Filters
  (all optional, all ANDed):
  - `--created-after <date-or-datetime>` — notes created after this
    date (`2026-08-01`) or date-time.
  - `--created-before <date-or-datetime>` — notes created before this
    date or date-time.
  - `--updated-after <date-or-datetime>` — notes updated after this
    date or date-time — the change-detection pull.
  - `--folder-id <fol_...>` — only notes in this folder; child folders
    are included.

  Values pass through to the API verbatim. Output fields (snake_case in
  every format): `note_id`, `title`, `owner` (owner email), `created_at`,
  `updated_at`.

```sh
everything-cli granola note list --format json
everything-cli granola note list --created-after 2026-08-01 --created-before 2026-09-01 --format table
everything-cli granola note list --updated-after 2026-08-31T00:00:00Z
everything-cli granola note list --folder-id fol_abc123def456gh
everything-cli granola note list --account work --format toon
```

### note get

- `granola note get <note-id>` — one note with its AI summary. Flag:
  `--include-transcript` — inline the full transcript (very long notes
  may fail with `413 TRANSCRIPT_TOO_LARGE`; see Errors).

  JSON/TOON output is the full note object: `note_id`, `title`, `owner`
  (`{name, email}`), `created_at`, `updated_at`, `web_url` (Granola web
  app URL), `calendar_event` (`event_title`, `invitees`, `organiser`,
  `calendar_event_id`, `scheduled_start_time`, `scheduled_end_time`;
  null when the note is not attached to a meeting), `attendees`
  (`{name, email}`), `folder_membership` (`{id, name,
  parent_folder_id}`, ancestor folders included), `summary_text` (AI
  summary, plain text), `summary_markdown` (AI summary, markdown; null
  when absent), `private_notes_text` / `private_notes_markdown` (the
  owner's own private notes — only when the API key belongs to the
  note's creator, else null), and `transcript` (only with
  `--include-transcript`: an array of `{speaker, text, start_time,
  end_time}`, where `speaker` is `{source: microphone|speaker,
  attribution: me|them, diarization_label, name}`). Table output is a
  one-row summary: `note_id`, `title`, `owner`, `created_at`,
  `updated_at`, `web_url`.

```sh
everything-cli granola note get not_abc123def456 --format json
everything-cli granola note get not_abc123def456 --include-transcript
everything-cli granola note get not_abc123def456 --format json | jq -r .summary_markdown
```

## Pagination

`granola note list` follows the API's cursor across pages on its own —
there is no `--max` flag; one command returns the whole matching listing.
Requests use `page_size=30` (the API maximum). As a guard against a
misbehaving endpoint looping cursors forever, listing gives up after 100
pages (3000 notes) with `note listing did not terminate after 100 pages`
— far beyond any real listing, so this should never fire against the real
API.

## Errors

Non-200 API responses surface as descriptive errors:

- **401** — `granola API rejected the API key (401)`: the key is invalid,
  expired, or revoked. Remediation: add a valid key with `everything-cli
  granola account add`.
- **404** — `granola note not found (404)`: no such note id — or the note
  is still processing / was never summarized (the API only serves notes
  with a generated AI summary and transcript). Remediation: verify the id
  via `granola note list`; retry later for a freshly recorded note.
- **413** — `granola transcript too large (413 TRANSCRIPT_TOO_LARGE)`:
  the transcript does not fit one response. The API serves oversized
  transcripts in pages from `GET /v1/notes/<note_id>/transcript`, which
  this CLI does not support yet. Remediation: retry without
  `--include-transcript` to get the summary.
- **429** — `granola API rate limit exceeded (429)`: documented limits
  are 25 requests per 5-second burst and 5 requests/second sustained
  (300/minute), applied per user or workspace depending on key scope.
  Remediation: back off and retry shortly.

A malformed filter value (bad date, an id not matching the API's
`not_...`/`fol_...` patterns) comes back as a generic `granola API
returned 400: <detail>`.

Decoding is strict: if Granola adds or renames response fields, commands
fail loudly with `decoding granola /v1/... response (upstream schema
changed?): ...` instead of silently dropping data. Report that error — it
means the CLI's pinned schema needs an update.

## Tips & gotchas (granola)

- **No search endpoint exists** in the public API. `granola note list`
  plus the date/folder filters is the discovery path — list (narrow by
  `--created-after`/`--updated-after`/`--folder-id`), pick the id, then
  `granola note get`.
- Only notes with a generated AI summary **and** transcript are returned.
  Brand-new or unsummarized notes are invisible to both `list` (absent)
  and `get` (404).
- `note list` is read-only and paginates internally — there is no `--max`
  to tune; filters are the way to shrink a listing.
- `--include-transcript` can fail with 413 on very long notes; the
  summary-only `get` always works.
- `private_notes_*` fields are null unless the API key belongs to the
  note's creator — with a workspace key, expect null on other members'
  notes.
- The API key is a secret: it is registered for redaction before anything
  could print it and never appears in output, including `--debug`.
