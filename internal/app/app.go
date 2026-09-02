// Package app builds the everything-cli command tree.
package app

import (
	"fmt"
	"io"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/oskarhane/everything-cli/internal/config"
	"github.com/oskarhane/everything-cli/internal/output"
	"github.com/oskarhane/everything-cli/internal/redact"
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

// Store opens the CLI's account store on the configured filesystem. Every
// command needing the store gets it from here, so the store-root
// resolution lives in exactly one place.
func (c *Config) Store() (*config.Store, error) {
	return config.NewStore(c.Fs, "")
}

// NewRootCommand builds the everything-cli root command with persistent flags bound to cfg.
// Subcommands are attached by callers and receive cfg.
func NewRootCommand(cfg *Config) *cobra.Command {
	root := &cobra.Command{
		Use:   "everything-cli",
		Short: "One CLI for many SaaS providers (Google, Linear, Granola)",
		// Version enables the built-in --version flag; cobra prints it as
		// "everything-cli version <Version>" via its default template.
		Version: Version,
		// Run prints help so the root's flags and usage stay visible
		// until subcommands are attached.
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	f := root.PersistentFlags()
	f.StringVar(&cfg.Account, "account", "", "Account to act as (empty = default account)")
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

// PrintError writes err to w in cobra's default "Error: <msg>" shape with
// registered secrets scrubbed. main wires this in place of cobra's error
// printing (root.SilenceErrors = true) so an error message carrying a
// secret can never reach stderr.
func PrintError(w io.Writer, err error) {
	_, _ = fmt.Fprintf(w, "Error: %s\n", redact.Redact(err.Error()))
}
