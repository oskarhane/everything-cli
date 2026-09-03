# everything-cli

One command-line tool for many SaaS providers — Google (Gmail, Calendar, Drive, Docs, Sheets, Slides, YouTube), Linear, Granola, and regular IMAP/SMTP email — behind one set of conventions, with multi-account support per provider. Built to be agent-friendly: every read command supports `--format json|table|toon`, and output auto-detects agent harnesses (e.g. `CLAUDECODE`) and switches to token-efficient `toon` automatically.

The command layout is provider-first:

```
everything-cli <provider> <resource> <action>
```

```
everything-cli account list                                        # every account, every provider
everything-cli google gmail message list --query "from:boss" --unread-only
everything-cli google calendar event decline kq3abc123_20260901T100000Z   # one occurrence
everything-cli google calendar freebusy --from -1d --to +7d
everything-cli google drive file share 1AbCdEfGh --role reader --email a@x.com
everything-cli google youtube transcript https://youtu.be/dQw4w9WgXcQ --lang en
everything-cli linear issue list --team 9c1e2f3a-... --format json
everything-cli granola note list --created-after 2026-08-01
everything-cli email message list --mailbox INBOX --limit 10
everything-cli email message send --to a@x.com --subject "Hi" --body "hello"
```

## Providers

| Provider | Covers | Auth |
|---|---|---|
| `google` | Gmail, Calendar, Drive, Docs, Sheets, Slides, YouTube metadata/transcripts | Google OAuth (your own OAuth Desktop-app client; per-account token cache). YouTube needs no account at all. |
| `linear` | Linear issues, teams, projects | Personal API key **or** OAuth (browser flow with PKCE) |
| `granola` | Granola notes (read-only) | Official `grn_` API key — requires a Granola **Business or Enterprise** plan |
| `email` | Regular email: IMAP reads (mailboxes, message list/get) and SMTP send | Username + password per account |

