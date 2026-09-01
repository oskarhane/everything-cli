# google

The `google` provider: Gmail, Calendar, Drive, Docs, Sheets, Slides, and
YouTube — plus Google account management via OAuth. Command layout:

```sh
everything-cli google <resource> <action>
```

Provider-specific persistent flag: `--credentials <path>` on the `google`
command — path to the OAuth app credentials JSON (empty = auto-resolve).

## Setup (OAuth)

1. Have a Google Cloud OAuth client's `credentials.json` (Desktop app
   type). Place it at the default location or pass `--credentials
   <path>` on any google command. Provisioning the app (consent-screen
   user type, External vs Internal, publishing status, the full scope
   list) is covered in the README's *Getting started* section.
2. `everything-cli google account add <name>` — runs the OAuth flow,
   verifies identity, and stores the token. `--scopes` overrides the
   default scope set (Gmail modify/send/compose + Calendar + Drive +
   Docs + Sheets + Slides + userinfo.email).
3. `everything-cli google account use <name>` — set the default Google
   account. Every command also accepts the global `--account <name>`
   override.

Minimal Drive profile: swap the full `Drive` scope for
`https://www.googleapis.com/auth/drive.file` to reach only app-created
files, e.g. `everything-cli google account add work --scopes
https://www.googleapis.com/auth/gmail.modify,...,https://www.googleapis.com/auth/drive.file`;
the sharing commands (`google drive file share`/`unshare`/`permissions`)
still require the full drive scope.

## account

Manage Google accounts and their cached OAuth tokens. Tokens are stored
at `<config>/accounts/google/<name>.json` (mode 0600) and never printed.

- `google account list` — all configured Google accounts; the default
  carries `default: true`.
- `google account add <name>` — authorize a new account via OAuth.
  Flags: `--credentials <path>` (explicit OAuth client
  credentials.json), `--scopes <csv>`. Scope minimization: with a
  narrowed grant, Drive/Docs/Sheets/Slides commands fail fast with the
  missing scope and a re-consent action instead of raw 403s. File
  read/write leaves work under `drive.file`; the sharing commands
  require the full `https://www.googleapis.com/auth/drive` scope.
- `google account get <name>` — account metadata (name, email, scopes,
  default). Token values are never printed, in any output format.
- `google account use <name>` — make `<name>` the default Google
  account.
- `google account remove <name> [--force]` — remove an account and its
  cached token. Refuses without `--force`.

```sh
everything-cli google account list --format json
everything-cli google account add work
everything-cli google account add work --credentials ~/google/credentials.json \
  --scopes https://www.googleapis.com/auth/gmail.send
everything-cli google account add work --scopes https://www.googleapis.com/auth/gmail.modify,https://www.googleapis.com/auth/calendar,https://www.googleapis.com/auth/drive.file,https://www.googleapis.com/auth/documents,https://www.googleapis.com/auth/spreadsheets,https://www.googleapis.com/auth/presentations
everything-cli google account get work --format json
everything-cli google account use work
everything-cli google account remove work --force
```

Accounts added before Drive/Docs/Sheets/Slides support lack the new
scopes: re-run `google account add <name>` with the same name to grant
them (the flow always re-prompts with `prompt=consent`, and the account
is updated in place, keyed by email).

## gmail

Manage Gmail labels, messages, drafts, threads, and attachments.

### label

- `google gmail label list` — list labels.
- `google gmail label get <id-or-name>` — show one label by id or name.
- `google gmail label create <name>` — flags: `--color-text #rrggbb`,
  `--color-bg #rrggbb`, `--label-list-visibility labelShow|labelHide`,
  `--message-list-visibility show|hide`.
- `google gmail label update <id>` — `--name`, plus the create flags.
- `google gmail label delete <id> [--force]` — refuses without `--force`.

```sh
everything-cli google gmail label create Travel --color-text "#ffffff" --color-bg "#039be5" \
  --label-list-visibility labelHide
everything-cli google gmail label delete Label_42 --force
```

### message

- `google gmail message list` — flags: `--query/-q` (Gmail search, e.g.
  `from:boss@corp.com subject:invoice`), `--label-ids`
  (comma-separated), `--unread-only`, `--max` (default 25).
