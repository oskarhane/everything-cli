# docs

Google Docs content, at the text level. Create documents with
`drive file create <name> --type doc`. Docs are Drive files: `docs delete`
is a thin Drive delete, and `drive file trash <doc-id>` is the recoverable
alternative to `docs delete`.

## docs get

- `docs get <doc-id>` — the document's raw text, streamed to stdout exactly
  as exported (bypasses `--format`); `--out <file>` writes it there instead.

```sh
google-cli docs get 1AbCdEfGh --out notes.txt
google-cli docs get 1AbCdEfGh | head -20
```

## docs append

- `docs append <doc-id>` — add text at the very end of the body. Flags:
  `--text` | `--text-file <path>` (exactly one). A trailing newline is
  added when missing, so successive appends each start on their own line.

```sh
google-cli docs append 1AbCdEfGh --text "Reviewed by Oskar"
google-cli docs append 1AbCdEfGh --text-file notes.txt
```

## docs insert

- `docs insert <doc-id>` — insert text immediately BEFORE the given
  `--index` (required, > 0 — a zero-based Docs-API content index in UTF-16
  code units; `--index 1` puts the text at the very start of the body).
  Flags: `--text` | `--text-file` (exactly one). Unlike append, the text is
  sent verbatim — no newline is added.

```sh
google-cli docs insert 1AbCdEfGh --index 1 --text "Q4 plan"
google-cli docs insert 1AbCdEfGh --text-file block.txt --index 120
```

## docs replace

- `docs replace <doc-id>` — replace every occurrence of `--find` (required)
  with `--replace-with` (empty deletes the matches). `--match-case` makes
  matching case-sensitive (default is case-insensitive). Prints the
  replaced occurrence count.

```sh
google-cli docs replace 1AbCdEfGh --find "Project Falcon" --replace-with "Project Falcon 2"
google-cli docs replace 1AbCdEfGh --find TODO --replace-with "TBD"
```

## docs delete

- `docs delete <doc-id> [--force]` — permanently delete the document's
  Drive file; refuses without `--force`. Prefer `drive file trash <doc-id>`.

```sh
google-cli docs delete 1AbCdEfGh --force
```
