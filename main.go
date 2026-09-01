package main

import (
	"os"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/provider"
	"github.com/oskarhane/everything-cli/internal/subcommands/account"
	"github.com/oskarhane/everything-cli/internal/subcommands/skill"
	"github.com/oskarhane/everything-cli/internal/subcommands/update"

	// Providers self-register via init(); adding a provider is one import.
	_ "github.com/oskarhane/everything-cli/internal/providers/google"
	_ "github.com/oskarhane/everything-cli/internal/providers/granola"
	_ "github.com/oskarhane/everything-cli/internal/providers/linear"
)

func main() {
	cfg := app.NewConfig()
	root := app.NewRootCommand(cfg)
	for _, p := range provider.List() {
		root.AddCommand(p.NewCmd(cfg))
	}
	// CLI-own commands stay top-level: the read-only cross-provider account
	// aggregate, the embedded skill, and self-update.
	root.AddCommand(
		account.NewCmd(cfg),
		skill.NewCmd(cfg),
		update.NewCmd(cfg),
	)
	// Back-compat shim: rewrite bare pre-provider invocations (`gmail list`,
	// `account add work`) to their provider-first form before cobra parses.
	root.SetArgs(app.RewriteLegacyArgs(root.Use, os.Args[1:], os.Stderr))
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