- `google gmail message get <id>` — headers + body as JSON/table;
  `--raw` prints the decoded RFC 2822 message (control bytes stripped).
- `google gmail message send` — `--to` (required, comma-separated),
  `--cc`, `--bcc`, `--subject`, `--body | --body-file` (mutually
  exclusive), `--attachment <path>` (repeatable).
- `google gmail message modify <id>` — `--add-label-ids`,
  `--remove-label-ids`.
- `google gmail message mark <id>` — `--read`, `--unread`, `--starred`,
  `--unstarred`.
- `google gmail message trash <id>` /
  `google gmail message untrash <id>` — recoverable.
- `google gmail message delete <id> [--force]` — permanent; refuses
  without `--force`. Prefer `trash`.

```sh
everything-cli google gmail message list --query "from:boss@corp.com subject:invoice" \
  --unread-only --max 10 --format json
everything-cli google gmail message send --to a@example.com,b@example.com --subject "Report" \
  --body-file report.txt --attachment q1.pdf --attachment data.csv
everything-cli google gmail message modify 19c2a4b7 --add-label-ids Label_7
everything-cli google gmail message trash 19c2a4b7 --account work
```

### thread

- `google gmail thread list` — `--query/-q`, `--label-ids`, `--max`
  (default 25).
- `google gmail thread get <id>` — a thread's messages with headers.

```sh
everything-cli google gmail thread list --query "subject:invoice" --label-ids Label_7 --max 10
```

### draft

- `google gmail draft list` — `--max` (default 25).
- `google gmail draft get <id>` — stored message headers.
- `google gmail draft create` — `--to` (required), `--subject`, `--body
  | --body-file`.
- `google gmail draft send <id>` — sends and echoes the sent message.
- `google gmail draft delete <id> [--force]` — permanent; refuses
  without `--force`.

```sh
everything-cli google gmail draft create --to alice@example.com --subject "Lunch" --body "Noon works"
everything-cli google gmail draft send draft_19c2a4b7 --format json
```

### attachment

- `google gmail attachment get <attachment-id>` — `--message-id <id>`
  (required), `--out <file>` (else decoded bytes to stdout for piping).

```sh
everything-cli google gmail attachment get ANG1xQ8q --message-id 19c2a4b7 --out report.pdf
```

## calendar

Manage calendars, their sharing rules, events, and free/busy.

Timestamp flags (`--start`, `--end`, `--from`, `--to`,
`--updated-since`) accept RFC3339 (`2026-09-03T14:00:00Z`), naive
RFC3339 (`2026-09-03T14:00:00` — read in the local timezone), a date
(`2026-09-03`, needs `--all-day` for event create/update), or relative
offsets anchored at now (`now`, `-1d`, `+7d`, `-30m`, `+2h`, bare `7d`
counts forward).

### calendar CRUD

- `google calendar list` — every listed calendar.
- `google calendar create <summary>` — `--timezone`, `--description`,
  `--color-id`.
- `google calendar get <calendar-id>` — show one calendar by id.
- `google calendar update <calendar-id>` — `--summary`, `--timezone`,
  `--description`, `--color-id`.
- `google calendar delete <calendar-id> [--force]` — destructive;
  refuses without `--force`.

```sh
everything-cli google calendar create "Team PTO" --timezone Europe/Stockholm --color-id tomato
everything-cli google calendar delete abc123.group.calendar.google.com --force
```

### event

