# sheets

Google Sheets metadata and cell values. Create spreadsheets with
`drive file create <name> --type sheet`; `sheets delete` is a thin Drive
delete — `drive file trash <spreadsheet-id>` is the recoverable alternative.

## sheets get

- `sheets get <spreadsheet-id>` — one row per sheet tab: sheet_id, title,
  index, row_count, col_count, and a best-effort header (the tab's first
  row; empty when unreadable).

```sh
google-cli sheets get 1AbCdEfGh --format json
google-cli sheets get 1AbCdEfGh --format json | jq '.sheets[] | select(.title=="Budget")'
```

## sheets values get

- `sheets values get <spreadsheet-id>` — read an A1 range; `--range` is
  required (e.g. `Sheet1!A1:D10`). One row per spreadsheet row (`row` +
  tab-joined `values` in table output; `{"range", "values"}` object in
  JSON/TOON).

```sh
google-cli sheets values get 1AbCdEfGh --range "Sheet1!A1:D10" --format json
google-cli sheets values get 1AbCdEfGh --range "Budget!A1:C20" --format table
```

## sheets values append / update

- `sheets values append <spreadsheet-id>` — add rows after the last row of
  the table containing the range. Flags: `--range` (required, locates the
  table, e.g. `Sheet1!A1:D`), `--values` | `--values-file` (exactly one),
  `--input-option RAW|USER_ENTERED` (default `USER_ENTERED`).
- `sheets values update <spreadsheet-id>` — write rows starting at the
  top-left of `--range` (required), overwriting what is there. Same
  `--values`/`--values-file`/`--input-option` flags.
- Value input: `--values` takes an inline JSON array of arrays,
  e.g. `'[[1,"a"],[2,"b"]]'`; `--values-file` takes a `.json`, `.csv`, or
  `.tsv` file (by extension). `--input-option RAW` writes values as literal
  strings (e.g. no formula parsing); `USER_ENTERED` parses them.

```sh
google-cli sheets values append 1AbCdEfGh --range "Sheet1!A1:D" \
  --values '[[1,"a",true],[2,"b",false]]' --format json
google-cli sheets values update 1AbCdEfGh --range "Sheet1!A1:B2" \
  --values-file ./cells.tsv
google-cli sheets values update 1AbCdEfGh --range "Sheet1!C1" \
  --values '[[=SUM(A1:A2)]]' --input-option RAW
```

## sheets values clear

- `sheets values clear <spreadsheet-id>` — empty every cell in `--range`
  (required; formatting kept). No `--force`: it is bounded to the range and
  recoverable via revision history.

```sh
google-cli sheets values clear 1AbCdEfGh --range "Sheet1!A2:D10"
```

## sheets delete

- `sheets delete <spreadsheet-id> [--force]` — permanently delete the
  spreadsheet's Drive file; refuses without `--force`. Prefer
  `drive file trash <spreadsheet-id>`.

```sh
google-cli sheets delete 1AbCdEfGh --force
```
