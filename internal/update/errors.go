package update

import "errors"

// User-facing sentinel errors for the self-update flow.
var (
	// ErrNoReleases is returned when the repo has no published releases.
	ErrNoReleases = errors.New("no releases published yet")
	// ErrRateLimited is returned when the GitHub API rejects the request
	// for rate-limit reasons (HTTP 403).
	ErrRateLimited = errors.New("rate limited by GitHub, retry later")
	// ErrAssetNotFound is returned when a release has no asset with the
	// requested name.
	ErrAssetNotFound = errors.New("release asset not found")
)