- `google calendar event list` — `--calendar` (default `primary`),
  `--from` (default `now`), `--to` (default `+7d`), `--query`,
  `--updated-since <ts>` (events modified since; deletions since then
  included — good for change-detection pulls), `--show-deleted` (default
  true; cancelled events surface with `status: "cancelled"`), `--max`
  (default 250; `0` = no cap), `--recurring instances|masters|all`
  (default `instances` = recurring series expanded into occurrences;
  `masters` = raw masters plus one-offs and exceptions; `all` = both
  merged and deduped). Rows carry 12 snake_case fields — `status`,
  `self_response` (the account's RSVP), `created`, `updated`,
  `organizer`, `description`, plus the scheduling fields; `get` adds
  `location`, `attendees`, and `recurrence`.
- `google calendar event get <event-id>` — `--calendar`.
- `google calendar event create` — `--summary` (required), `--start`,
  `--end` (required; RFC3339, or YYYY-MM-DD with `--all-day`),
  `--calendar`, `--all-day`, `--timezone` (required for recurring
  series), `--location`, `--description`, `--attendee` (repeatable),
  `--reminder-minutes`, `--color-id`, `--recurrence` (repeatable raw
  `RRULE:`/`RDATE:`/`EXDATE:`).
- `google calendar event update <event-id>` — `--summary`, `--start`,
  `--end`, `--location`, `--description`,
  `--add-attendee`/`--remove-attendee` (repeatable), `--calendar`,
  `--this-only` (default true; false with an instance id patches its
  master, i.e. the whole series).
- `google calendar event delete <event-id> [--force]` — `--calendar`,
  `--this-only` (default true; false with an instance id deletes its
  master). Refuses without `--force`.
- `google calendar event instances <master-id>` — expand a recurring
  series; `--calendar`, `--from` (default `now`), `--to` (default
  `+7d`), `--max` (default 250), `--show-deleted` (default true). Pass a
  wide window explicitly for long-range expansion.
- `google calendar event move <event-id>` — `--to-calendar` (required),
  `--calendar` (source).
- `google calendar event accept <event-id>`,
  `google calendar event decline <event-id>`,
  `google calendar event tentative <event-id>` —
  respond to an invitation. `--calendar`, `--this-only` (default true),
  `--all` (respond for the whole series; overrides `--this-only`). An
  instance id (ends in `_UTC-time`) responds for that occurrence only;
  `--all` or a master id responds for the series.

```sh
everything-cli google calendar event create --summary "Design review" \
  --start 2026-09-03T14:00:00Z --end 2026-09-03T15:00:00Z
everything-cli google calendar event create --summary "Standup" --start 2026-09-01T09:00:00+02:00 \
  --end 2026-09-01T09:30:00+02:00 --attendee colleague@example.com \
  --recurrence 'RRULE:FREQ=WEEKLY;COUNT=10' --format json
everything-cli google calendar event list --from 2026-09-01T00:00:00Z --to 2026-09-08T00:00:00Z --format json
everything-cli google calendar event list --calendar work@example.com --query "design review" --max 10
everything-cli google calendar event list --updated-since -1d --format json
everything-cli google calendar event get abc123 --format json
everything-cli google calendar event update abc123 --summary "Design review"
everything-cli google calendar event delete kq3abc123_20260929T030000Z --force
everything-cli google calendar event accept kq3abc123_20260929T030000Z --all
everything-cli google calendar event instances kq3abc123 --from now --to +14d
everything-cli google calendar event move abc123 --to-calendar work.group.calendar.google.com
```

Recurring events default to ONE occurrence: an instance id ends in
`_UTC-time` (e.g. `abc123_20260929T030000Z`) and acts on that occurrence
only. Pass `--all` (respond) or omit `--this-only` (update/delete with
an instance id) to act on the whole series.

### acl (sharing rules)

- `google calendar acl list <calendar-id>` — list sharing rules.
- `google calendar acl add <calendar-id>` — `--scope-user <email>`
  (required), `--role reader|writer`.
- `google calendar acl remove <calendar-id>` — `--rule-id <id>` (e.g.
  `user:colleague@example.com`).

```sh
everything-cli google calendar acl add primary --scope-user colleague@example.com --role reader
everything-cli google calendar acl remove primary --rule-id user:colleague@example.com
```

### freebusy

- `google calendar freebusy` — `--calendar <id>` (repeatable; default =
  every listed calendar), `--from` (default `now`), `--to` (default
  `+1d`).

```sh
everything-cli google calendar freebusy --from 2026-09-01T09:00:00Z --to 2026-09-01T17:00:00Z \
  --calendar work@example.com --calendar personal@example.com --format table
```

## drive

Generic Drive file operations — list, get, create, upload, download,
trash, delete, and sharing. Create Google Docs/Sheets/Slides here; their
content-level verbs live in the docs/sheets/slides sections below.

### file list

- `google drive file list` — flags: `--query/-q` (raw Drive q term
  passed through verbatim, e.g. `owner = 'me'`), `--name` (substring
  match), `--parent` (folder id), `--mime` (`folder`, `doc`, `sheet`,
  `slide`, or a raw MIME type), `--trashed` (include trashed; default
  lists only non-trashed), `--max` (default 25, `0` = uncapped). All
  terms are ANDed.

```sh
everything-cli google drive file list --format json
everything-cli google drive file list --parent 1AbC --mime folder --format table
everything-cli google drive file list --name "invoice" --query "owner = 'me'" --max 100
```

`--mime` shorthands (any other value passes through raw): `folder` →
`application/vnd.google-apps.folder`, `doc` → `...document`, `sheet` →
`...spreadsheet`, `slide` → `...presentation`. A full MIME string works
too.

### file get

- `google drive file get <file-id>` — metadata only (name, mime_type,
  size, owner, parent_ids, trashed, shared, modified_time, web_link,
  description). Use `download` for content.

```sh
everything-cli google drive file get 1AbCdEfGh --format json
```

### file create

- `google drive file create <name>` — flags: `--type
  folder|doc|sheet|slide` (default `folder`), `--mime-type` (raw MIME;
  mutually exclusive with `--type`), `--parent`, `--description`. This
  is how Docs/Sheets/Slides files are created — there is no separate
  `docs create`/`sheets create`/`slides create`.

```sh
everything-cli google drive file create "Reports"
everything-cli google drive file create "Q3 budget" --type sheet --parent 1AbCdEfGh
everything-cli google drive file create "notes.md" --mime-type text/markdown
```

### file upload

- `google drive file upload <local-path>` — flags: `--name` (default:
  the local path's base name), `--parent`, `--mime-type` (default from
  the extension, else `application/octet-stream`).

```sh
everything-cli google drive file upload ./report.pdf --name "Q3 report" --parent 1AbCdEfGh
```

### file download

- `google drive file download <file-id>` — `--out <file>` (else bytes to
  stdout for piping), `--export <mime>`. Binary files stream via
  alt=media. Google-native types have no downloadable binary and must be
  exported. Export defaults when `--export` is unset: doc →
  `text/plain`, sheet → `text/csv`, slide → `text/plain`; other native
  types refuse until `--export` names a supported export MIME.
- Export caveat: sheet exports (`text/csv`, `text/tab-separated-values`)
  cover the FIRST SHEET ONLY, and Drive caps exports at 10 MB (larger
  exports are truncated with a warning marker). Other supported exports:
  docs: text/plain, text/markdown, application/pdf, application/rtf,
  application/epub+zip, application/zip, docx, odt; sheets: text/csv,
  text/tab-separated-values, application/pdf, application/zip, xlsx,
  ods; slides: text/plain, application/pdf, pptx, odp; drawings:
  image/png, image/jpeg, image/svg+xml, application/pdf.

```sh
everything-cli google drive file download 1AbCdEfGh --out report.pdf
everything-cli google drive file download 1AbCdEfGh --export text/csv > data.csv
```

### file trash / untrash

- `google drive file trash <file-id>` /
  `google drive file untrash <file-id>` — move to Drive's trash and
  restore. Recoverable; prefer over delete.

```sh
everything-cli google drive file trash 1AbCdEfGh
everything-cli google drive file untrash 1AbCdEfGh --account work
```

### file delete

- `google drive file delete <file-id> [--force]` — permanent, bypasses
  the trash; refuses without `--force`. Prefer `google drive file
  trash`.

```sh
everything-cli google drive file delete 1AbCdEfGh --force
```

### file permissions

- `google drive file permissions <file-id>` — list every permission (id,
  type, role, email_address, display_name, deleted). Run it to find the
  id `unshare` needs.

```sh
everything-cli google drive file permissions 1AbCdEfGh --format json
```

### file share

- `google drive file share <file-id>` — `--role reader|commenter|writer`
  (required) plus exactly one of `--email <addr>` | `--anyone` |
  `--domain <domain>`. `--expires <rfc3339>` sets an expiry, which the
  Drive API honors only on user and group permissions (and allows at
  most one year out).

```sh
everything-cli google drive file share 1AbCdEfGh --role reader --email alice@example.com \
  --expires 2027-09-01T00:00:00Z
everything-cli google drive file share 1AbCdEfGh --role commenter --anyone
everything-cli google drive file share 1AbCdEfGh --role writer --domain example.com
```

### file unshare

- `google drive file unshare <file-id>` — exactly one of `--permission
  <id>` (see `google drive file permissions`) or `--email <addr>`
  (resolved via the permissions list, case-insensitively; zero or
  multiple matches refuse and ask for `--permission`).

```sh
everything-cli google drive file unshare 1AbCdEfGh --email alice@example.com
everything-cli google drive file unshare 1AbCdEfGh --permission 8zK
```

## docs

Google Docs content, at the text level. Create documents with `google
drive file create <name> --type doc`. Docs are Drive files: `google docs
delete` is a thin Drive delete, and `google drive file trash <doc-id>`
is the recoverable alternative to `google docs delete`.

- `google docs get <doc-id>` — the document's raw text, streamed to
  stdout exactly as exported (bypasses `--format`); `--out <file>`
  writes it there instead.
- `google docs append <doc-id>` — add text at the very end of the body.
  Flags: `--text` | `--text-file <path>` (exactly one). A trailing
  newline is added when missing, so successive appends each start on
  their own line.
- `google docs insert <doc-id>` — insert text immediately BEFORE the
  given `--index` (required, > 0 — a zero-based Docs-API content index
  in UTF-16 code units; `--index 1` puts the text at the very start of
  the body). Flags: `--text` | `--text-file` (exactly one). Unlike
  append, the text is sent verbatim — no newline is added.
- `google docs replace <doc-id>` — replace every occurrence of `--find`
  (required) with `--replace-with` (empty deletes the matches).
  `--match-case` makes matching case-sensitive (default is
  case-insensitive). Prints the replaced occurrence count.
- `google docs delete <doc-id> [--force]` — permanently delete the
  document's Drive file; refuses without `--force`. Prefer `google drive
  file trash <doc-id>`.

```sh
everything-cli google docs get 1AbCdEfGh --out notes.txt
everything-cli google docs get 1AbCdEfGh | head -20
everything-cli google docs append 1AbCdEfGh --text "Reviewed by Oskar"
everything-cli google docs append 1AbCdEfGh --text-file notes.txt
everything-cli google docs insert 1AbCdEfGh --index 1 --text "Q4 plan"
everything-cli google docs insert 1AbCdEfGh --text-file block.txt --index 120
everything-cli google docs replace 1AbCdEfGh --find "Project Falcon" --replace-with "Project Falcon 2"
everything-cli google docs replace 1AbCdEfGh --find TODO --replace-with "TBD"
everything-cli google docs delete 1AbCdEfGh --force
```

## sheets

Google Sheets metadata and cell values. Create spreadsheets with `google
drive file create <name> --type sheet`; `google sheets delete` is a thin
Drive delete — `google drive file trash <spreadsheet-id>` is the
recoverable alternative.

- `google sheets get <spreadsheet-id>` — one row per sheet tab:
  sheet_id, title, index, row_count, col_count, and a best-effort header
  (the tab's first row; empty when unreadable).
- `google sheets values get <spreadsheet-id>` — read an A1 range;
  `--range` is required (e.g. `Sheet1!A1:D10`). One row per spreadsheet
  row (`row` + tab-joined `values` in table output; `{"range",
  "values"}` object in JSON/TOON).
- `google sheets values append <spreadsheet-id>` — add rows after the
  last row of the table containing the range. Flags: `--range`
  (required, locates the table, e.g. `Sheet1!A1:D`), `--values` |
  `--values-file` (exactly one), `--input-option RAW|USER_ENTERED`
  (default `USER_ENTERED`).
- `google sheets values update <spreadsheet-id>` — write rows starting
  at the top-left of `--range` (required), overwriting what is there.
  Same `--values`/`--values-file`/`--input-option` flags.
- `google sheets values clear <spreadsheet-id>` — empty every cell in
  `--range` (required; formatting kept). No `--force`: it is bounded to
  the range and recoverable via revision history.
- `google sheets delete <spreadsheet-id> [--force]` — permanently delete
  the spreadsheet's Drive file; refuses without `--force`. Prefer
  `google drive file trash <spreadsheet-id>`.

Value input: `--values` takes an inline JSON array of arrays, e.g.
`'[[1,"a"],[2,"b"]]'`; `--values-file` takes a `.json`, `.csv`, or
`.tsv` file (by extension). `--input-option RAW` writes values as
literal strings (e.g. no formula parsing); `USER_ENTERED` parses them.

```sh
everything-cli google sheets get 1AbCdEfGh --format json
everything-cli google sheets get 1AbCdEfGh --format json | jq '.sheets[] | select(.title=="Budget")'
everything-cli google sheets values get 1AbCdEfGh --range "Sheet1!A1:D10" --format json
everything-cli google sheets values get 1AbCdEfGh --range "Budget!A1:C20" --format table
everything-cli google sheets values append 1AbCdEfGh --range "Sheet1!A1:D" \
  --values '[[1,"a",true],[2,"b",false]]' --format json
everything-cli google sheets values update 1AbCdEfGh --range "Sheet1!A1:B2" \
  --values-file ./cells.tsv
everything-cli google sheets values update 1AbCdEfGh --range "Sheet1!C1" \
  --values '[[=SUM(A1:A2)]]' --input-option RAW
everything-cli google sheets values clear 1AbCdEfGh --range "Sheet1!A2:D10"
everything-cli google sheets delete 1AbCdEfGh --force
```

## slides

Google Slides text operations. Create presentations with `google drive
file create <name> --type slide`; `google slides delete` is a thin Drive
delete — `google drive file trash <presentation-id>` is the recoverable
alternative.

- `google slides get <presentation-id>` — one row per text-bearing
  shape, in slide order (fields: slide, shape_id, text). `--slide <n>`
  narrows to one slide's shapes (1-based, matching the slide column;
  0 = all slides).
- `google slides replace <presentation-id>` — replace every occurrence
  of `--find` (required) with `--replace-with` across every slide, in
  one API call. `--match-case` makes matching case-sensitive (default
  is case-insensitive). Prints the replaced occurrence count.
- `google slides delete <presentation-id> [--force]` — permanently
  delete the presentation's Drive file; refuses without `--force`.
  Prefer `google drive file trash <presentation-id>`.

```sh
everything-cli google slides get 1AbCpresentationID --format json
everything-cli google slides get 1AbCpresentationID --slide 3
everything-cli google slides replace 1AbCpresentationID --find Acme --replace-with Zenith
everything-cli google slides replace 1AbCpresentationID --find KPI --replace-with OKR --match-case
everything-cli google slides delete 1AbCpresentationID --force
```

## youtube

YouTube video metadata and timed transcripts — no Google account, no
OAuth, and no API key of your own. `google youtube transcript` prints a
video's captions and `google youtube metadata` reports its player
metadata. Both take a single `<url-or-id>` argument in any of these
forms:

- a watch URL: `https://www.youtube.com/watch?v=<id>` (also
  `m.youtube.com`/`music.youtube.com` subdomains)
