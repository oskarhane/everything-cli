# google-cli

Command-line client for Gmail, Google Calendar, and Google Drive (including Docs, Sheets, and Slides), across multiple Google accounts (Workspace or personal) — plus YouTube video metadata and timed transcripts that need no account at all.

```
google-cli account list
google-cli gmail message list --query "from:boss" --unread-only
google-cli calendar event decline kq3abc123_20260901T100000Z   # one occurrence
google-cli calendar freebusy --from -1d --to +7d
google-cli drive file share 1AbCdEfGh --role reader --email a@x.com
google-cli youtube transcript https://youtu.be/dQw4w9WgXcQ --lang en
```

## Install

```sh
curl -fsSL https://oskarhane.github.io/google-cli/install.sh | sh
```

Installs the latest release binary (with sha256 verification) to `~/.local/bin/google-cli` — make sure that directory is on your `PATH`. After upgrading, run `google-cli update` to check for new releases in the future, or `google-cli skill install` to (re)install the agent skill bundle.

## Build & development

Requires Go (the repo pins a `toolchain` line — any Go ≥ 1.26.2 auto-downloads it).

```sh
make build      # -> bin/google-cli
make test       # hermetic test suite (no network, no real credentials)
make lint       # golangci-lint (govet, errcheck, staticcheck)
make fmt-check  # gofmt gate (CI-failing)
make fmt        # rewrite formatting
```

Smoke tests against a real account (read-only, skip cleanly when none is configured):

```sh
go test -tags=smoke ./test/smoke/... -v
```

Conventions for contributing: see [CLAUDE.md](CLAUDE.md).

## Getting started

1. **OAuth app credentials.** google-cli needs an OAuth *Desktop app* client from a Google Cloud project you control. One app can serve both personal and Workspace accounts.

   a. **Project + APIs (scriptable).** Create the project and enable the APIs from the shell:

   ```sh
   gcloud projects create <project-id>
   gcloud services enable gmail.googleapis.com calendar-json.googleapis.com \
       drive.googleapis.com docs.googleapis.com sheets.googleapis.com slides.googleapis.com \
       --project <project-id>
   ```

   b. **Consent screen (console only).** *APIs & Services → OAuth consent screen*. There is no public API or `gcloud` command for this step (the old `gcloud iap oauth-brands` path is deprecated and forces Internal). Choose the **user type**:

   - **External** — any Google account. Required for personal (gmail.com) accounts, and the right choice when one app should serve both a personal and a work account. Caveat: if the work org's admin restricts untrusted third-party apps (Admin console → *Security → API controls → App access control*), consent for the work account fails with an admin-policy error — the admin must trust your client ID, or you keep a separate org-internal app for the work account and pass `--credentials <path>` on its commands.
   - **Internal** — only accounts inside the same Workspace org (only offered for projects created inside an org).

   **Publish the app** (Production status). In "Testing" status you must list each email as a test user *and refresh tokens expire after 7 days* — weekly re-auth via `account add`. Self-use doesn't need Google's verification: click through the "unverified app" warning once per account.

   c. **Scopes (permissions).** Add these on the consent screen — the default `account add` grant requests them all. Copy-paste block (one per line):

   ```
   https://www.googleapis.com/auth/gmail.modify
   https://www.googleapis.com/auth/gmail.send
   https://www.googleapis.com/auth/gmail.compose
   https://www.googleapis.com/auth/calendar
   https://www.googleapis.com/auth/drive
   https://www.googleapis.com/auth/documents
   https://www.googleapis.com/auth/spreadsheets
   https://www.googleapis.com/auth/presentations
   https://www.googleapis.com/auth/userinfo.email
   ```

   | Scope | Grants |
   |---|---|
   | `https://www.googleapis.com/auth/gmail.modify` | read, label, mark, trash messages |
   | `https://www.googleapis.com/auth/gmail.send` | send mail |
   | `https://www.googleapis.com/auth/gmail.compose` | create/send drafts |
   | `https://www.googleapis.com/auth/calendar` | full Google Calendar (events, ACLs, free/busy) |
   | `https://www.googleapis.com/auth/drive` | full Drive: files and sharing |
   | `https://www.googleapis.com/auth/documents` | edit Google Docs content |
   | `https://www.googleapis.com/auth/spreadsheets` | read/write Google Sheets values |
   | `https://www.googleapis.com/auth/presentations` | edit Google Slides content |
   | `https://www.googleapis.com/auth/userinfo.email` | resolve the account's email after auth |

   Minimal Drive profile: swap `https://www.googleapis.com/auth/drive` for `https://www.googleapis.com/auth/drive.file` (only files this app created or opened) — file read/write commands still work, but the sharing commands (`drive file share`/`unshare`/`permissions`) require the full `drive` scope. A narrower `account add --scopes <csv>` grant is possible too; commands that need a missing scope fail fast with a re-consent action. Ready-to-paste minimal profile:

   ```
   google-cli account add personal --scopes https://www.googleapis.com/auth/gmail.modify,https://www.googleapis.com/auth/gmail.send,https://www.googleapis.com/auth/gmail.compose,https://www.googleapis.com/auth/calendar,https://www.googleapis.com/auth/drive.file,https://www.googleapis.com/auth/documents,https://www.googleapis.com/auth/spreadsheets,https://www.googleapis.com/auth/presentations
   ```

   d. **Client credentials (console only).** *APIs & Services → Credentials → Create Credentials → OAuth client ID → Desktop app* — no API exists for creating client IDs. Download the JSON and put it at `~/.config/google-cli/credentials.json` (or pass `--credentials <path>`). Only this one file is read from the working directory's perspective — the CLI never picks up a `./credentials.json` from the CWD, and it always talks to Google's OAuth endpoints regardless of what the file claims.

