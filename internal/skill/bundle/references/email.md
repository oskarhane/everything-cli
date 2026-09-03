# email

The `email` provider: regular email over IMAP (read) and SMTP (send).
Command layout:

```sh
everything-cli email <resource> <action>
```

## Auth (username + password)

Email accounts authenticate to their IMAP and SMTP servers with a
username (usually the email address) and a password or app password.

1. `everything-cli email account add <name> --imap-host <host>
   --smtp-host <host> --username <user>` — captures the endpoints and
   credential. Password capture order: the `--password` flag, then the
   `$EMAIL_PASSWORD` environment variable, then a hidden prompt (never
   echoed). Prefer the prompt or the env var — a literal `--password ...`
   lands in shell history. Host flags accept a bare host or a `host:port`
   value (IPv6 literals as `[::1]:1143` or bare `::1`). Port precedence:
   an explicit `--imap-port`/`--smtp-port` flag wins, then a port embedded
   in the host value, then the defaults (`993` implicit TLS for IMAP,
   `587` STARTTLS submission for SMTP). The stored payload always keeps a
   pure host (no port) plus the resolved integer port.
2. `everything-cli email account use <name>` — set the default email
   account. Every command also accepts the global `--account <name>`
   override.

The credential is stored at `<config>/accounts/email/<name>.json` (mode
0600) as `{"username": ..., "password": ..., "imap": {"host": ...,
"port": ...}, "smtp": {"host": ..., "port": ...}}` and the password is
registered for redaction at capture and read time: it is never printed,
in any output format, including `--debug`. `email account get` and
`email account list` show metadata only. Treat the password as a secret
on par with an OAuth refresh token.

## account

Manage email accounts and their stored IMAP/SMTP credentials.

- `email account add <name>` — add an account from its server endpoints
  and credentials. Flags: `--imap-host <host>` (required), `--smtp-host
  <host>` (required), `--username <user>` (required), `--password <pw>`
  (empty = `$EMAIL_PASSWORD`, then a hidden prompt), `--imap-port <n>`
  (default `993`), `--smtp-port <n>` (default `587`). The host flags
  accept `host:port` too (`--imap-host 127.0.0.1:1143`, `[::1]:1143`); an
  explicit port flag overrides an embedded port, an embedded port
  overrides the default. Prints the added account's `name` only — never
  the password.
- `email account list` — list configured email accounts. Fields: `name`,
  `default` (the default account carries `default: true` in JSON/TOON,
  `(default)` in table output).
- `email account get <name>` — account metadata. Fields: `name`,
  `provider`. The password is never printed.
- `email account use <name>` — make `<name>` the default email account.
- `email account remove <name> --force` — remove an account and its
  stored password. Refuses without `--force`. Removing the default
  promotes another email account and announces the new default.

```sh
everything-cli email account add work --imap-host imap.example.com --smtp-host smtp.example.com --username me@example.com
everything-cli email account add dev --imap-host 127.0.0.1:1143 --smtp-host 127.0.0.1:1025 --username dev@example.com
EMAIL_PASSWORD=... everything-cli email account add work --imap-host imap.example.com --smtp-host smtp.example.com --username me@example.com
everything-cli email account list --format json
everything-cli email account get work --format json
everything-cli email account use work
everything-cli email account remove old --force
```

## mailbox

List the account's IMAP mailboxes (folders).

### mailbox list

- `email mailbox list` — every mailbox on the acting account, in the
  server's sorted order. No flags of its own. Output field: `name` (one
  row per mailbox).

```sh
everything-cli email mailbox list
everything-cli email mailbox list --format json
```

## message

List, read, and send messages inside a mailbox. IMAP UIDs are
per-mailbox, so the read commands take `--mailbox <name>` (default
`INBOX`) to scope the lookup.

### message list

- `email message list` — message envelopes (headers only, never bodies)
  from one mailbox, newest first. Flags: `--mailbox <name>` (default
  `INBOX`), `--limit <n>` (default `25`; `<= 0` means all). Output
  fields: `uid`, `date` (RFC3339, UTC), `from`, `subject`, `flags`
  (array in JSON/TOON; comma-joined in the table cell).

```sh
everything-cli email message list
everything-cli email message list --mailbox Archive --limit 10 --format json
```

### message get

- `email message get <uid>` — one fully fetched message by IMAP UID.
  Flag: `--mailbox <name>` (default `INBOX`). JSON/TOON output: `uid`,
  `from`, `to`, `subject`, `date`, `body_text` (decoded plain-text
  body), `attachments` (array of `{filename, content_type, size}` —
  metadata only from BODYSTRUCTURE, `size` the server-declared encoded
  octet count; attachment bytes are never fetched). Table output is a
  header row (`uid`, `from`, `to`, `subject`, `date`) with the body
  printed as plain text below it, control bytes stripped.

```sh
everything-cli email message get 7 --format json
everything-cli email message get 42 --mailbox Archive
```

### message send

- `email message send` — submit one plain-text message via the account's
  SMTP server. Flags: `--to <addr>` (repeatable, at least one required),
  `--cc <addr>` (repeatable), `--subject <s>` (required), and exactly
  one of `--body <text>` / `--body-file <path>` (`-` reads stdin).
  Addresses accept plain `a@b` or `Name <a@b>` forms. Prints `{sent:
  true, to: [...]}` — the subject, body, and credentials are never
  echoed.

```sh
everything-cli email message send --to alice@example.com --subject "Lunch" --body "Noon works"
everything-cli email message send --to a@example.com --to b@example.com --cc carol@example.com --subject "Report" --body-file report.txt
printf 'hi' | everything-cli email message send --to alice@example.com --subject "Hi" --body-file -
```

## Tips & gotchas (email)

- TLS only: IMAP uses implicit TLS on port 993; SMTP uses STARTTLS
  submission on port 587 (or implicit TLS on 465). There is no plaintext
  fallback.
- IMAP UIDs are per-mailbox: a `uid` from `message list --mailbox
  Archive` must be fetched with `message get <uid> --mailbox Archive`.
- `message list` caps at `--limit 25` by default — pass `--limit 0` for
  the whole mailbox.
- The account password is a secret: it is registered for redaction
  before anything could print it and never appears in output, including
  `--debug`.
