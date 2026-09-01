---
name: google-cli
description: >
  google-cli is a multi-account command-line tool for Gmail, Google Calendar,
  Google Drive, Docs, Sheets, and Slides with OAuth and a per-account token
  cache. Use it when the user wants to send, read, search, label, mark, trash,
  or untrash Gmail messages, threads, drafts, labels, or attachments; list or
  search inbox; create, list, update, move, delete, or RSVP
  (accept/decline/tentative) calendar events and invitations; manage
  calendars, recurring event series and their instances, ACL sharing rules,
  or free/busy time; or list, get, create, upload, download, trash, delete,
  share, and unshare Drive files, and read or edit Google Docs (text),
  Spreadsheets (values), and Presentations (text); or add, switch between,
  and inspect Google accounts via OAuth; or fetch YouTube video metadata
  and timed transcripts with `youtube metadata` / `youtube transcript`
  (any watch URL or video ID — no account or OAuth needed). Skip for Meet,
  Chrome, or anything the gcloud CLI manages (projects, Cloud SDK services)
  — google-cli covers only Gmail, Google Calendar, Drive, Docs, Sheets,
  Slides, and YouTube metadata/transcripts.
version: dev
---

# google-cli

A command-line tool for managing Gmail, Google Calendar, Drive, Docs,
Sheets, and Slides across multiple Google accounts — and for YouTube video
metadata and timed transcripts without any account. `google-cli` is the
binary name; the command layout is `google-cli <resource> <action>`.

## Overview

- Multi-account: each named account holds its own OAuth token, cached on disk.
- OAuth device/browser flow at account-add time; access tokens are refreshed
  transparently.
- Token cache at `~/.config/google-cli/accounts/<name>.json` (mode 0600);
  override the location with `$GOOGLE_CLI_CONFIG_DIR`. Token values are never
  printed in any output format.
- No-account YouTube: `youtube transcript` and `youtube metadata` work on any
  watch URL (`?v=...`), youtu.be / shorts / embed / live link, or bare
  11-character video ID, via YouTube's unofficial InnerTube endpoint — no
  OAuth, no API key of your own (details and caveats in
  [references/youtube.md](references/youtube.md)).

## Setup

1. Have a Google Cloud OAuth client's `credentials.json` (Desktop app type).
   Place it at the default location or pass `--credentials <path>` /
   `--credentials` on `account add`. Provisioning the app (consent-screen
   user type, External vs Internal, publishing status, the full scope list)
   is covered in the README's *Getting started* section.
2. `google-cli account add <name>` — runs the OAuth flow, verifies identity,
   and stores the token. `--scopes` overrides the default scope set
   (Gmail modify/send/compose + Calendar + Drive + Docs + Sheets + Slides +
   userinfo.email). Minimal Drive profile: swap `Drive` for
   `https://www.googleapis.com/auth/drive.file` to reach only app-created
   files, e.g. `google-cli account add work --scopes
   https://www.googleapis.com/auth/gmail.modify,...,https://www.googleapis.com/auth/drive.file`
   (details in [references/account.md](references/account.md)); sharing
   commands (`drive file share`/`unshare`/`permissions`) still require the
   full drive scope.
3. `google-cli account use <name>` — set the default account. Every command
   also accepts a global `--account <name>` override.

## Global flags

| Flag | Values | Meaning |
| --- | --- | --- |
| `--account <name>` | account name | Act as this account instead of the default |
| `--format <fmt>` | `json`, `table`, `toon` | Output format (empty = auto-detect) |
| `--credentials <path>` | path | OAuth app credentials JSON (empty = auto-resolve) |
| `--debug` | boolean | Emit debug output (redacted, control-byte stripped) |
| `--version` | boolean | Print `google-cli version <version>` and exit |

Auto-format precedence when `--format` is not given: explicit flag > agent
harness detected (→ `toon`) > interactive TTY (→ `table`) > piped (→ `json`).

## Command reference

| Area | File |
| --- | --- |
| Accounts, auth, switching | [references/account.md](references/account.md) |
| Gmail: messages, threads, drafts, labels, attachments | [references/gmail.md](references/gmail.md) |
| Calendars, events, invitations, ACL, free/busy | [references/calendar.md](references/calendar.md) |
| Drive files: list, get, create, upload, download, trash, delete | [references/drive.md](references/drive.md) |
| Drive file sharing: permissions, share, unshare | [references/drive.md](references/drive.md) |
| Docs: read and edit Google Doc text | [references/docs.md](references/docs.md) |
| Sheets: spreadsheet metadata and cell values | [references/sheets.md](references/sheets.md) |
| Slides: presentation text | [references/slides.md](references/slides.md) |
| YouTube: video metadata and timed transcripts (`youtube metadata`, `youtube transcript`) | [references/youtube.md](references/youtube.md) |

