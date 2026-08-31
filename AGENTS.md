# CLAUDE.md

Learnings and patterns for future agents working on this project.

## Feedback Instructions

TEST: `make test` · BUILD: `make build` · LINT: `make lint` · FORMAT: `make fmt-check`

**Final gates before done: `make test`, `make fmt-check`, `make lint` — all must pass.** `fmt-check` runs `gofmt -l .` (fails on output); `make fmt` rewrites but is not a gate.

## Project Overview

PRIMARY LANGUAGES: [Go]

`google-cli` — command-line tool for managing Gmail and Google Calendar across multiple Google accounts (OAuth, per-account token cache).

## Cobra Command Layout

Strict one-file-per-leaf layout under `internal/subcommands/<resource>/`. Mirror it for new trees.

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
- **Never `afero.NewOsFs()` in tests that touch credential/token paths** — the dev machine has real tokens at `~/.config/google-cli`. Use an empty in-memory FS (`afero.NewMemMapFs()`).

## Build System

BUILD SYSTEMS: [Go toolchain, Makefile, golangci-lint].

- `make build` → `bin/google-cli`.
- Final gates: `make test`, `make fmt-check`, `make lint`.
- gofmt discipline: `fmt-check` runs `gofmt -l .` and fails on any output; run `make fmt` to rewrite before committing.

## Secrets / Redaction

- OAuth access and refresh tokens are secrets: never print them. `account get` shows account metadata only — never token values.
- Any secret value that must appear in output is registered for redaction BEFORE printing in table/toon (the JSON field regex closes it only for `--format json`; table cells and TOON rows put the value on a different line from its header, so shape-based redaction misses them). Register at the mint/read point.
- `--debug` output passes through redaction + control-byte stripping before emission.
- When emitting free-text debug/log lines through the redactor, do NOT start a line with a `token:`/`secret:`-style prefix (`<secretword>:`) — its assignment regex scrubs the next word to `***`. Word prose so no secret word immediately precedes a `:`/`=`.
