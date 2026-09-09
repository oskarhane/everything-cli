// Package podcast wires Podcast as a provider of the CLI: the transcript
// resource tree. The provider self-registers at init time; main.go wires it
// in with a side-effect import.
package podcast

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/provider"
)

// ID is the provider's registry key and command path segment.
const ID = "podcast"

// Provider is Podcast's provider.Provider implementation.
type Provider struct{}

// Compile-time proof that Provider satisfies the provider contract.
var _ provider.Provider = Provider{}

func init() { provider.Register(Provider{}) }

// ID returns the provider identifier.
func (Provider) ID() string { return ID }

// NewCmd builds the `podcast` command tree.
func (Provider) NewCmd(cfg *app.Config) *cobra.Command { return NewCmd(cfg) }
