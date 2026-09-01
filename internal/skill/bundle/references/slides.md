# slides

Google Slides text operations. Create presentations with
`drive file create <name> --type slide`; `slides delete` is a thin Drive
delete — `drive file trash <presentation-id>` is the recoverable
alternative.

## slides get

- `slides get <presentation-id>` — one row per text-bearing shape, in slide
  order (fields: slide, shape_id, text). `--slide <n>` narrows to one
  slide's shapes (1-based, matching the slide column; 0 = all slides).

```sh
google-cli slides get 1AbCpresentationID --format json
google-cli slides get 1AbCpresentationID --slide 3
```

## slides replace

- `slides replace <presentation-id>` — replace every occurrence of `--find`
  (required) with `--replace-with` across every slide, in one API call.
  `--match-case` makes matching case-sensitive (default is
  case-insensitive). Prints the replaced occurrence count.

```sh
google-cli slides replace 1AbCpresentationID --find Acme --replace-with Zenith
google-cli slides replace 1AbCpresentationID --find KPI --replace-with OKR --match-case
```

## slides delete

- `slides delete <presentation-id> [--force]` — permanently delete the
  presentation's Drive file; refuses without `--force`. Prefer
  `drive file trash <presentation-id>`.

```sh
google-cli slides delete 1AbCpresentationID --force
```
