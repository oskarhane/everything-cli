# drive

Generic Drive file operations — list, get, create, upload, download,
trash, delete, and sharing. Create Google Docs/Sheets/Slides here; their
content-level verbs live in the docs/sheets/slides references.

## file list

- `drive file list` — flags: `--query/-q` (raw Drive q term passed through
  verbatim, e.g. `owner = 'me'`), `--name` (substring match), `--parent`
  (folder id), `--mime` (`folder`, `doc`, `sheet`, `slide`, or a raw MIME
  type), `--trashed` (include trashed; default lists only non-trashed),
  `--max` (default 25, `0` = uncapped). All terms are ANDed.

```sh
google-cli drive file list --format json
google-cli drive file list --parent 1AbC --mime folder --format table
google-cli drive file list --name "invoice" --query "owner = 'me'" --max 100
```

`--mime` shorthands (any other value passes through raw): `folder` →
`application/vnd.google-apps.folder`, `doc` → `...document`, `sheet` →
`...spreadsheet`, `slide` → `...presentation`. A full MIME string works too.

## file get

- `drive file get <file-id>` — metadata only (name, mime_type, size, owner,
  parent_ids, trashed, shared, modified_time, web_link, description). Use
  `download` for content.

```sh
google-cli drive file get 1AbCdEfGh --format json
```

## file create

- `drive file create <name>` — flags: `--type folder|doc|sheet|slide`
  (default `folder`), `--mime-type` (raw MIME; mutually exclusive with
  `--type`), `--parent`, `--description`. This is how Docs/Sheets/Slides
  files are created — there is no separate `docs create`/`sheets create`/
  `slides create`.

```sh
google-cli drive file create "Reports"
google-cli drive file create "Q3 budget" --type sheet --parent 1AbCdEfGh
google-cli drive file create "notes.md" --mime-type text/markdown
```

## file upload

- `drive file upload <local-path>` — flags: `--name` (default: the local
  path's base name), `--parent`, `--mime-type` (default from the extension,
  else `application/octet-stream`).

```sh
google-cli drive file upload ./report.pdf --name "Q3 report" --parent 1AbCdEfGh
```

## file download

- `drive file download <file-id>` — `--out <file>` (else bytes to stdout for
  piping), `--export <mime>`. Binary files stream via alt=media. Google-native
  types have no downloadable binary and must be exported. Export defaults
  when `--export` is unset: doc → `text/plain`, sheet → `text/csv`,
  slide → `text/plain`; other native types refuse until `--export` names a
  supported export MIME.
- Export caveat: sheet exports (`text/csv`, `text/tab-separated-values`)
  cover the FIRST SHEET ONLY, and Drive caps exports at 10 MB (larger
  exports are truncated with a warning marker). Other supported exports:
  docs: text/plain, text/markdown, application/pdf, application/rtf,
  application/epub+zip, application/zip, docx, odt; sheets: text/csv,
  text/tab-separated-values, application/pdf, application/zip, xlsx, ods;
  slides: text/plain, application/pdf, pptx, odp; drawings: image/png,
  image/jpeg, image/svg+xml, application/pdf.

```sh
google-cli drive file download 1AbCdEfGh --out report.pdf
google-cli drive file download 1AbCdEfGh --export text/csv > data.csv
```

## file trash / untrash

- `drive file trash <file-id>` / `drive file untrash <file-id>` — move to
  Drive's trash and restore. Recoverable; prefer over delete.

```sh
google-cli drive file trash 1AbCdEfGh
google-cli drive file untrash 1AbCdEfGh --account work
```

## file delete

- `drive file delete <file-id> [--force]` — permanent, bypasses the trash;
  refuses without `--force`. Prefer `drive file trash`.

```sh
google-cli drive file delete 1AbCdEfGh --force
```

## file permissions

- `drive file permissions <file-id>` — list every permission (id, type,
  role, email_address, display_name, deleted). Run it to find the id
  `unshare` needs.

```sh
google-cli drive file permissions 1AbCdEfGh --format json
```

## file share

- `drive file share <file-id>` — `--role reader|commenter|writer` (required)
  plus exactly one of `--email <addr>` | `--anyone` | `--domain <domain>`.
  `--expires <rfc3339>` sets an expiry, which the Drive API honors only on
  user and group permissions (and allows at most one year out).

```sh
google-cli drive file share 1AbCdEfGh --role reader --email alice@example.com
google-cli drive file share 1AbCdEfGh --role commenter --anyone
google-cli drive file share 1AbCdEfGh --role writer --domain example.com \
  --expires 2027-09-01T00:00:00Z
```

## file unshare

- `drive file unshare <file-id>` — exactly one of `--permission <id>` (see
  `drive file permissions`) or `--email <addr>` (resolved via the
  permissions list, case-insensitively; zero or multiple matches refuse and
  ask for `--permission`).

```sh
google-cli drive file unshare 1AbCdEfGh --email alice@example.com
google-cli drive file unshare 1AbCdEfGh --permission 8zK
```