- a `youtu.be` short link: `https://youtu.be/<id>`
- a `/shorts/`, `/embed/`, or `/live/` link:
  `https://www.youtube.com/live/<id>`
- a bare 11-character video ID: `dQw4w9WgXcQ`

Anything else fails with `invalid YouTube video ID`. The global
`--account` flag and the provider's `--credentials` flag are irrelevant
here — these commands never touch the token cache.

### youtube transcript

- `google youtube transcript <url-or-id>` — print the video's timed
  captions. Flags:
  - `--lang <code>` — caption track language, ISO 639-1 (default `en`).
    Preference: a human-written track in that language, then an
    auto-generated (ASR) track in that language, then the first
    available track regardless of language.
  - `--raw` — print the caption text as plain lines even on an
    interactive terminal.
  - `--out <file>` — write the plain caption text to a file instead of
    stdout (beats everything, like `google docs get --out`).
  - Plain-text caption lines are control-byte sanitized before writing —
    caption text is creator-controlled, so C0 bytes (ANSI/OSC terminal
    escapes) are replaced with `?` (tabs and newlines are preserved).
- Rendering: piped/non-TTY stdout streams plain text (one caption line
  per segment); an interactive terminal shows the structured report as a
  table; an explicit `--format json|table|toon` always renders the
  structured report. Fields: `video_id`, `title`, `lang`,
  `is_generated`, `segments` — each segment has `start_ms`,
  `duration_ms`, and `text` (JSON/TOON carry the full timed array; the
  table cell shows a compact "N segments · total duration" summary).