Every provider has its own account subtree (`everything-cli <provider> account add|list|get|use|remove`); see [Accounts](#accounts).

## Install

```sh
curl -fsSL https://oskarhane.github.io/everything-cli/install.sh | sh
```

Installs the latest release binary (with sha256 verification) to `~/.local/bin` — make sure that directory is on your `PATH`. Note: the repo was renamed from `google-cli`, but until the first `everything-cli` release is cut the installer still delivers the last `google-cli`-named release (v0.3.0, binary `google-cli`) — build from source with `make build` (→ `bin/everything-cli`) to run the new CLI today.

After installing, run `everything-cli skill install` to install the agent skill bundle, and `everything-cli update` later to check for new releases (on the v0.3.0 binary, substitute `google-cli`).

## Quick start per provider

### Google (OAuth)

The `google` provider needs an OAuth *Desktop app* client from a Google Cloud project you control (one app can serve both personal and Workspace accounts), then one `account add` per Google account. The full provisioning walkthrough — project + API enablement, consent screen, scopes, client credentials — is in [Google OAuth provisioning](#google-oauth-provisioning) below, and in the embedded skill's [google reference](internal/skill/bundle/references/google.md).

```sh
everything-cli google account add personal        # browser OAuth flow (loopback redirect, PKCE)
everything-cli google account add work            # a second account
everything-cli google account use work            # set the provider default
everything-cli google gmail label list --account personal   # one-off override
```

### Linear (API key or OAuth)

```sh
# Personal API key (https://linear.app/settings/account/security) — hidden prompt
everything-cli linear account add work

# ...or non-interactively from the environment
LINEAR_API_KEY=lin_api_... everything-cli linear account add work

# ...or OAuth (browser flow with PKCE; app from https://linear.app/settings/api/applications)
everything-cli linear account add work --oauth --client-id 7231...
```

### Granola (`grn_` API key)

Requires a Granola **Business or Enterprise** plan — personal plans cannot create API keys. Create a key in the Granola desktop app (**Settings → Connectors → API keys → Create new key**), then:

```sh
everything-cli granola account add work                       # hidden prompt
GRANOLA_API_KEY=grn_... everything-cli granola account add work  # non-interactive
```

### Email (IMAP/SMTP username + password)

```sh
everything-cli email account add work \
    --imap-host imap.example.com --smtp-host smtp.example.com --username me@example.com  # hidden password prompt
EMAIL_PASSWORD=... everything-cli email account add work \
    --imap-host imap.example.com --smtp-host smtp.example.com --username me@example.com  # non-interactive

everything-cli email mailbox list
everything-cli email message list --limit 10
everything-cli email message send --to alice@example.com --subject "Hi" --body "hello"
```

### Verify your accounts

```sh
everything-cli account list        # every account across all providers, defaults marked
```

## Google OAuth provisioning

The `google` provider talks to Google with an OAuth *Desktop app* client from a Google Cloud project you control. Setup, once per OAuth app:

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

**Publish the app** (Production status). In "Testing" status you must list each email as a test user *and refresh tokens expire after 7 days* — weekly re-auth via `google account add`. Self-use doesn't need Google's verification: click through the "unverified app" warning once per account.

c. **Scopes (permissions).** Add these on the consent screen — the default `google account add` grant requests them all. Copy-paste block (one per line):

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

Minimal Drive profile: swap `https://www.googleapis.com/auth/drive` for `https://www.googleapis.com/auth/drive.file` (only files this app created or opened) — file read/write commands still work, but the sharing commands (`google drive file share`/`unshare`/`permissions`) require the full `drive` scope. A narrower `google account add --scopes <csv>` grant is possible too; commands that need a missing scope fail fast with a re-consent action. Ready-to-paste minimal profile:

```
everything-cli google account add personal --scopes https://www.googleapis.com/auth/gmail.modify,https://www.googleapis.com/auth/gmail.send,https://www.googleapis.com/auth/gmail.compose,https://www.googleapis.com/auth/calendar,https://www.googleapis.com/auth/drive.file,https://www.googleapis.com/auth/documents,https://www.googleapis.com/auth/spreadsheets,https://www.googleapis.com/auth/presentations
```

d. **Client credentials (console only).** *APIs & Services → Credentials → Create Credentials → OAuth client ID → Desktop app* — no API exists for creating client IDs. Download the JSON and put it at `~/.config/everything-cli/credentials.json` (or pass `--credentials <path>`). Only this one file is read from the working directory's perspective — the CLI never picks up a `./credentials.json` from the CWD, and it always talks to Google's OAuth endpoints regardless of what the file claims.

The flow stores the token at `~/.config/everything-cli/accounts/google/<name>.json` (0600, atomic writes, auto-refreshed on use). Accounts added before Drive support lack the new scopes: re-run `everything-cli google account add <name>` to consent again — the flow re-prompts and updates that account in place (same name, refreshed token).

## Accounts

- **Multi-account per provider.** Each named account holds its own secret (OAuth token or API key) at `<config>/accounts/<provider>/<name>.json` (mode 0600, atomic writes). Secrets are never printed — `<provider> account get` shows metadata only.
- **Defaults are per provider and auto-managed.** The first account added for a provider becomes its default; removing a default promotes another of that provider's accounts (announced as `default account is now <name>`). `<provider> account use <name>` is the explicit switch.
- The global `--account <name>` flag overrides the default, resolved within the executing provider's accounts only.
- `everything-cli account list` is a read-only aggregate of every account across all providers (name, provider, identity, default) — start here to discover what's configured anywhere.

## Usage

### Google: Gmail

```sh
everything-cli google gmail label list
everything-cli google gmail label create news --color-bg #fb4c2f

everything-cli google gmail message list --query "invoice" --max 50
everything-cli google gmail message get 19c2a4b7 [--raw]     # --raw prints the RFC 2822 source (control bytes stripped)
everything-cli google gmail message send --to a@x.com --subject "Hi" --body "text" [--attachment report.pdf]
everything-cli google gmail message mark 19c2a4b7 --read
everything-cli google gmail message trash 19c2a4b7          # recoverable
everything-cli google gmail message delete 19c2a4b7 --force # permanent

everything-cli google gmail thread list / get
everything-cli google gmail draft create --to a@x.com --subject "Draft" --body-file notes.txt
everything-cli google gmail draft send draft_19c2a4b7
everything-cli google gmail attachment get <attachment-id> --message-id <id> [--out report.pdf]
```

### Google: Calendar

```sh
everything-cli google calendar list
everything-cli google calendar create "Team Sync" --timezone Europe/Stockholm

everything-cli google calendar event list --from -1d --to +7d [--recurring instances|masters|all]
everything-cli google calendar event create --summary "Standup" --start "2026-09-01T09:00" --end "2026-09-01T09:30" \
    --attendee a@x.com --recurrence 'RRULE:FREQ=WEEKLY;COUNT=10'
everything-cli google calendar event get <id>              # master or <masterId>_<time> instance id

# Recurring events: respond to or delete ONE occurrence (default) or the whole series
everything-cli google calendar event decline kq3abc123_20260901T100000Z          # just that occurrence
everything-cli google calendar event decline kq3abc123 --all                    # the series
everything-cli google calendar event delete kq3abc123_20260901T100000Z --force  # cancels 1 occurrence
everything-cli google calendar event instances kq3abc123                        # occurrences, now..+7d

everything-cli google calendar freebusy --from now --to +1d [--calendar work@group.calendar.google.com]
everything-cli google calendar acl list primary
everything-cli google calendar acl add primary --scope-user a@x.com --role reader
```

Dates accept RFC3339 (with or without a UTC offset — naive times are read in the local timezone), `YYYY-MM-DD` (with `--all-day`), and relative forms (`now`, `-1d`, `+7d`, `-30m`).

Change-detection pulls: `everything-cli google calendar event list --updated-since -1d` lists events modified since that time (deletions included, `--show-deleted`, default true).

### Google: Drive, Docs, Sheets, Slides

Drive covers files and sharing; the Docs, Sheets, and Slides trees edit one Google-native document's content by its Drive file id.

```sh
everything-cli google drive file list --name "invoice" [--parent <folder-id>] [--mime folder] [--trashed]
everything-cli google drive file get 1AbCdEfGh --format json
everything-cli google drive file create "Reports"                    # folder; --type doc|sheet|slide or --mime-type <raw>
everything-cli google drive file upload ./report.pdf [--name "Q3 report" --parent 1AbCdEfGh]
everything-cli google drive file download 1AbCdEfGh --out report.pdf # Google-native files export instead: --export text/csv
everything-cli google drive file trash 1AbCdEfGh                     # recoverable (drive file untrash)
everything-cli google drive file delete 1AbCdEfGh --force            # permanent

everything-cli google drive file permissions 1AbCdEfGh                              # who it's shared with
everything-cli google drive file share 1AbCdEfGh --role reader --email a@x.com
everything-cli google drive file share 1AbCdEfGh --role reader --email a@x.com --expires 2027-01-01T00:00:00Z   # expiry: user/group only
everything-cli google drive file share 1AbCdEfGh --role commenter --anyone          # anyone with the link
everything-cli google drive file share 1AbCdEfGh --role writer --domain example.com
everything-cli google drive file unshare 1AbCdEfGh --email a@x.com                  # or --permission <id>

everything-cli google docs get 1AbCdEfGh --out notes.txt       # raw text; without --out it streams to stdout
everything-cli google docs append 1AbCdEfGh --text "Reviewed by Oskar"
everything-cli google docs insert 1AbCdEfGh --index 1 --text "Q4 plan"   # before a Docs-API content index
everything-cli google docs replace 1AbCdEfGh --find "Project Falcon" --replace-with "Project Falcon 2"
everything-cli google docs delete 1AbCdEfGh --force            # permanent; drive file trash is the recoverable path

everything-cli google sheets get 1AbCdEfGh                     # sheet tabs, grid sizes, header row
everything-cli google sheets values get 1AbCdEfGh --range "Sheet1!A1:D10"
everything-cli google sheets values append 1AbCdEfGh --range "Sheet1!A1:D" --values '[[1,"a",true],[2,"b",false]]'
everything-cli google sheets values update 1AbCdEfGh --range "Sheet1!A1:B2" --values '[[1,"a"],[2,"b"]]'
everything-cli google sheets values clear 1AbCdEfGh --range "Sheet1!A2:D10"

everything-cli google slides get 1AbCdEfGh --slide 3           # text per shape; --slide narrows to one slide
everything-cli google slides replace 1AbCdEfGh --find Acme --replace-with Zenith
everything-cli google slides delete 1AbCdEfGh --force          # permanent
```

`--values` takes an inline JSON array of arrays; `--values-file` reads the same shape from a `.json`/`.csv`/`.tsv` file instead. Both `google sheets values append` and `google sheets values update` take `--input-option RAW|USER_ENTERED` (default `USER_ENTERED`, which parses formulas).

### Google: YouTube

Video metadata and timed transcripts — no Google account, OAuth, or API key required. Accepts watch URLs (`?v=...`), youtu.be / shorts / embed / live links, or bare 11-character video IDs. Data comes from YouTube's unofficial InnerTube player endpoint (Android client context), which can break when YouTube changes it — failures surface as errors, never as silent empty output.

```sh
everything-cli google youtube transcript "https://www.youtube.com/watch?v=dQw4w9WgXcQ"   # plain text when piped, table on a TTY
everything-cli google youtube transcript dQw4w9WgXcQ --lang de --out de.txt             # German caption track to a file
everything-cli google youtube transcript dQw4w9WgXcQ --format json                       # timed segments as JSON
everything-cli google youtube transcript dQw4w9WgXcQ --raw                               # force plain text even on a TTY

everything-cli google youtube metadata "https://youtu.be/dQw4w9WgXcQ" --format json      # title, channel, views, publish date
everything-cli google youtube metadata dQw4w9WgXcQ                                       # table: fields incl. available_langs
```

`--lang` (default `en`, ISO 639-1) prefers a human-written caption track, then an auto-generated (ASR) one, then the first available track. `metadata`'s `available_langs` lists every track language — use it to pick a `transcript --lang` value.

### Linear

Issues, teams, and projects over Linear's GraphQL API.

```sh
everything-cli linear team list                              # find a team ID
everything-cli linear project list

everything-cli linear issue list [--team 9c1e2f3a-...]       # empty --team = workspace-wide
everything-cli linear issue get BLA-123
everything-cli linear issue create --team 9c1e2f3a-... --title "Fix login redirect" \
    [--description "..." --assignee 4d5e6f7a-... --state 8b9c0d1e-...]
everything-cli linear issue update BLA-123 --state 8b9c0d1e-... [--title ... --assignee ...]
```

### Granola

Read-only: list and get notes (with AI summaries) via the official Granola public API.

```sh
everything-cli granola note list [--created-after 2026-08-01] [--created-before 2026-09-01] [--folder-id fol_...]
everything-cli granola note get not_abc123def456 [--include-transcript]
```

### Email

Regular email over IMAP (read) and SMTP (send). IMAP UIDs are per-mailbox, so `--mailbox` scopes both `message list` and `message get`.

```sh
everything-cli email mailbox list                                        # mailbox (folder) names
everything-cli email message list [--mailbox INBOX] [--limit 25]         # envelopes: uid, date, from, subject, flags
everything-cli email message get 42 [--mailbox Archive]                  # full message + attachment metadata
everything-cli email message send --to a@x.com --to b@x.com [--cc c@x.com] \
    --subject "Report" (--body "text" | --body-file report.txt | --body-file -)   # - reads stdin
```

## Agent skills

`everything-cli` embeds a skill bundle (a `SKILL.md` router plus per-provider `references/*.md` deep docs — this is the agent-facing documentation) that teaches AI agents how to drive the CLI. Install it into any supported agent's skills directory, remove it again, or print the raw markdown:

```sh
everything-cli skill install                        # every detected agent
everything-cli skill install --agent claude-code    # only one agent
everything-cli skill remove                         # idempotent; no-ops where absent
everything-cli skill print > everything-cli-skill.md    # bundle as raw markdown
```

`--agent <id>` (on install/remove) scopes the command to one agent, case-insensitive. Supported agents: `claude-code`, `cursor`, `windsurf`, `copilot`, `antigravity`, `gemini-cli`, `cline`, `codex`, `pi`, `opencode`, `junie`.

Detection is by the agent's config directory existing on disk (e.g. `~/.claude`). Installed files live under each agent's skills dir — e.g. `~/.claude/skills/everything-cli/` — not under `~/.config/everything-cli`.

## Self-update

```sh
everything-cli update --check      # report current + latest version, change nothing
everything-cli update              # interactive: prompt before skill refresh
everything-cli update --yes        # update, auto-install the refreshed skill bundle
everything-cli update --yes --agent claude-code   # skill reinstall into one agent
```

`everything-cli update` checks GitHub releases for a newer version; if one exists it downloads the tarball matching your platform, verifies it against the release checksum manifest, and atomically replaces the running binary. It then refreshes the installed skill bundle: `[Y/n]` prompt (Yes is the default) at an interactive terminal, automatic with `--yes`, or skipped with a `run: everything-cli skill install` hint in non-interactive/agent contexts. `--agent <id>` scopes the post-update skill reinstall, same semantics as `skill install --agent`. Output includes `current_version`, `latest_version`, `update_available`, `binary_path`, and skill fields; table output lists the installed skill paths as one `installed everything-cli -> <path>` line each below the summary table instead of a `skill_installed` column. Rate-limit errors suggest `export GITHUB_TOKEN`.

## Output formats

`--format json|table|toon` on read commands; when unset it auto-detects:

| Environment | Format |
|---|---|
| explicit `--format` | wins |
| agent harness (e.g. `CLAUDECODE`) | `toon` |
| interactive terminal | `table` |
| piped / non-TTY | `json` |

Invalid values warn and fall back to auto-detection. Output field names are snake_case in every format (table headers render UPPER-CASE).

## Global flags

| Flag | Meaning |
|---|---|
| `--account <name>` | act as this account (default: the executing provider's default account) |
| `--format` | output format (see above) |
| `--debug` | redacted debug lines to stderr (credential paths, account names — never token or key values) |
| `--version` | print `everything-cli version <version>` and exit |

Provider-specific persistent flags live on the provider command, not the root — e.g. `--credentials <path>` (OAuth client JSON) on `everything-cli google`.

## Files

```
~/.config/everything-cli/
  credentials.json                 # Google OAuth app client (yours, from Google Cloud)
  config.json                      # per-provider default account pointers
  accounts/<provider>/<name>.json  # per-account secrets (tokens, API keys), 0600, atomic writes
```

`$EVERYTHING_CLI_CONFIG_DIR` relocates the whole directory (used by tests). `$GOOGLE_CLI_CONFIG_DIR` is still honored as a deprecated fallback, with a stderr warning. A relative env value is absolutized against the current working directory, with a stderr warning. An env-pointed directory keeps its existing permissions — only directories the tool creates are tightened to 0700.

**Migration from `google-cli`:** on first run with the default config dir, a legacy `~/.config/google-cli` tree is copied over automatically, so existing Google accounts survive the rename with no user action. Legacy flat `accounts/<name>.json` files load as `google` accounts and are rewritten to the nested layout on save.

Security posture: OAuth endpoints are hard-pinned to the provider, PKCE (S256) on auth flows, refresh/exchange/API calls carry timeouts, secret files are chmod-enforced and written atomically, API keys are captured via hidden prompt or env var and registered for redaction at capture/read time, MIME headers reject CRLF injection, `--raw` output is control-byte stripped, and `govulncheck` runs clean (module-level informational for an uncalled `x/crypto/openpgp` transitive is the only note).

## Build & development

Requires Go (the repo pins a `toolchain` line — any Go ≥ 1.26.2 auto-downloads it).

```sh
make build      # -> bin/everything-cli
make test       # hermetic test suite (no network, no real credentials)
make lint       # golangci-lint (govet, errcheck, staticcheck)
make fmt-check  # gofmt gate (CI-failing)
make fmt        # rewrite formatting
```

Smoke tests against a real account (read-only, skip cleanly when none is configured):

```sh
go test -tags=smoke ./test/smoke/... -v
```

Conventions for contributing: see [AGENTS.md](AGENTS.md).
