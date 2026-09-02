package granola

import (
	"github.com/oskarhane/everything-cli/internal/auth/apikey"
)

// strategy is Granola's auth strategy: a grn_ API key sent as
// "Authorization: Bearer <key>", captured from --api-key, then
// GRANOLA_API_KEY, then a hidden prompt. The key is a secret on par with an
// OAuth refresh token: apikey registers it for redaction at capture and
// read points, and no command ever prints it. The config is compile-time
// constant, so Must panics on a misconfiguration rather than returning an
// error no caller could handle.
var strategy = apikey.Must(apikey.Config{
	Provider:     providerID,
	HeaderName:   "Authorization",
	HeaderFormat: "Bearer %s",
	EnvVar:       "GRANOLA_API_KEY",
})
