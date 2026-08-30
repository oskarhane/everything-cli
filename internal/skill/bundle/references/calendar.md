# calendar

Manage calendars, their sharing rules, events, and free/busy.

Timestamp flags (`--start`, `--end`, `--from`, `--to`) accept RFC3339
(`2026-09-03T14:00:00Z`), a date (`2026-09-03`, needs `--all-day` for event
create/update), or relative offsets anchored at now (`now`, `-1d`, `+7d`,
`-30m`, `+2h`, bare `7d` counts forward).

## calendar (CRUD)

- `calendar list` — every listed calendar.
- `calendar create <summary>` — `--timezone`, `--description`, `--color-id`.
- `calendar get <calendar-id>` — show one calendar by id.
- `calendar update <calendar-id>` — `--summary`, `--timezone`,
  `--description`, `--color-id`.
- `calendar delete <calendar-id> [--force]` — destructive; refuses without
  `--force`.

```sh
google-cli calendar create "Team PTO" --timezone Europe/Stockholm --color-id tomato
google-cli calendar delete abc123.group.calendar.google.com --force
```

## event

- `calendar event list` — `--calendar` (default `primary`), `--from` (default
  `now`), `--to` (default `+7d`), `--query`, `--max` (default 250; `0` = no
  cap), `--recurring instances|masters|all` (default `instances` = recurring
  series expanded into occurrences; `masters` = raw masters plus one-offs and
  exceptions; `all` = both merged and deduped).
- `calendar event get <event-id>` — `--calendar`.
- `calendar event create` — `--summary` (required), `--start`, `--end`
  (required; RFC3339, or YYYY-MM-DD with `--all-day`), `--calendar`,
  `--all-day`, `--timezone` (required for recurring series), `--location`,
  `--description`, `--attendee` (repeatable), `--reminder-minutes`,
  `--color-id`, `--recurrence` (repeatable raw `RRULE:`/`RDATE:`/`EXDATE:`).
- `calendar event update <event-id>` — `--summary`, `--start`, `--end`,
  `--location`, `--description`, `--add-attendee`/`--remove-attendee`
  (repeatable), `--calendar`, `--this-only` (default true; false with an
  instance id patches its master, i.e. the whole series).
- `calendar event delete <event-id> [--force]` — `--calendar`, `--this-only`
  (default true; false with an instance id deletes its master). Refuses
  without `--force`.
- `calendar event instances <master-id>` — expand a recurring series;
  `--calendar`, `--from`/`--to` (empty = unbounded), `--max` (default 250).
- `calendar event move <event-id>` — `--to-calendar` (required),
  `--calendar` (source).
- `calendar event accept <event-id>`, `calendar event decline <event-id>`,
  `calendar event tentative <event-id>` — respond to an invitation.
  `--calendar`, `--this-only` (default true), `--all` (respond
  for the whole series; overrides `--this-only`). An instance id (ends in
  `_UTC-time`) responds for that occurrence only; `--all` or a master id
  responds for the series.

```sh
google-cli calendar event create --summary "Design review" \
  --start 2026-09-03T14:00:00Z --end 2026-09-03T15:00:00Z
google-cli calendar event create --summary "Standup" --start 2026-09-01T09:00:00+02:00 \
  --end 2026-09-01T09:30:00+02:00 --attendee colleague@example.com \
  --recurrence 'RRULE:FREQ=WEEKLY;COUNT=10' --format json
google-cli calendar event list --from 2026-09-01T00:00:00Z --to 2026-09-08T00:00:00Z --format json
google-cli calendar event list --calendar work@example.com --query "design review" --max 10
google-cli calendar event update abc123 --summary "Design review"
google-cli calendar event delete kq3abc123_20260929T030000Z --force
google-cli calendar event accept kq3abc123_20260929T030000Z --all
google-cli calendar event instances kq3abc123 --from now --to +14d
google-cli calendar event move abc123 --to-calendar work.group.calendar.google.com
```

## acl (sharing rules)

- `calendar acl list <calendar-id>` — list sharing rules.
- `calendar acl add <calendar-id>` — `--scope-user <email>` (required),
  `--role reader|writer`.
- `calendar acl remove <calendar-id>` — `--rule-id <id>` (e.g.
  `user:colleague@example.com`).

```sh
google-cli calendar acl add primary --scope-user colleague@example.com --role reader
google-cli calendar acl remove primary --rule-id user:colleague@example.com
```

## freebusy

- `calendar freebusy` — `--calendar <id>` (repeatable; default = every listed
  calendar), `--from` (default `now`), `--to` (default `+1d`).

```sh
google-cli calendar freebusy --from 2026-09-01T09:00:00Z --to 2026-09-01T17:00:00Z \
  --calendar work@example.com --calendar personal@example.com --format table
```