2. **Add an account** (opens a browser; loopback redirect, PKCE, CSRF state):

```sh
google-cli account add personal
```

The flow requests Gmail + Calendar + Drive/Docs/Sheets/Slides + userinfo scopes and stores the token at `~/.config/google-cli/accounts/<name>.json` (0600, atomic writes, auto-refreshed on use).

3. **Add more accounts** and switch between them:

```sh
google-cli account add work
google-cli account use work          # set default
google-cli gmail label list --account personal   # one-off override
```

## Usage

### Gmail

```sh
google-cli gmail label list
google-cli gmail label create news --color-bg #fb4c2f

google-cli gmail message list --query "invoice" --max 50
google-cli gmail message get 19c2a4b7 [--raw]     # --raw prints the RFC 2822 source (control bytes stripped)
google-cli gmail message send --to a@x.com --subject "Hi" --body "text" [--attachment report.pdf]
google-cli gmail message mark 19c2a4b7 --read
google-cli gmail message trash 19c2a4b7          # recoverable
google-cli gmail message delete 19c2a4b7 --force # permanent

google-cli gmail thread list / get
google-cli gmail draft create --to a@x.com --subject "Draft" --body-file notes.txt
google-cli gmail draft send draft_19c2a4b7
google-cli gmail attachment get <attachment-id> --message-id <id> [--out report.pdf]
```

### Calendar

```sh
google-cli calendar list
google-cli calendar create "Team Sync" --timezone Europe/Stockholm

google-cli calendar event list --from -1d --to +7d [--recurring instances|masters|all]
google-cli calendar event create --summary "Standup" --start "2026-09-01T09:00" --end "2026-09-01T09:30" \
    --attendee a@x.com --recurrence 'RRULE:FREQ=WEEKLY;COUNT=10'
google-cli calendar event get <id>              # master or <masterId>_<time> instance id

# Recurring events: respond to or delete ONE occurrence (default) or the whole series
google-cli calendar event decline kq3abc123_20260901T100000Z          # just that occurrence
google-cli calendar event decline kq3abc123 --all                    # the series
google-cli calendar event delete kq3abc123_20260901T100000Z --force  # cancels 1 occurrence
google-cli calendar event instances kq3abc123                        # occurrences, now..+7d

google-cli calendar freebusy --from now --to +1d [--calendar work@group.calendar.google.com]
google-cli calendar acl list primary
google-cli calendar acl add primary --scope-user a@x.com --role reader
```

