// Package linear wires Linear as a provider of the CLI: a composite auth
// strategy (personal API key by default, OAuth via --oauth) plus the
// issue, team, project, and account command trees. The provider
// self-registers at init time; main.go wires it in with a side-effect
// import.
package linear

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/provider"
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

// NewCmd builds the `linear` command tree.
func (Provider) NewCmd(cfg *app.Config) *cobra.Command { return newLinearCmd(cfg) }