## Skill bundle management

`google-cli` ships its own agent documentation as an embedded bundle and can
install it into AI agents' skills directories.

- `google-cli skill install` — install the bundle into every detected agent
  (`--agent <id>` to limit). Clean-slate: replaces any prior install.
- `google-cli skill remove` — delete the installed bundle (idempotent).
- `google-cli skill print` — print the whole bundle as raw markdown to
  stdout (bypasses `--format`).

Detection is by config directory existing on disk (e.g. `~/.claude`).
Supported agents: `claude-code`, `cursor`, `windsurf`, `copilot`,
`antigravity`, `gemini-cli`, `cline`, `codex`, `pi`, `opencode`, `junie`.
Files install under each agent's skills dir (e.g. `~/.claude/skills/google-cli/`),
never under `~/.config/google-cli`.

## Self-update

Fresh install (macOS/Linux, latest release binary to `~/.local/bin`):
`curl -fsSL https://oskarhane.github.io/google-cli/install.sh | sh`.

`google-cli update` self-updates the binary from GitHub releases, then
refreshes the installed skill bundle — run it after any upgrade so the
bundled agent docs match the binary.

- `google-cli update` — check GitHub releases; if a newer version exists,
  download the tarball matching this platform, verify its sha256 against
  the release checksum manifest, and atomically replace the running
  binary. The refreshed skill bundle is then installed: prompted
  `[y/N]` at an interactive terminal, automatically with `--yes`, or not
  at all in non-interactive/agent contexts (a `run: google-cli skill
  install` hint is printed instead). Local/`dev` builds always offer the
  latest release.
- `google-cli update --check` — report current and latest versions and
  change nothing.
- `--yes` — skip confirmation and auto-install the updated skill bundle.
- `--agent <id>` — limit the post-update skill reinstall to one agent
  (same semantics as `skill install --agent`).

Errors read plainly: `no releases published yet` when none exist, and a
hint to `export GITHUB_TOKEN` when the GitHub API rate limit is hit.

## Tips & gotchas

- Under agent harnesses (e.g. `CLAUDECODE`) output is automatically `toon`;
  pass `--format json` explicitly to get JSON instead.
- Destructive verbs refuse to act without `--force`: `gmail message delete`,
  `gmail label delete`, `gmail draft delete`, `account remove`,
  `calendar delete`, `calendar event delete`, `drive file delete`,
  `docs delete`, `sheets delete`, `slides delete`. Prefer the trash verbs
  (`gmail message trash`, `drive file trash`) over the permanent deletes.
- No confirmation prompts anywhere: `drive file share`/`unshare` and every
  delete/trash verb act immediately. Resource ids are explicit on the
  command line — verify the id (via a `list`/`get` first) before running
  destructive or sharing commands.
- Accounts added before Drive/Docs/Sheets/Slides support lack the new
  scopes: re-run `account add <name>` with the same name to grant them
  (the flow always re-prompts with `prompt=consent`, and the account is
  updated in place, keyed by email).
- Recurring events default to ONE occurrence: an instance id ends in
  `_UTC-time` (e.g. `abc123_20260929T030000Z`) and acts on that occurrence
  only. Pass `--all` (respond) or omit `--this-only` (update/delete with an
  instance id) to act on the whole series.
- Token files live at `<config>/accounts/<name>.json` with mode 0600 and are
  never printed — `account get` shows metadata only.
- `--max` budgets API paging (default 25 on gmail list commands and
  `drive file list`, 250 on calendar event list/instances; `0` = no cap).
- `youtube transcript` deviates from the auto-format table above: when
  piped it streams plain caption text (not JSON), and `--raw` forces that
  even on a TTY; only an explicit `--format` renders the structured
  report. `--out <file>` always writes plain text, like `docs get`.
- `gmail message get --raw` prints the RFC 2822 message with control bytes
  stripped.
- `$GOOGLE_CLI_CONFIG_DIR` relocates the whole token cache.
