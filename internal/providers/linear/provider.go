// Package linear wires Linear as a provider of the CLI: an API-key auth
// strategy plus the issue, team, project, and account command trees. The
// provider self-registers at init time; main.go wires it in with a
// side-effect import.
package linear

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/app"
	"github.com/oskarhane/google-cli/internal/auth"
	"github.com/oskarhane/google-cli/internal/auth/apikey"
	"github.com/oskarhane/google-cli/internal/provider"
)

// ID is the provider's registry key and command path segment.
const ID = "linear"

// envVarAPIKey is the environment variable consulted for a Linear personal
// API key when --api-key is not given.
const envVarAPIKey = "LINEAR_API_KEY"

// Provider is Linear's provider.Provider implementation.
type Provider struct{}

// Compile-time proof that Provider satisfies the provider contract.
var _ provider.Provider = Provider{}

func init() { provider.Register(Provider{}) }

// ID returns the provider identifier.
func (Provider) ID() string { return ID }

// Auth returns Linear's API-key auth strategy.
func (Provider) Auth() auth.Strategy { return newStrategy() }

// NewCmd builds the `linear` command tree.
func (Provider) NewCmd(cfg *app.Config) *cobra.Command { return newLinearCmd(cfg) }

// newStrategy builds the Linear API-key strategy: the raw key in the
// Authorization header (no Bearer prefix), captured from --api-key, then
// LINEAR_API_KEY, then a hidden prompt. The strategy registers the key for
// redaction at capture/read, so it never reaches output.
func newStrategy() *apikey.Strategy {
	s, err := apikey.New(apikey.Config{
		Provider:     ID,
		HeaderName:   "Authorization",
		HeaderFormat: "%s",
		EnvVar:       envVarAPIKey,
	})
	if err != nil {
		// The config is static; a failure here is a programmer error.
		panic(err)
	}
	return s
}
