// Package app builds the google-cli command tree.
package app

import (
	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/output"
)

// Config holds the values of the root command's persistent flags.
// It is constructed once in main and passed to subcommand constructors.
type Config struct {
	// Account is the Google account name to act as. Empty means the default account.
	Account string

	// Format is the output format: "json", "table", or "toon". Empty means auto-detect.
	Format string

	// Debug enables debug output.
	Debug bool

	// Credentials is the path to an OAuth app credentials JSON file. Empty means auto-resolve.
	Credentials string

	// Fs abstracts filesystem access for subcommands.
	Fs afero.Fs
}

// NewConfig returns the default config for the root command.
func NewConfig() *Config {
	return &Config{
		Fs: afero.NewOsFs(),
	}
}

// NewRootCommand builds the google-cli root command with persistent flags bound to cfg.
// Subcommands are attached by callers and receive cfg.
func NewRootCommand(cfg *Config) *cobra.Command {
	root := &cobra.Command{
		Use:   "google-cli",
		Short: "Interact with Google services from the command line",
		// Run prints help so the root's flags and usage stay visible
		// until subcommands are attached.
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	f := root.PersistentFlags()
	f.StringVar(&cfg.Account, "account", "", "Google account to act as (empty = default account)")
	f.StringVar(&cfg.Format, "format", "", "Output format: json, table, or toon (empty = auto-detect)")
	f.BoolVar(&cfg.Debug, "debug", false, "Enable debug output")
	f.StringVar(&cfg.Credentials, "credentials", "", "Path to OAuth app credentials JSON (empty = auto-resolve)")

	// The single place cfg.Debug is consumed: it gates output.Debug lines
	// (stderr, control-stripped). Subcommands that define their own
	// PersistentPreRun must call this one first if they override it.
	root.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		output.SetDebug(cfg.Debug)
		return nil
	}

	return root
}