```sh
everything-cli google youtube transcript https://www.youtube.com/watch?v=dQw4w9WgXcQ
everything-cli google youtube transcript https://youtu.be/dQw4w9WgXcQ --format json
everything-cli google youtube transcript dQw4w9WgXcQ --raw              # plain text on a TTY
everything-cli google youtube transcript dQw4w9WgXcQ --lang de --out de.txt
everything-cli google youtube transcript "https://www.youtube.com/shorts/dQw4w9WgXcQ" | head -20
```

### youtube metadata

- `google youtube metadata <url-or-id>` — the video's player metadata
  plus every caption track it offers. Structured `--format` output
  (table on a TTY). Fields: `video_id`, `title`, `channel`,
  `channel_id`, `duration_seconds`, `view_count`, `publish_date`,
  `upload_date`, `category`, `description`, `available_langs`.

`available_langs` lists one entry per caption track language — human and
ASR duplicates included — so it is the way to discover `transcript
--lang` choices before fetching a track.

```sh
everything-cli google youtube metadata "https://www.youtube.com/watch?v=dQw4w9WgXcQ" --format json
everything-cli google youtube metadata dQw4w9WgXcQ --format table
everything-cli google youtube metadata "https://youtu.be/dQw4w9WgXcQ" --format toon
```

