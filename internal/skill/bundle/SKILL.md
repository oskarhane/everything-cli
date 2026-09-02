---
name: everything-cli
description: >
  everything-cli is a multi-account, multi-provider command-line tool —
  one binary, one set of conventions across SaaS providers. Use it when
  the user wants to work with Gmail, Google Calendar, Google Drive, Docs,
  Sheets, or Slides (provider `google`: send, read, search, label, or
  trash mail; manage calendars, events, invitations, and free/busy; list,
  share, upload, and download Drive files; read or edit Docs, Sheets, and
  Slides); fetch YouTube video metadata and timed transcripts (any watch
  URL or video ID — no account needed); manage Linear issues, teams, and
  projects (provider `linear`); or read Granola notes (provider
  `granola`). Accounts are added, switched, and inspected per provider
  (`everything-cli <provider> account ...`). Skip for Meet, Chrome, or
  anything the gcloud CLI manages (projects, Cloud SDK services).
version: dev
---

# everything-cli

One binary for many SaaS providers, behind one set of conventions. The
command layout is provider-first:

```sh
everything-cli <provider> <resource> <action>
```

e.g. `everything-cli google gmail list`, `everything-cli linear issue
list`. Per-command flags and resource detail live in each provider's
reference file — this document covers only the shared conventions.

## Providers

| Provider | What it covers | Reference |
| --- | --- | --- |
| `google` | Gmail, Calendar, Drive, Docs, Sheets, Slides, YouTube metadata/transcripts; OAuth accounts | [references/google.md](references/google.md) |
| `granola` | Granola notes via the Granola public API; API-key accounts | [references/granola.md](references/granola.md) |
| `linear` | Linear issues, teams, projects; API key or OAuth | [references/linear.md](references/linear.md) |

Every provider also has its own account subtree: `everything-cli
<provider> account add|list|get|use|remove`.

## Accounts

- Multi-account per provider: each named account holds its own secret
  (OAuth token or API key), cached at
  `<config>/accounts/<provider>/<name>.json` (mode 0600). Secrets are
  never printed in any output format — `<provider> account get` shows
  metadata only.
- Defaults are per provider and auto-managed: the first account added for
  a provider becomes its default; removing a default promotes another of
  that provider's accounts. `<provider> account use <name>` is the
  explicit switch for multi-account providers.
- The global `--account <name>` flag overrides the default, resolved
  within the executing provider's accounts only.
- `everything-cli account list` — read-only aggregate of every account
  across all providers (name, provider, identity, is-default). Start here
  to discover what accounts exist anywhere.
- Config dir resolution: `$EVERYTHING_CLI_CONFIG_DIR`, then
  `$GOOGLE_CLI_CONFIG_DIR` (deprecated — warns on stderr), then
  `~/.config/everything-cli`. A relative env value is absolutized against
  the CWD, with a stderr warning; an env-pointed dir's permissions are
  left alone (only dirs the tool creates are tightened to 0700). On first
  run with the default dir, a legacy `~/.config/google-cli` tree is copied
  over (atomically per file, resumable after a crash), so existing Google
  accounts survive the rename with no user action.

## Global flags

| Flag | Values | Meaning |
| --- | --- | --- |
| `--account <name>` | account name | Act as this account instead of the provider's default |
| `--format <fmt>` | `json`, `table`, `toon` | Output format (empty = auto-detect) |
| `--debug` | boolean | Emit debug output (redacted, control-byte stripped) |
| `--version` | boolean | Print `everything-cli version <version>` and exit |

Provider-specific persistent flags live on the provider command, not the
root (e.g. `--credentials` on `google` — see
[references/google.md](references/google.md)).

## Output conventions

- Auto-format precedence when `--format` is not given: explicit flag >
  agent harness detected (→ `toon`) > interactive TTY (→ `table`) >
  piped (→ `json`). Under agent harnesses (e.g. `CLAUDECODE`) output is
  automatically `toon`; pass `--format json` explicitly to get JSON.
- Output field names are snake_case in every format (JSON/TOON keys,
  table headers — table headers render UPPER-CASE).
- Command names and flag long names are kebab-case.
- Paginated reads budget API paging with `--max` (`0` = no cap); the
  defaults are per command in the provider references.

## Back-compat shim

Bare pre-provider invocations (`everything-cli gmail list`,
`everything-cli account add work`) are rewritten to their provider-first
form (`everything-cli google gmail list`, …) with a deprecation warning
on stderr. The shim will be removed in a future release — always write
provider-first commands.

## Skill bundle management

`everything-cli` ships its own agent documentation as an embedded bundle
and can install it into AI agents' skills directories.

- `everything-cli skill install` — install the bundle into every detected
  agent (`--agent <id>` to limit). Clean-slate: replaces any prior
  install.
- `everything-cli skill remove` — delete the installed bundle
  (idempotent).
- `everything-cli skill print` — print the whole bundle as raw markdown
  to stdout (bypasses `--format`).

Detection is by config directory existing on disk (e.g. `~/.claude`).
Supported agents: `claude-code`, `cursor`, `windsurf`, `copilot`,
`antigravity`, `gemini-cli`, `cline`, `codex`, `pi`, `opencode`, `junie`.
Files install under each agent's skills dir (e.g.
`~/.claude/skills/everything-cli/`), never under the everything-cli
config dir.

## Self-update

Fresh install (macOS/Linux, latest release binary to `~/.local/bin`):
`curl -fsSL https://oskarhane.github.io/google-cli/install.sh | sh`.

`everything-cli update` self-updates the binary from GitHub releases,
then refreshes the installed skill bundle — run it after any upgrade so
the bundled agent docs match the binary.

- `everything-cli update` — check GitHub releases; if a newer version
  exists, download the tarball matching this platform, verify its sha256
  against the release checksum manifest, and atomically replace the
  running binary. The refreshed skill bundle is then installed: prompted
  `[Y/n]` (Yes is the default) at an interactive terminal, automatically
  with `--yes`, or not at all in non-interactive/agent contexts (a `run:
  everything-cli skill install` hint is printed instead). Local/`dev`
  builds always offer the latest release. Table output lists the
  installed skill paths as one `installed everything-cli -> <path>` line
  each below the summary table instead of a `skill_installed` column;
  json/toon keep the field.
- `everything-cli update --check` — report current and latest versions
  and change nothing.
- `--yes` — skip confirmation and auto-install the updated skill bundle.
- `--agent <id>` — limit the post-update skill reinstall to one agent
  (same semantics as `skill install --agent`).

Errors read plainly: `no releases published yet` when none exist, and a
hint to `export GITHUB_TOKEN` when the GitHub API rate limit is hit.

## Tips & gotchas

- Destructive verbs across all providers refuse to act without `--force`.
  Prefer trash verbs over permanent deletes where a resource offers both.
- No confirmation prompts anywhere: sharing and delete/trash verbs act
  immediately. Resource ids are explicit on the command line — verify the
  id (via a `list`/`get` first) before running destructive or sharing
  commands.
- Account secrets (OAuth tokens, API keys) are mode 0600 on disk and
  registered for redaction at read time — they never appear in output,
  including `--debug`.
