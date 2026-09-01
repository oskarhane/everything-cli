// Package app builds the google-cli command tree.
package app

import (
	"fmt"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/oskarhane/google-cli/internal/output"
)

// Version is stamped at build time via ldflags (-X); "dev" is the fallback.
var Version = "dev"

// Config holds the values of the root command's persistent flags.
// It is constructed once in main and passed to subcommand constructors.
type Config struct {
	// Account is the Google account name to act as. Empty means the default account.
	Account string

	// Format is the output format: "json", "table", or "toon". Empty means auto-detect.
	Format string

	// Debug enables debug output.
	Debug bool

	// Credentials is the path to a Google OAuth app credentials JSON file.
	// Empty means auto-resolve. It is bound to the google provider command's
	// persistent --credentials flag, not the root's.
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
		// Version enables the built-in --version flag; cobra prints it as
		// "google-cli version <Version>" via its default template.
		Version: Version,
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

	// Fail closed on `--account ""`: an explicitly set but empty account would
	// silently fall back to the default account, so e.g. `--account "$ACCT"`
	// with an unset variable would act on the wrong account. Fires here —
	// before any subcommand RunE — because the root's PersistentPreRunE runs
	// on every invocation (no subcommand overrides it). cmd is the executing
	// command; cobra has already merged the root's persistent flags into
	// cmd.Flags(), so Changed("account") sees the flag wherever it was given.
	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		if cmd.Flags().Changed("account") && cfg.Account == "" {
			return fmt.Errorf("--account is empty: pass an account name or drop the flag")
		}
		output.SetDebug(cfg.Debug)
		return nil
	}

	return root
}
