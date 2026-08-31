---
name: google-cli
description: >
  google-cli is a multi-account command-line tool for Gmail and Google Calendar
  with OAuth and a per-account token cache. Use it when the user wants to send,
  read, search, label, mark, trash, or untrash Gmail messages, threads, drafts,
  labels, or attachments; list or search inbox; create, list, update, move,
  delete, or RSVP (accept/decline/tentative) calendar events and invitations;
  manage calendars, recurring event series and their instances, ACL sharing
  rules, or free/busy time; or add, switch between, and inspect Google
  accounts via OAuth. Skip for Google Drive, Docs, Sheets, Meet, Chrome, or
  anything the gcloud CLI manages (projects, Cloud SDK services) — google-cli
  covers only Gmail and Google Calendar.
version: dev
---

# google-cli

A command-line tool for managing Gmail and Google Calendar across multiple
Google accounts. `google-cli` is the binary name; the command layout is
`google-cli <resource> <action>`.

## Overview

- Multi-account: each named account holds its own OAuth token, cached on disk.
- OAuth device/browser flow at account-add time; access tokens are refreshed
  transparently.
- Token cache at `~/.config/google-cli/accounts/<name>.json` (mode 0600);
  override the location with `$GOOGLE_CLI_CONFIG_DIR`. Token values are never
  printed in any output format.

## Setup

1. Have a Google Cloud OAuth client's `credentials.json` (Desktop app type).
   Place it at the default location or pass `--credentials <path>` /
   `--credentials` on `account add`.
2. `google-cli account add <name>` — runs the OAuth flow, verifies identity,
   and stores the token. `--scopes` overrides the default scope set
   (Gmail modify/send/compose + Calendar + userinfo.email).
3. `google-cli account use <name>` — set the default account. Every command
   also accepts a global `--account <name>` override.

## Global flags

| Flag | Values | Meaning |
| --- | --- | --- |
| `--account <name>` | account name | Act as this account instead of the default |
| `--format <fmt>` | `json`, `table`, `toon` | Output format (empty = auto-detect) |
| `--credentials <path>` | path | OAuth app credentials JSON (empty = auto-resolve) |
| `--debug` | boolean | Emit debug output (redacted, control-byte stripped) |

Auto-format precedence when `--format` is not given: explicit flag > agent
harness detected (→ `toon`) > interactive TTY (→ `table`) > piped (→ `json`).

## Command reference

| Area | File |
| --- | --- |
| Accounts, auth, switching | [references/account.md](references/account.md) |
| Gmail: messages, threads, drafts, labels, attachments | [references/gmail.md](references/gmail.md) |
| Calendars, events, invitations, ACL, free/busy | [references/calendar.md](references/calendar.md) |

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

## Tips & gotchas

- Under agent harnesses (e.g. `CLAUDECODE`) output is automatically `toon`;
  pass `--format json` explicitly to get JSON instead.
- Destructive verbs refuse to act without `--force`: `gmail message delete`,
  `gmail label delete`, `gmail draft delete`, `account remove`,
  `calendar delete`, `calendar event delete`. Prefer `gmail message trash`
  (recoverable) over `gmail message delete` (permanent).
- Recurring events default to ONE occurrence: an instance id ends in
  `_UTC-time` (e.g. `abc123_20260929T030000Z`) and acts on that occurrence
  only. Pass `--all` (respond) or omit `--this-only` (update/delete with an
  instance id) to act on the whole series.
- Token files live at `<config>/accounts/<name>.json` with mode 0600 and are
  never printed — `account get` shows metadata only.
- `--max` budgets API paging (default 25 on gmail list commands, 250 on
  calendar event list/instances; `0` = no cap).
- `gmail message get --raw` prints the RFC 2822 message with control bytes
  stripped.
- `$GOOGLE_CLI_CONFIG_DIR` relocates the whole token cache.
