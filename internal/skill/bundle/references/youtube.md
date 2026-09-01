# youtube

YouTube video metadata and timed transcripts — no Google account, no OAuth,
and no API key of your own. `youtube transcript` prints a video's captions
and `youtube metadata` reports its player metadata. Both take a single
`<url-or-id>` argument in any of these forms:

- a watch URL: `https://www.youtube.com/watch?v=<id>` (also
  `m.youtube.com`/`music.youtube.com` subdomains)
- a `youtu.be` short link: `https://youtu.be/<id>`
- a `/shorts/`, `/embed/`, or `/live/` link: `https://www.youtube.com/live/<id>`
- a bare 11-character video ID: `dQw4w9WgXcQ`

Anything else fails with `invalid YouTube video ID`. The global `--account`
and `--credentials` flags are irrelevant here — these commands never touch
the token cache.

## youtube transcript

- `youtube transcript <url-or-id>` — print the video's timed captions.
  Flags:
  - `--lang <code>` — caption track language, ISO 639-1 (default `en`).
    Preference: a human-written track in that language, then an
    auto-generated (ASR) track in that language, then the first available
    track regardless of language.
  - `--raw` — print the caption text as plain lines even on an interactive
    terminal.
  - `--out <file>` — write the plain caption text to a file instead of
    stdout (beats everything, like `docs get --out`).
- Rendering: piped/non-TTY stdout streams plain text (one caption line per
  segment); an interactive terminal shows the structured report as a table;
  an explicit `--format json|table|toon` always renders the structured
  report. Fields: `video_id`, `title`, `lang`, `is_generated`, `segments`
  — each segment has `start_ms`, `duration_ms`, and `text` (JSON/TOON carry
  the full timed array; the table cell shows a compact
  "N segments · total duration" summary).

```sh
google-cli youtube transcript https://www.youtube.com/watch?v=dQw4w9WgXcQ
google-cli youtube transcript https://youtu.be/dQw4w9WgXcQ --format json
google-cli youtube transcript dQw4w9WgXcQ --raw              # plain text on a TTY
google-cli youtube transcript dQw4w9WgXcQ --lang de --out de.txt
google-cli youtube transcript "https://www.youtube.com/shorts/dQw4w9WgXcQ" | head -20
```

## youtube metadata

- `youtube metadata <url-or-id>` — the video's player metadata plus every
  caption track it offers. Structured `--format` output (table on a TTY).
  Fields: `video_id`, `title`, `channel`, `channel_id`, `duration_seconds`,
  `view_count`, `publish_date`, `upload_date`, `category`, `description`,
  `available_langs`.

`available_langs` lists one entry per caption track language — human and ASR
duplicates included — so it is the way to discover `transcript --lang`
choices before fetching a track.

```sh
google-cli youtube metadata "https://www.youtube.com/watch?v=dQw4w9WgXcQ" --format json
google-cli youtube metadata dQw4w9WgXcQ --format table
google-cli youtube metadata "https://youtu.be/dQw4w9WgXcQ" --format toon
```

## Unofficial endpoint

Both commands fetch from YouTube's undocumented InnerTube player endpoint
(the Android client context the official mobile app uses) — not the Google
Data API. There is nothing to provision (no OAuth, no API key), but the
endpoint can change or break when YouTube does. Failures are surfaced as
errors, never as silent empty output: do not treat an absent transcript as
an empty success.

## Errors

- `invalid YouTube video ID` — the argument is not a recognized URL shape or
  a valid 11-character ID.
- `no caption tracks available` — the video offers no captions at all (not
  even auto-generated); the error names the video, e.g.
  `video dQw4w9WgXcQ: no caption tracks available`.
- `empty transcript` — the caption track answered HTTP 200 but carried no
  parseable content (YouTube's PoToken gate answers the timedtext URL with
  an empty body). Reported as an error, never as an empty transcript.
- `video is not playable: <reason>` — the player endpoint reports a
  non-OK playability status (age-restricted, private, member-only, etc.);
  the wrapped reason is YouTube's own wording.
- Non-200 endpoint statuses and oversized response bodies surface as plain
  errors as well.
