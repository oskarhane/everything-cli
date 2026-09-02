# CLAUDE.md

Learnings and patterns for future agents working on this project.

## Feedback Instructions

TEST: `make test` · BUILD: `make build` · LINT: `make lint` · FORMAT: `make fmt-check`

**Final gates before done: `make test`, `make fmt-check`, `make lint` — all must pass.** `fmt-check` runs `gofmt -l .` (fails on output); `make fmt` rewrites but is not a gate.

## Project Overview

PRIMARY LANGUAGES: [Go]

`everything-cli` — one command-line tool for many SaaS providers (Google, Linear, Granola), multi-account per provider, behind one set of conventions.

`internal/providers/google/drive/service` is shared by the drive, docs, sheets, and slides trees — dialing, the per-API service seam, and pagination live there; do not duplicate them per resource. The other providers have their own seams (`internal/providers/linear/service`, `internal/providers/granola/service.go` + `dial.go`).

The CLI is provider-first: `everything-cli <provider> <resource> <action>`. Providers self-register via `init()` under `internal/providers/<id>/` (side-effect imports in main.go; registry in `internal/provider`). The Google resource trees (gmail, calendar, drive, docs, sheets, slides, youtube, account) live under `internal/providers/google/`; `internal/subcommands/` holds only CLI-own commands (`skill`, `update`, the read-only cross-provider `account list`, plus the shared `cmdtest`/`gates` test scaffolding). A back-compat shim in `internal/app/shim.go` rewrites bare pre-provider invocations (`gmail list`, `account add`) to `google <args...>` with a stderr deprecation warning. Provider-specific persistent flags live on the provider command, not the root (e.g. `--credentials` on `google`).

Auth strategies live in `internal/auth`: the OAuth flow/token machinery plus a pluggable strategy registry (`strategy.go`) — `internal/auth/apikey` is the API-key strategy used by `linear` (default) and `granola`; `linear` additionally has its own OAuth flow (`internal/providers/linear/oauth.go`).

Adding a provider = `internal/providers/<id>/` (provider.go with an `init()` registration) + one side-effect import in main.go + `internal/skill/bundle/references/<id>.md` + a provider-index row in `internal/skill/bundle/SKILL.md`. `TestProviderDocsDrift` (internal/skill/drift_test.go) enforces the SKILL.md index link and the references file for every registered provider.

## Cobra Command Layout

Strict one-file-per-leaf layout under `internal/providers/<id>/<resource>/` (CLI-own commands under `internal/subcommands/<resource>/`). Mirror it for new trees.

- Parent `<resource>.go` — `NewCmd(cfg, ...)`, persistent flags, `cmd.AddCommand(newXxxCmd(...))` per leaf. ≤80 lines. Name it after the resource (not `command.go`).
- Leaf `<action>.go` — private constructor (`newListCmd(...)`) with the leaf's flags + `RunE`. No leaf bodies in the parent.
- Colocated `<action>_test.go`; shared helpers in `<resource>_helpers_test.go`.
- New leaf = `<action>.go` + `<action>_test.go` + one `AddCommand` line.

## CLI Conventions

- Singular nouns; `<resource> <action>`; ≤1 positional (extras → flags); `--format json|table|toon` on reads.
- Follow https://clig.dev/.

## Docs Sync

Feature work that adds or changes commands, flags, output fields, or defaults MUST also update the embedded skill (`internal/skill/bundle/SKILL.md` + `internal/skill/bundle/references/*.md`) and `README.md` in the same change. Verify before done; `make test` (incl. skill drift tests) must pass.

## Output / TTY / Casing

- Output format auto-detection precedence: explicit `--format` flag > agent harness (`IsAgent` → `toon`) > TTY (`table`) > `json`. Agent detection reads env (e.g. `CLAUDECODE`); tests seed `IsAgent = func() bool { return false }` via TestMain so the host's harness env can't flip expectations.
- `toon` marshal rejects C0 control bytes (accepts `\t\n\r`). Strip control bytes (recursively) before marshal and fall back to JSON on residual error — never panic on data-driven marshal failure.
- The table printer (go-pretty `StyleLight`) UPPER-CASES header cells — table-output tests must assert upper-case headers, not lower-case.
- **Casing**: OUTPUT field names = snake_case (JSON/TOON keys, table headers). INPUT identifiers = kebab-case (`Use`, aliases, flag long names; single-char shorthands exempt). Exemptions: wire/parse structs (Google API types, OAuth), config keys, enum values.

## Testing Framework

TESTING FRAMEWORKS: [Go testing, testify, afero in-memory FS].

- Colocated `*_test.go`; prefer table-driven tests. Hermetic tests — no network.
- Name test files per command (`get_test.go`, not one big `config_test.go`); shared helpers in `helpers_test.go`. The rule is anti-monolith, NOT literal one-file-per-command: a concern-named file aggregating a single cross-command concern is fine.
- **Never `afero.NewOsFs()` in tests that touch credential/token paths** — the dev machine has real tokens at `~/.config/everything-cli` (and the legacy `~/.config/google-cli`). Use an empty in-memory FS (`afero.NewMemMapFs()`).

## Build System

BUILD SYSTEMS: [Go toolchain, Makefile, golangci-lint].

- `make build` → `bin/everything-cli`.
- Final gates: `make test`, `make fmt-check`, `make lint`.
- gofmt discipline: `fmt-check` runs `gofmt -l .` and fails on any output; run `make fmt` to rewrite before committing.

## Secrets / Redaction

- OAuth access/refresh tokens, OAuth app client_secrets, and provider API keys are secrets: never print them. `<provider> account get` shows account metadata only — never token or key values.
- The redaction registry lives in the leaf package `internal/redact` (re-exported as `auth.RegisterSecret`/`auth.Redact` so auth and provider strategies don't import the leaf directly). It is a process-global set of exact secret values; `Redact` does whole-text substring replacement to `***` — no shape-based heuristics, so JSON fields, table cells, and TOON rows are covered alike. With no secrets registered it short-circuits and output passes through untouched.
- Every secret is registered at its mint/read point — the moment the value enters the process (token save/load/refresh, API-key read, `ParseClientCredentials` for the Google client_secret) — never at print time.
- Emission points that pass through the redactor: `output.writeLine` (the single chokepoint under `Print`/`PrintJSON`/`PrintTable`/`PrintToon` and `output.Debug`) and the top-level error print — main sets `root.SilenceErrors = true` and prints via `app.PrintError`, which redacts before writing stderr.
- `--debug` output passes through control-byte stripping (`StripControl`) + redaction before emission.