Dates accept RFC3339 (with or without a UTC offset — naive times are read in the local timezone), `YYYY-MM-DD` (with `--all-day`), and relative forms (`now`, `-1d`, `+7d`, `-30m`).

Change-detection pulls: `google-cli calendar event list --updated-since -1d` lists events modified since that time (deletions included, `--show-deleted`, default true).

### Drive, Docs, Sheets, Slides

Drive covers files and sharing; the Docs, Sheets, and Slides trees edit one Google-native document's content by its Drive file id.

```sh
google-cli drive file list --name "invoice" [--parent <folder-id>] [--mime folder] [--trashed]
google-cli drive file get 1AbCdEfGh --format json
google-cli drive file create "Reports"                    # folder; --type doc|sheet|slide or --mime-type <raw>
google-cli drive file upload ./report.pdf [--name "Q3 report" --parent 1AbCdEfGh]
google-cli drive file download 1AbCdEfGh --out report.pdf # Google-native files export instead: --export text/csv
google-cli drive file trash 1AbCdEfGh                     # recoverable (drive file untrash)
google-cli drive file delete 1AbCdEfGh --force            # permanent

google-cli drive file permissions 1AbCdEfGh                              # who it's shared with
google-cli drive file share 1AbCdEfGh --role reader --email a@x.com
google-cli drive file share 1AbCdEfGh --role reader --email a@x.com --expires 2027-01-01T00:00:00Z   # expiry: user/group only
google-cli drive file share 1AbCdEfGh --role commenter --anyone          # anyone with the link
google-cli drive file share 1AbCdEfGh --role writer --domain example.com
google-cli drive file unshare 1AbCdEfGh --email a@x.com                  # or --permission <id>

google-cli docs get 1AbCdEfGh --out notes.txt       # raw text; without --out it streams to stdout
google-cli docs append 1AbCdEfGh --text "Reviewed by Oskar"
google-cli docs insert 1AbCdEfGh --index 1 --text "Q4 plan"   # before a Docs-API content index
google-cli docs replace 1AbCdEfGh --find "Project Falcon" --replace-with "Project Falcon 2"
google-cli docs delete 1AbCdEfGh --force            # permanent; drive file trash is the recoverable path

google-cli sheets get 1AbCdEfGh                     # sheet tabs, grid sizes, header row
google-cli sheets values get 1AbCdEfGh --range "Sheet1!A1:D10"
google-cli sheets values append 1AbCdEfGh --range "Sheet1!A1:D" --values '[[1,"a",true],[2,"b",false]]'
google-cli sheets values update 1AbCdEfGh --range "Sheet1!A1:B2" --values '[[1,"a"],[2,"b"]]'
google-cli sheets values clear 1AbCdEfGh --range "Sheet1!A2:D10"

google-cli slides get 1AbCdEfGh --slide 3           # text per shape; --slide narrows to one slide
google-cli slides replace 1AbCdEfGh --find Acme --replace-with Zenith
google-cli slides delete 1AbCdEfGh --force          # permanent
```

`--values` takes an inline JSON array of arrays; `--values-file` reads the same shape from a `.json`/`.csv`/`.tsv` file instead. Both `sheets values append` and `sheets values update` take `--input-option RAW|USER_ENTERED` (default `USER_ENTERED`, which parses formulas).

Accounts added before Drive support lack the new scopes: re-run `google-cli account add <name>` to consent again — the flow re-prompts and updates that account in place (same name, refreshed token).

### YouTube

Video metadata and timed transcripts — no Google account, OAuth, or API key required. Accepts watch URLs (`?v=...`), youtu.be / shorts / embed / live links, or bare 11-character video IDs. Data comes from YouTube's unofficial InnerTube player endpoint (Android client context), which can break when YouTube changes it — failures surface as errors, never as silent empty output.

