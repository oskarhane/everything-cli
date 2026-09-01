# granola

The `granola` provider: read Granola notes via the Granola public API
(`public-api.granola.ai`). Command layout:

```sh
everything-cli granola <resource> <action>
```

> **Stub** — this reference is filled in by the provider's docs node.
> Commands below are listed at headline level only.

## Auth

Granola authenticates with a `grn_` API key (Business plan), sent as
`Authorization: Bearer grn_...`. Key capture at `granola account add`
time: `--api-key` flag, then `$GRANOLA_API_KEY`, then a hidden prompt.
The key is stored at `<config>/accounts/granola/<name>.json` (mode 0600)
and registered for redaction — it is never printed.

## account

- `granola account add <name>` — add an account from a `grn_` API key
  (`--api-key`, else `$GRANOLA_API_KEY`, else a hidden prompt).
- `granola account list` — list configured Granola accounts.
- `granola account get <name>` — account metadata (the key is never
  printed).
- `granola account use <name>` — set the default Granola account.
- `granola account remove <name>` — remove an account and its stored
  key.

## note

- `granola note list` — list notes; filters: `--created-after`,
  `--created-before`, `--updated-after`, `--folder-id`.
- `granola note get <id>` — one note with its AI summary;
  `--include-transcript` inlines the transcript (very long notes may
  fail with `413 TRANSCRIPT_TOO_LARGE`).
