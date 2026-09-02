# email

The `email` provider: regular email over IMAP (read) and SMTP (send).
Command layout:

```sh
everything-cli email <resource> <action>
```

Currently shipped: the `account` subtree. Mailbox/message commands are
added by later work.

## Auth (username + password)

Email accounts authenticate to their IMAP and SMTP servers with a
username (usually the email address) and a password or app password.

1. `everything-cli email account add <name> --imap-host <host>
   --smtp-host <host> --username <user>` — captures the endpoints and
   credential. Password capture order: the `--password` flag, then the
   `$EMAIL_PASSWORD` environment variable, then a hidden prompt (never
   echoed). Prefer the prompt or the env var — a literal `--password ...`
   lands in shell history. Port flags: `--imap-port` (default `993`,
   implicit TLS) and `--smtp-port` (default `587`, STARTTLS submission).
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
  (default `993`), `--smtp-port <n>` (default `587`). Prints the added
  account's `name` only — never the password.
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
EMAIL_PASSWORD=... everything-cli email account add work --imap-host imap.example.com --smtp-host smtp.example.com --username me@example.com
everything-cli email account list --format json
everything-cli email account get work --format json
everything-cli email account use work
everything-cli email account remove old --force
```

## Tips & gotchas (email)

- TLS only: IMAP uses implicit TLS on port 993; SMTP uses STARTTLS
  submission on port 587 (or implicit TLS on 465). There is no plaintext
  fallback.
- The account password is a secret: it is registered for redaction
  before anything could print it and never appears in output, including
  `--debug`.
