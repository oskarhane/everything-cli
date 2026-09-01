# linear

The `linear` provider: Linear issues, teams, and projects. Command
layout:

```sh
everything-cli linear <resource> <action>
```

> **Stub** — this reference is filled in by the provider's docs node.
> Commands below are listed at headline level only.

## Auth

Linear authenticates with a personal API key, sent raw in the
`Authorization` header (no Bearer prefix). Key capture at
`linear account add` time: `--api-key` flag, then `$LINEAR_API_KEY`,
then a hidden prompt. The key is stored at
`<config>/accounts/linear/<name>.json` (mode 0600) and registered for
redaction — it is never printed.

## account

- `linear account add <name>` — add an account with a personal API key
  (`--api-key`, else `$LINEAR_API_KEY`, else a hidden prompt).
- `linear account list` — list configured Linear accounts.
- `linear account get <name>` — account metadata (the key is never
  printed).
- `linear account use <name>` — set the default Linear account.
- `linear account remove <name>` — remove an account and its stored key.

## issue

- `linear issue list` — list issues; `--team <id>` scopes to one team.
- `linear issue get <id>` — show one issue.
- `linear issue create` — `--team` and `--title` required;
  `--description`, `--assignee`, `--state`.
- `linear issue update <id>` — `--title`, `--description`, `--assignee`,
  `--state`.

## team

- `linear team list` — list teams.

## project

- `linear project list` — list projects.