### Unofficial endpoint

Both commands fetch from YouTube's undocumented InnerTube player
endpoint (the Android client context the official mobile app uses) —
not the Google Data API. There is nothing to provision (no OAuth, no
API key), but the endpoint can change or break when YouTube does.
Failures are surfaced as errors, never as silent empty output: do not
treat an absent transcript as an empty success.

### Errors

- `invalid YouTube video ID` — the argument is not a recognized URL
  shape or a valid 11-character ID.
- `no caption tracks available` — the video offers no captions at all
  (not even auto-generated); the error names the video, e.g. `video
  dQw4w9WgXcQ: no caption tracks available`.
- `empty transcript` — the caption track answered HTTP 200 but carried
  no parseable content (YouTube's PoToken gate answers the timedtext URL
  with an empty body). Reported as an error, never as an empty
  transcript.
- `video is not playable: <reason>` — the player endpoint reports a
  non-OK playability status (age-restricted, private, member-only,
  etc.); the wrapped reason is YouTube's own wording.
- Non-200 endpoint statuses and oversized response bodies surface as
  plain errors as well.

## Tips & gotchas (google)

- Destructive verbs refuse to act without `--force`: `google gmail
  message delete`, `google gmail label delete`, `google gmail draft
  delete`, `google account remove`, `google calendar delete`, `google
  calendar event delete`, `google drive file delete`, `google docs
  delete`, `google sheets delete`, `google slides delete`. Prefer the
  trash verbs (`google gmail message trash`, `google drive file trash`)
  over the permanent deletes.
- `--max` budgets API paging (default 25 on gmail list commands and
  `google drive file list`, 250 on calendar event list/instances; `0` =
  no cap).
- `google youtube transcript` deviates from the auto-format table: when
  piped it streams plain caption text (not JSON), and `--raw` forces
  that even on a TTY; only an explicit `--format` renders the structured
  report. `--out <file>` always writes plain text, like `google docs
  get`.
- `google gmail message get --raw` prints the RFC 2822 message with
  control bytes stripped.
