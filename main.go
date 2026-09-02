package main

import (
	"os"

	"github.com/oskarhane/everything-cli/internal/app"
	"github.com/oskarhane/everything-cli/internal/cmdtree"

	// Providers self-register via init(); adding a provider is one import.
	// cmdtree.New then discovers them through provider.List().
	_ "github.com/oskarhane/everything-cli/internal/providers/google"
	_ "github.com/oskarhane/everything-cli/internal/providers/granola"
	_ "github.com/oskarhane/everything-cli/internal/providers/linear"
)

func main() {
	cfg := app.NewConfig()
	root := cmdtree.New(cfg)
	// Back-compat shim: rewrite bare pre-provider invocations (`gmail list`,
	// `account add work`) to their provider-first form before cobra parses.
	root.SetArgs(app.RewriteLegacyArgs(root.Use, os.Args[1:], os.Stderr))
	// Errors print through PrintError (redacted) instead of cobra's default
	// stderr print, so a secret inside an error message cannot leak.
	root.SilenceErrors = true
	if err := root.Execute(); err != nil {
		app.PrintError(os.Stderr, err)
		os.Exit(1)
	}
}
