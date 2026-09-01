package granola

import (
	"fmt"

	"github.com/oskarhane/google-cli/internal/auth/apikey"
)

// strategy is Granola's auth strategy: a grn_ API key sent as
// "Authorization: Bearer <key>", captured from --api-key, then
// GRANOLA_API_KEY, then a hidden prompt. The key is a secret on par with an
// OAuth refresh token: apikey registers it for redaction at capture and
// read points, and no command ever prints it.
var strategy = mustStrategy()

// mustStrategy builds the API-key strategy, panicking on a misconfiguration.
// The config is compile-time constant, so an error is a programmer error
// that must fail loudly at startup, like a duplicate registry ID.
func mustStrategy() *apikey.Strategy {
	s, err := apikey.New(apikey.Config{
		Provider:     providerID,
		HeaderName:   "Authorization",
		HeaderFormat: "Bearer %s",
		EnvVar:       "GRANOLA_API_KEY",
	})
	if err != nil {
		panic(fmt.Sprintf("granola: building API key strategy: %v", err))
	}
	return s
}
