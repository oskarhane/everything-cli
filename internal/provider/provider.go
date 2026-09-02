// Package provider defines the contract every CLI provider (google,
// linear, granola, …) implements, plus the registry the root command uses
// to discover them. The seam exists so the root command and the account
// machinery never branch on provider specifics: a provider is a command
// tree, nothing more.
package provider

import (
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/app"
)

// Provider is the contract between a provider package (e.g.
// internal/providers/google) and the CLI core. Implementations must be
// safe to register at package init time, so constructors must not do I/O.
type Provider interface {
	// ID returns the kebab-case provider identifier ("google", "linear").
	// It doubles as the registry key and is stored on each account record,
	// so it must be stable and unique across all registered providers.
	ID() string

	// NewCmd builds the provider's command tree for attachment to the root
	// command. cfg is the root's shared config (account, format, debug,
	// filesystem); the returned command's Use should be the provider ID so
	// invocation is `<cli> <provider> <resource> <action>`.
	NewCmd(cfg *app.Config) *cobra.Command
}