```sh
google-cli youtube transcript "https://www.youtube.com/watch?v=dQw4w9WgXcQ"   # plain text when piped, table on a TTY
google-cli youtube transcript dQw4w9WgXcQ --lang de --out de.txt             # German caption track to a file
google-cli youtube transcript dQw4w9WgXcQ --format json                       # timed segments as JSON
google-cli youtube transcript dQw4w9WgXcQ --raw                               # force plain text even on a TTY

google-cli youtube metadata "https://youtu.be/dQw4w9WgXcQ" --format json      # title, channel, views, publish date
google-cli youtube metadata dQw4w9WgXcQ                                       # table: fields incl. available_langs
```

`--lang` (default `en`, ISO 639-1) prefers a human-written caption track, then an auto-generated (ASR) one, then the first available track. `metadata`'s `available_langs` lists every track language — use it to pick a `transcript --lang` value.

### Agent skills

`google-cli` embeds a skill bundle (a `SKILL.md` plus `references/*.md` docs covering accounts, Gmail, Calendar, Drive, Docs, Sheets, Slides, and YouTube) that teaches AI agents how to drive the CLI. Install it into any supported agent's skills directory, remove it again, or print the raw markdown:

```sh
google-cli skill install                        # every detected agent
google-cli skill install --agent claude-code    # only one agent
google-cli skill remove                         # idempotent; no-ops where absent
google-cli skill print > google-cli-skill.md    # bundle as raw markdown
```

`--agent <id>` (on install/remove) scopes the command to one agent, case-insensitive. Supported agents: `claude-code`, `cursor`, `windsurf`, `copilot`, `antigravity`, `gemini-cli`, `cline`, `codex`, `pi`, `opencode`, `junie`.

Detection is by the agent's config directory existing on disk (e.g. `~/.claude`). Installed files live under each agent's skills dir — e.g. `~/.claude/skills/google-cli/` — not under `~/.config/google-cli`.

### Self-update

```sh
google-cli update --check      # report current + latest version, change nothing
google-cli update              # interactive: prompt before skill refresh
google-cli update --yes        # update, auto-install the refreshed skill bundle
google-cli update --yes --agent claude-code   # skill reinstall into one agent
```

`google-cli update` checks GitHub releases for a newer version; if one exists it downloads the tarball matching your platform, verifies it against the release checksum manifest, and atomically replaces the running binary. It then refreshes the installed skill bundle: `[Y/n]` prompt (Yes is the default) at an interactive terminal, automatic with `--yes`, or skipped with a `run: google-cli skill install` hint in non-interactive/agent contexts. `--agent <id>` scopes the post-update skill reinstall, same semantics as `skill install --agent`. Output includes `current_version`, `latest_version`, `update_available`, `binary_path`, and skill fields; table output lists the installed skill paths as one `installed google-cli -> <path>` line each below the summary table instead of a `skill_installed` column. Rate-limit errors suggest `export GITHUB_TOKEN`.

## Output formats

`--format json|table|toon` on read commands; when unset it auto-detects:

| Environment | Format |
|---|---|
| explicit `--format` | wins |
| agent harness (e.g. `CLAUDECODE`) | `toon` |
| interactive terminal | `table` |
| piped / non-TTY | `json` |

Invalid values warn and fall back to auto-detection.

## Global flags

| Flag | Meaning |
|---|---|
| `--account <name>` | act as this account (default: `account use` value) |
| `--credentials <path>` | OAuth client JSON (default: `<config>/credentials.json`) |
| `--format` | output format (see above) |
| `--debug` | redacted debug lines to stderr (credential paths, account names — never token values) |
| `--version` | print `google-cli version <version>` and exit |

## Files

```
~/.config/google-cli/
  credentials.json     # OAuth app client (yours, from Google Cloud)
  config.json          # default account pointer
  accounts/<name>.json # per-account tokens, 0600, atomic writes
```

`$GOOGLE_CLI_CONFIG_DIR` relocates the whole directory (used by tests).

Security posture: OAuth endpoints are hard-pinned to Google, PKCE (S256) on the auth flow, refresh/exchange/API calls carry timeouts, token files are chmod-enforced and written atomically, MIME headers reject CRLF injection, `--raw` output is control-byte stripped, and `govulncheck` runs clean (module-level informational for an uncalled `x/crypto/openpgp` transitive is the only note).
