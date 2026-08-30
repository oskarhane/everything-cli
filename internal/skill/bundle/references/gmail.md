# gmail

Manage Gmail labels, messages, drafts, threads, and attachments.

## label

- `gmail label list` — list labels.
- `gmail label get <id-or-name>` — show one label by id or name.
- `gmail label create <name>` — flags: `--color-text #rrggbb`,
  `--color-bg #rrggbb`, `--label-list-visibility labelShow|labelHide`,
  `--message-list-visibility show|hide`.
- `gmail label update <id>` — `--name`, plus the create flags.
- `gmail label delete <id> [--force]` — refuses without `--force`.

```sh
google-cli gmail label create Travel --color-text "#ffffff" --color-bg "#039be5" \
  --label-list-visibility labelHide
google-cli gmail label delete Label_42 --force
```

## message

- `gmail message list` — flags: `--query/-q` (Gmail search, e.g.
  `from:boss@corp.com subject:invoice`), `--label-ids` (comma-separated),
  `--unread-only`, `--max` (default 25).
- `gmail message get <id>` — headers + body as JSON/table; `--raw` prints the
  decoded RFC 2822 message (control bytes stripped).
- `gmail message send` — `--to` (required, comma-separated), `--cc`, `--bcc`,
  `--subject`, `--body | --body-file` (mutually exclusive), `--attachment
<path>` (repeatable).
- `gmail message modify <id>` — `--add-label-ids`, `--remove-label-ids`.
- `gmail message mark <id>` — `--read`, `--unread`, `--starred`,
  `--unstarred`.
- `gmail message trash <id>` / `gmail message untrash <id>` — recoverable.
- `gmail message delete <id> [--force]` — permanent; refuses without `--force`.
  Prefer `trash`.

```sh
google-cli gmail message list --query "from:boss@corp.com subject:invoice" \
  --unread-only --max 10 --format json
google-cli gmail message send --to a@example.com,b@example.com --subject "Report" \
  --body-file report.txt --attachment q1.pdf --attachment data.csv
google-cli gmail message modify 19c2a4b7 --add-label-ids Label_7
google-cli gmail message trash 19c2a4b7 --account work
```

## thread

- `gmail thread list` — `--query/-q`, `--label-ids`, `--max` (default 25).
- `gmail thread get <id>` — a thread's messages with headers.

```sh
google-cli gmail thread list --query "subject:invoice" --label-ids Label_7 --max 10
```

## draft

- `gmail draft list` — `--max` (default 25).
- `gmail draft get <id>` — stored message headers.
- `gmail draft create` — `--to` (required), `--subject`, `--body | --body-file`.
- `gmail draft send <id>` — sends and echoes the sent message.
- `gmail draft delete <id> [--force]` — permanent; refuses without `--force`.

```sh
google-cli gmail draft create --to alice@example.com --subject "Lunch" --body "Noon works"
google-cli gmail draft send draft_19c2a4b7 --format json
```

## attachment

- `gmail attachment get <attachment-id>` — `--message-id <id>` (required),
  `--out <file>` (else decoded bytes to stdout for piping).

```sh
google-cli gmail attachment get ANG1xQ8q --message-id 19c2a4b7 --out report.pdf
```
