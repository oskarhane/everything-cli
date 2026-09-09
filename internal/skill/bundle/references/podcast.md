# podcast

The `podcast` provider fetches podcast episode transcripts — no account, no
OAuth, no API key, and no token cache. It has one leaf command:

```sh
everything-cli podcast transcript <apple-or-spotify-episode-url> [--raw] [--out file] [--format json|table|toon]
```

Registered provider-first like every provider: the `transcript` leaf lives
under the `podcast` provider and is invoked as `everything-cli podcast
transcript` — never as a bare `everything-cli transcript`.

The global `--account` flag and any provider `--credentials` flag are
irrelevant here — this provider never touches the token cache.

## podcast transcript

`podcast transcript <url>` prints one episode's timed transcript. The
`<url>` argument must be an Apple Podcasts or Spotify episode page:

- **Apple**: `https://podcasts.apple.com/<storefront>/podcast/<slug>/id<podcastId>?i=<episodeId>` —
  the `?i=` query is **required**; it carries the episode id.
- **Spotify**: `https://open.spotify.com/episode/<id>` or
  `https://open.spotify.com/intl-<xx>/episode/<id>` (an optional
  `/intl-<2 letters>/` segment).

Flags:

- `--raw` — print the transcript text as plain lines even on an interactive
  terminal.
- `--out <file>` — write the plain transcript text to a file instead of
  stdout (beats everything, like `google docs get --out`).
- The global `--format json|table|toon` renders the structured report.

Render priority (mirrors `google youtube transcript`): `--out` always means
"plain text to this file instead of stdout", so it beats everything; then an
explicit `--format` renders the structured report; then `--raw` — and any
piped/non-TTY stdout — streams plain text (one line per segment); only an
interactive terminal with no explicit format gets the auto-detected report.
Plain-text lines are control-byte sanitized (`output.StripControl`) before
writing — transcript text is creator-controlled, so C0 bytes (ANSI/OSC
terminal escapes) are neutralized.

Structured report fields (snake_case): `show`, `episode`, `platform`,
`episode_id`, `segments` — each segment has `start_ms`, `duration_ms`, and
`text` (JSON/TOON carry the full timed array; the table cell shows a compact
"N segments · total duration" summary).

```sh
everything-cli podcast transcript "https://podcasts.apple.com/us/podcast/slug/id123?i=456"
everything-cli podcast transcript "https://open.spotify.com/episode/abc" --format json
everything-cli podcast transcript "https://podcasts.apple.com/us/podcast/slug/id123?i=456" --raw   # plain text on a TTY
everything-cli podcast transcript "https://open.spotify.com/intl-se/episode/abc" --out notes.txt   # plain text to a file
everything-cli podcast transcript "https://open.spotify.com/episode/abc" | head -20                 # piped plain text
```

## Source & coverage

Transcripts come from the show's creator-published RSS `podcast:transcript`
tag (WebVTT or SRT) — the same tags podcast players surface. Coverage is
**limited to feeds that carry that tag** (~6.3% of feeds globally per
podcast-standard.org, skewed toward professionally hosted shows): a show
that does not publish transcripts errors instead of returning empty.

Resolution is fully unauthenticated — no tokens, no OAuth, no API key, no
token cache. Apple episode pages are resolved via `og:title` plus an iTunes
lookup by podcast id; Spotify via the embed page's `__NEXT_DATA__` titles
plus an iTunes search. Failures surface as errors, never as silent empty
output.

### Errors

- `podcast: unsupported episode URL` — the argument is not a recognized
  Apple or Spotify episode URL shape (e.g. a missing `?i=` on an Apple link,
  or a URL targeting something other than an episode).
- `podcast: show not found` — couldn't resolve the show's metadata / RSS
  feed from the URL: an Apple iTunes lookup miss, a dead Spotify episode, or
  an iTunes search miss.
- `podcast: episode not found in feed` — the show's feed was fetched but no
  `<item>` title matched the episode.
- `podcast: feed carries no usable transcript` — the show's feed has no
  VTT/SRT `podcast:transcript` tag for this episode.
- `podcast: empty transcript` — the transcript URL answered HTTP 200 but
  carried no parseable cues. Reported as an error, never as an empty
  transcript.
